package lsp

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultMaxRows   = 50000
	compactThreshold = 10 * 1024 * 1024 // 10MB
)

// SQLiteLogger implements StructuredLogger with SQLite storage
type SQLiteLogger struct {
	db     *sql.DB
	level  LogLevel
	stmt   *sql.Stmt   // Prepared INSERT statement
	fields LogEntry    // Current fields (for builder pattern)
	mu     *sync.Mutex // Protects database writes (pointer - shared across instances)
}

// NewSQLiteLogger creates a SQLite-backed structured logger
// dbPath: path to SQLite database file
// levelStr: log level (debug/info/warn/error)
func NewSQLiteLogger(dbPath string, levelStr string) (*SQLiteLogger, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Configure for performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA cache_size=2000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	// Create schema
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	// Prepare insert statement
	stmt, err := db.Prepare(`
		INSERT INTO logs (ts, lvl, comp, mth, dir, rid, file, line, col, dpos, gpos, dur, msg, err, ctx)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}

	logger := &SQLiteLogger{
		db:    db,
		level: parseLogLevel(levelStr),
		stmt:  stmt,
		mu:    &sync.Mutex{}, // Create mutex once
	}

	// Check if compaction needed
	if err := logger.compactIfNeeded(); err != nil {
		// Log but don't fail - compaction is best-effort
		fmt.Fprintf(os.Stderr, "[dingo-lsp] SQLite compaction warning: %v\n", err)
	}

	return logger, nil
}

func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS logs (
		id      INTEGER PRIMARY KEY,
		ts      INTEGER NOT NULL,
		lvl     TEXT NOT NULL,
		comp    TEXT,
		mth     TEXT,
		dir     TEXT,
		rid     INTEGER,
		file    TEXT,
		line    INTEGER,
		col     INTEGER,
		dpos    TEXT,
		gpos    TEXT,
		dur     INTEGER,
		msg     TEXT NOT NULL,
		err     TEXT,
		ctx     TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_logs_ts ON logs(ts);
	CREATE INDEX IF NOT EXISTS idx_logs_lvl ON logs(lvl);
	CREATE INDEX IF NOT EXISTS idx_logs_rid ON logs(rid);
	CREATE INDEX IF NOT EXISTS idx_logs_file ON logs(file);
	CREATE INDEX IF NOT EXISTS idx_logs_comp ON logs(comp);
	CREATE INDEX IF NOT EXISTS idx_logs_mth ON logs(mth);
	`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Create partial index separately (SQLite syntax)
	_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_logs_err ON logs(err) WHERE err IS NOT NULL")

	return nil
}

// Close closes the database connection
func (l *SQLiteLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.stmt != nil {
		l.stmt.Close()
	}
	if l.db != nil {
		return l.db.Close()
	}
	return nil
}

// Compact removes old entries keeping only the most recent keepRows
func (l *SQLiteLogger) Compact(keepRows int) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	_, err := l.db.Exec(`
		DELETE FROM logs WHERE id NOT IN (
			SELECT id FROM logs ORDER BY id DESC LIMIT ?
		)
	`, keepRows)
	if err != nil {
		return err
	}

	_, err = l.db.Exec("VACUUM")
	return err
}

func (l *SQLiteLogger) compactIfNeeded() error {
	// Check file size
	var pageCount, pageSize int64
	if err := l.db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return err
	}
	if err := l.db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return err
	}

	dbSize := pageCount * pageSize
	if dbSize > compactThreshold {
		return l.Compact(defaultMaxRows)
	}
	return nil
}

// writeEntry writes a LogEntry to the database (synchronous)
func (l *SQLiteLogger) writeEntry(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().UnixMilli()
	}

	_, err := l.stmt.Exec(
		entry.Timestamp,
		entry.Level,
		nullStr(entry.Component),
		nullStr(entry.Method),
		nullStr(entry.Direction),
		nullInt(entry.RequestID),
		nullStr(entry.File),
		nullInt(int64(entry.Line)),
		nullInt(int64(entry.Col)),
		nullStr(entry.DingoPos),
		nullStr(entry.GoPos),
		nullInt(entry.Duration),
		entry.Message,
		nullStr(entry.Error),
		nullStr(entry.Context),
	)
	if err != nil {
		// Don't use logger (infinite loop risk) - write directly to stderr
		fmt.Fprintf(os.Stderr, "[dingo-lsp] SQLite write error: %v (msg: %s)\n", err, entry.Message)
	}
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(i int64) interface{} {
	if i == 0 {
		return nil
	}
	return i
}

