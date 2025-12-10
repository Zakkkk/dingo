package lsp

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSQLiteLoggerCreation tests basic logger creation and schema initialization
func TestSQLiteLoggerCreation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath, "info")
	if err != nil {
		t.Fatalf("Failed to create SQLiteLogger: %v", err)
	}
	defer logger.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created: %s", dbPath)
	}

	// Verify schema exists
	var tableName string
	err = logger.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='logs'").Scan(&tableName)
	if err != nil {
		t.Fatalf("Failed to query schema: %v", err)
	}
	if tableName != "logs" {
		t.Errorf("Expected table 'logs', got '%s'", tableName)
	}

	// Verify indexes exist
	expectedIndexes := []string{"idx_logs_ts", "idx_logs_lvl", "idx_logs_rid", "idx_logs_file", "idx_logs_comp", "idx_logs_mth", "idx_logs_err"}
	rows, err := logger.db.Query("SELECT name FROM sqlite_master WHERE type='index'")
	if err != nil {
		t.Fatalf("Failed to query indexes: %v", err)
	}
	defer rows.Close()

	foundIndexes := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Failed to scan index name: %v", err)
		}
		foundIndexes[name] = true
	}

	for _, expected := range expectedIndexes {
		if !foundIndexes[expected] {
			t.Errorf("Expected index '%s' not found", expected)
		}
	}
}

// TestSQLiteLoggerBasicLogging tests all log levels
func TestSQLiteLoggerBasicLogging(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath, "debug")
	if err != nil {
		t.Fatalf("Failed to create SQLiteLogger: %v", err)
	}
	defer logger.Close()

	// Log messages at all levels
	logger.Debugf("Debug message: %d", 1)
	logger.Infof("Info message: %s", "test")
	logger.Warnf("Warning message")
	logger.Errorf("Error message: %v", "test error")

	// Give a moment for writes to complete (synchronous, so immediate)
	time.Sleep(10 * time.Millisecond)

	// Verify all messages were written
	var count int
	err = logger.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 log entries, got %d", count)
	}

	// Verify log levels
	rows, err := logger.db.Query("SELECT lvl, msg FROM logs ORDER BY id")
	if err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}
	defer rows.Close()

	expected := []struct {
		level string
		msg   string
	}{
		{"D", "Debug message: 1"},
		{"I", "Info message: test"},
		{"W", "Warning message"},
		{"E", "Error message: test error"},
	}

	i := 0
	for rows.Next() {
		var level, msg string
		if err := rows.Scan(&level, &msg); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		if i >= len(expected) {
			t.Errorf("More rows than expected")
			break
		}
		if level != expected[i].level {
			t.Errorf("Row %d: expected level '%s', got '%s'", i, expected[i].level, level)
		}
		if msg != expected[i].msg {
			t.Errorf("Row %d: expected msg '%s', got '%s'", i, expected[i].msg, msg)
		}
		i++
	}

	if i != len(expected) {
		t.Errorf("Expected %d rows, got %d", len(expected), i)
	}
}

// TestSQLiteLoggerLogLevelFiltering tests log level filtering
func TestSQLiteLoggerLogLevelFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create logger with WARN level
	logger, err := NewSQLiteLogger(dbPath, "warn")
	if err != nil {
		t.Fatalf("Failed to create SQLiteLogger: %v", err)
	}
	defer logger.Close()

	// Log at all levels
	logger.Debugf("Debug message")
	logger.Infof("Info message")
	logger.Warnf("Warning message")
	logger.Errorf("Error message")

	time.Sleep(10 * time.Millisecond)

	// Should only have WARN and ERROR
	var count int
	err = logger.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 log entries (warn+error), got %d", count)
	}
}

// TestSQLiteLoggerStructuredFields tests StructuredLogger interface
func TestSQLiteLoggerStructuredFields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath, "info")
	if err != nil {
		t.Fatalf("Failed to create SQLiteLogger: %v", err)
	}
	defer logger.Close()

	// Use builder pattern
	structLogger := logger.
		WithComponent(CompSmap).
		WithMethod(MthHover).
		WithRequestID(42).
		WithPosition("test.dingo", 10, 5).
		WithTranslation("10:5", "15:8").
		WithDuration(123)

	structLogger.Infof("Structured log entry")

	time.Sleep(10 * time.Millisecond)

	// Verify all fields
	var comp, mth, file, dpos, gpos, msg string
	var rid, line, col, dur sql.NullInt64
	err = logger.db.QueryRow(`
		SELECT comp, mth, rid, file, line, col, dpos, gpos, dur, msg
		FROM logs WHERE msg = 'Structured log entry'
	`).Scan(&comp, &mth, &rid, &file, &line, &col, &dpos, &gpos, &dur, &msg)
	if err != nil {
		t.Fatalf("Failed to query structured log: %v", err)
	}

	if comp != CompSmap {
		t.Errorf("Expected comp '%s', got '%s'", CompSmap, comp)
	}
	if mth != MthHover {
		t.Errorf("Expected mth '%s', got '%s'", MthHover, mth)
	}
	if !rid.Valid || rid.Int64 != 42 {
		t.Errorf("Expected rid 42, got %v", rid)
	}
	if file != "test.dingo" {
		t.Errorf("Expected file 'test.dingo', got '%s'", file)
	}
	if !line.Valid || line.Int64 != 10 {
		t.Errorf("Expected line 10, got %v", line)
	}
	if !col.Valid || col.Int64 != 5 {
		t.Errorf("Expected col 5, got %v", col)
	}
	if dpos != "10:5" {
		t.Errorf("Expected dpos '10:5', got '%s'", dpos)
	}
	if gpos != "15:8" {
		t.Errorf("Expected gpos '15:8', got '%s'", gpos)
	}
	if !dur.Valid || dur.Int64 != 123 {
		t.Errorf("Expected dur 123, got %v", dur)
	}
}

// TestSQLiteLoggerLogEntry tests direct LogEntry writing
func TestSQLiteLoggerLogEntry(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath, "info")
	if err != nil {
		t.Fatalf("Failed to create SQLiteLogger: %v", err)
	}
	defer logger.Close()

	// Write complete entry
	entry := LogEntry{
		Timestamp: time.Now().UnixMilli(),
		Level:     "I",
		Component: CompTrans,
		Method:    MthDefinition,
		Direction: DirFromIDE,
		RequestID: 99,
		File:      "main.dingo",
		Line:      20,
		Col:       15,
		DingoPos:  "20:15",
		GoPos:     "25:20",
		Duration:  456,
		Message:   "Complete entry",
		Error:     "test error",
		Context:   `{"key":"value"}`,
	}

	logger.LogEntry(entry)
	time.Sleep(10 * time.Millisecond)

	// Verify all fields
	var stored LogEntry
	var ts, rid, line, col, dur sql.NullInt64
	var comp, mth, dir, file, dpos, gpos, msg, errStr, ctx sql.NullString

	err = logger.db.QueryRow(`
		SELECT ts, lvl, comp, mth, dir, rid, file, line, col, dpos, gpos, dur, msg, err, ctx
		FROM logs WHERE msg = 'Complete entry'
	`).Scan(&ts, &stored.Level, &comp, &mth, &dir, &rid, &file, &line, &col, &dpos, &gpos, &dur, &msg, &errStr, &ctx)
	if err != nil {
		t.Fatalf("Failed to query entry: %v", err)
	}

	if stored.Level != "I" {
		t.Errorf("Expected level 'I', got '%s'", stored.Level)
	}
	if !comp.Valid || comp.String != CompTrans {
		t.Errorf("Expected comp '%s', got %v", CompTrans, comp)
	}
	if !mth.Valid || mth.String != MthDefinition {
		t.Errorf("Expected mth '%s', got %v", MthDefinition, mth)
	}
	if !dir.Valid || dir.String != DirFromIDE {
		t.Errorf("Expected dir '%s', got %v", DirFromIDE, dir)
	}
	if !rid.Valid || rid.Int64 != 99 {
		t.Errorf("Expected rid 99, got %v", rid)
	}
	if !file.Valid || file.String != "main.dingo" {
		t.Errorf("Expected file 'main.dingo', got %v", file)
	}
	if !line.Valid || line.Int64 != 20 {
		t.Errorf("Expected line 20, got %v", line)
	}
	if !col.Valid || col.Int64 != 15 {
		t.Errorf("Expected col 15, got %v", col)
	}
	if !dpos.Valid || dpos.String != "20:15" {
		t.Errorf("Expected dpos '20:15', got %v", dpos)
	}
	if !gpos.Valid || gpos.String != "25:20" {
		t.Errorf("Expected gpos '25:20', got %v", gpos)
	}
	if !dur.Valid || dur.Int64 != 456 {
		t.Errorf("Expected dur 456, got %v", dur)
	}
	if !errStr.Valid || errStr.String != "test error" {
		t.Errorf("Expected err 'test error', got %v", errStr)
	}
	if !ctx.Valid || ctx.String != `{"key":"value"}` {
		t.Errorf("Expected ctx '{\"key\":\"value\"}', got %v", ctx)
	}
}