// Logger interface implementation
func (l *SQLiteLogger) Debugf(format string, args ...interface{}) {
	if l.level <= LogLevelDebug {
		entry := l.fields
		entry.Level = "D"
		entry.Message = fmt.Sprintf(format, args...)
		l.writeEntry(entry)
	}
}

func (l *SQLiteLogger) Infof(format string, args ...interface{}) {
	if l.level <= LogLevelInfo {
		entry := l.fields
		entry.Level = "I"
		entry.Message = fmt.Sprintf(format, args...)
		l.writeEntry(entry)
	}
}

func (l *SQLiteLogger) Warnf(format string, args ...interface{}) {
	if l.level <= LogLevelWarn {
		entry := l.fields
		entry.Level = "W"
		entry.Message = fmt.Sprintf(format, args...)
		l.writeEntry(entry)
	}
}

func (l *SQLiteLogger) Errorf(format string, args ...interface{}) {
	if l.level <= LogLevelError {
		entry := l.fields
		entry.Level = "E"
		entry.Message = fmt.Sprintf(format, args...)
		l.writeEntry(entry)
	}
}

func (l *SQLiteLogger) Fatalf(format string, args ...interface{}) {
	entry := l.fields
	entry.Level = "E"
	entry.Message = fmt.Sprintf(format, args...)
	l.writeEntry(entry)
	os.Exit(1)
}

// StructuredLogger interface implementation
func (l *SQLiteLogger) WithFields(entry LogEntry) StructuredLogger {
	return &SQLiteLogger{
		db:     l.db,
		level:  l.level,
		stmt:   l.stmt,
		fields: entry,
		mu:     l.mu, // Share mutex pointer
	}
}

func (l *SQLiteLogger) WithComponent(comp string) StructuredLogger {
	fields := l.fields
	fields.Component = comp
	return &SQLiteLogger{
		db:     l.db,
		level:  l.level,
		stmt:   l.stmt,
		fields: fields,
		mu:     l.mu, // Share mutex pointer
	}
}

func (l *SQLiteLogger) WithMethod(method string) StructuredLogger {
	fields := l.fields
	fields.Method = method
	return &SQLiteLogger{
		db:     l.db,
		level:  l.level,
		stmt:   l.stmt,
		fields: fields,
		mu:     l.mu, // Share mutex pointer
	}
}

func (l *SQLiteLogger) WithRequestID(rid int64) StructuredLogger {
	fields := l.fields
	fields.RequestID = rid
	return &SQLiteLogger{
		db:     l.db,
		level:  l.level,
		stmt:   l.stmt,
		fields: fields,
		mu:     l.mu, // Share mutex pointer
	}
}

func (l *SQLiteLogger) WithPosition(file string, line, col int) StructuredLogger {
	fields := l.fields
	fields.File = file
	fields.Line = line
	fields.Col = col
	return &SQLiteLogger{
		db:     l.db,
		level:  l.level,
		stmt:   l.stmt,
		fields: fields,
		mu:     l.mu, // Share mutex pointer
	}
}

func (l *SQLiteLogger) WithDuration(dur int64) StructuredLogger {
	fields := l.fields
	fields.Duration = dur
	return &SQLiteLogger{
		db:     l.db,
		level:  l.level,
		stmt:   l.stmt,
		fields: fields,
		mu:     l.mu, // Share mutex pointer
	}
}

func (l *SQLiteLogger) WithTranslation(dingoPos, goPos string) StructuredLogger {
	fields := l.fields
	fields.DingoPos = dingoPos
	fields.GoPos = goPos
	return &SQLiteLogger{
		db:     l.db,
		level:  l.level,
		stmt:   l.stmt,
		fields: fields,
		mu:     l.mu, // Share mutex pointer
	}
}

func (l *SQLiteLogger) LogEntry(entry LogEntry) {
	l.writeEntry(entry)
}