// TestSQLiteLoggerCompaction tests database compaction
func TestSQLiteLoggerCompaction(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath, "info")
	if err != nil {
		t.Fatalf("Failed to create SQLiteLogger: %v", err)
	}

	// Write 1000 log entries
	for i := 0; i < 1000; i++ {
		logger.Infof("Log entry %d", i)
	}

	time.Sleep(50 * time.Millisecond)

	// Verify all entries exist
	var count int
	err = logger.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}
	if count != 1000 {
		t.Errorf("Expected 1000 entries before compaction, got %d", count)
	}

	// Compact to 100 entries
	err = logger.Compact(100)
	if err != nil {
		t.Fatalf("Failed to compact: %v", err)
	}

	// Verify only 100 entries remain
	err = logger.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count logs after compaction: %v", err)
	}
	if count != 100 {
		t.Errorf("Expected 100 entries after compaction, got %d", count)
	}

	// Verify the most recent entries were kept
	var msg string
	err = logger.db.QueryRow("SELECT msg FROM logs ORDER BY id ASC LIMIT 1").Scan(&msg)
	if err != nil {
		t.Fatalf("Failed to get oldest message: %v", err)
	}
	if msg != "Log entry 900" {
		t.Errorf("Expected oldest remaining message 'Log entry 900', got '%s'", msg)
	}

	err = logger.db.QueryRow("SELECT msg FROM logs ORDER BY id DESC LIMIT 1").Scan(&msg)
	if err != nil {
		t.Fatalf("Failed to get newest message: %v", err)
	}
	if msg != "Log entry 999" {
		t.Errorf("Expected newest message 'Log entry 999', got '%s'", msg)
	}

	logger.Close()
}

// TestSQLiteLoggerClose tests proper cleanup
func TestSQLiteLoggerClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath, "info")
	if err != nil {
		t.Fatalf("Failed to create SQLiteLogger: %v", err)
	}

	logger.Infof("Test message")
	time.Sleep(10 * time.Millisecond)

	// Close should succeed
	err = logger.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Second close should also succeed (idempotent)
	err = logger.Close()
	if err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}
}

// TestMultiLogger tests MultiLogger routing
func TestMultiLogger(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath1 := filepath.Join(tmpDir, "test1.db")
	dbPath2 := filepath.Join(tmpDir, "test2.db")

	logger1, err := NewSQLiteLogger(dbPath1, "info")
	if err != nil {
		t.Fatalf("Failed to create logger1: %v", err)
	}
	defer logger1.Close()

	logger2, err := NewSQLiteLogger(dbPath2, "info")
	if err != nil {
		t.Fatalf("Failed to create logger2: %v", err)
	}
	defer logger2.Close()

	// Create MultiLogger
	multi := NewMultiLogger(logger1, logger2)

	// Log to MultiLogger
	multi.Infof("Test message 1")
	multi.Warnf("Test message 2")
	multi.Errorf("Test message 3")

	time.Sleep(50 * time.Millisecond)

	// Verify both databases have all messages
	for i, logger := range []*SQLiteLogger{logger1, logger2} {
		var count int
		err = logger.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
		if err != nil {
			t.Fatalf("Logger %d: failed to count logs: %v", i+1, err)
		}
		if count != 3 {
			t.Errorf("Logger %d: expected 3 log entries, got %d", i+1, count)
		}

		// Verify messages
		rows, err := logger.db.Query("SELECT msg FROM logs ORDER BY id")
		if err != nil {
			t.Fatalf("Logger %d: failed to query logs: %v", i+1, err)
		}

		expected := []string{"Test message 1", "Test message 2", "Test message 3"}
		j := 0
		for rows.Next() {
			var msg string
			if err := rows.Scan(&msg); err != nil {
				t.Fatalf("Logger %d: failed to scan message: %v", i+1, err)
			}
			if j >= len(expected) {
				t.Errorf("Logger %d: more messages than expected", i+1)
				break
			}
			if msg != expected[j] {
				t.Errorf("Logger %d: expected msg '%s', got '%s'", i+1, expected[j], msg)
			}
			j++
		}
		rows.Close()

		if j != len(expected) {
			t.Errorf("Logger %d: expected %d messages, got %d", i+1, len(expected), j)
		}
	}

	// Test Close on MultiLogger
	err = multi.Close()
	if err != nil {
		t.Errorf("MultiLogger.Close() failed: %v", err)
	}
}

// TestSQLiteLoggerConcurrency tests thread safety
func TestSQLiteLoggerConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath, "info")
	if err != nil {
		t.Fatalf("Failed to create SQLiteLogger: %v", err)
	}
	defer logger.Close()

	// Write concurrently from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				logger.Infof("Goroutine %d message %d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	time.Sleep(100 * time.Millisecond)

	// Should have all 1000 messages
	var count int
	err = logger.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count logs: %v", err)
	}
	if count != 1000 {
		t.Errorf("Expected 1000 entries from concurrent writes, got %d", count)
	}
}

// TestSQLiteLoggerNullFields tests NULL handling for optional fields
func TestSQLiteLoggerNullFields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath, "info")
	if err != nil {
		t.Fatalf("Failed to create SQLiteLogger: %v", err)
	}
	defer logger.Close()

	// Log with minimal fields (no structured data)
	logger.Infof("Minimal message")
	time.Sleep(10 * time.Millisecond)

	// Verify NULL fields are actually NULL in database
	var comp, mth, dir, file, dpos, gpos, err_, ctx sql.NullString
	var rid, line, col, dur sql.NullInt64

	err = logger.db.QueryRow(`
		SELECT comp, mth, dir, rid, file, line, col, dpos, gpos, dur, err, ctx
		FROM logs WHERE msg = 'Minimal message'
	`).Scan(&comp, &mth, &dir, &rid, &file, &line, &col, &dpos, &gpos, &dur, &err_, &ctx)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	// All should be NULL
	if comp.Valid {
		t.Errorf("Expected comp to be NULL, got '%s'", comp.String)
	}
	if mth.Valid {
		t.Errorf("Expected mth to be NULL, got '%s'", mth.String)
	}
	if dir.Valid {
		t.Errorf("Expected dir to be NULL, got '%s'", dir.String)
	}
	if rid.Valid {
		t.Errorf("Expected rid to be NULL, got %d", rid.Int64)
	}
	if file.Valid {
		t.Errorf("Expected file to be NULL, got '%s'", file.String)
	}
	if line.Valid {
		t.Errorf("Expected line to be NULL, got %d", line.Int64)
	}
	if col.Valid {
		t.Errorf("Expected col to be NULL, got %d", col.Int64)
	}
	if dpos.Valid {
		t.Errorf("Expected dpos to be NULL, got '%s'", dpos.String)
	}
	if gpos.Valid {
		t.Errorf("Expected gpos to be NULL, got '%s'", gpos.String)
	}
	if dur.Valid {
		t.Errorf("Expected dur to be NULL, got %d", dur.Int64)
	}
	if err_.Valid {
		t.Errorf("Expected err to be NULL, got '%s'", err_.String)
	}
	if ctx.Valid {
		t.Errorf("Expected ctx to be NULL, got '%s'", ctx.String)
	}
}
