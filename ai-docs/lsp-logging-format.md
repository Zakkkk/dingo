# Dingo LSP SQLite Logging Format

## Overview

The Dingo LSP server supports structured logging to SQLite for efficient debugging and analysis. This format uses compact field codes to minimize token usage when analyzing logs with AI assistants while maintaining full queryability with standard SQL tools.

**Key Features:**
- Structured SQLite storage for fast queries
- Request/response correlation via request IDs
- Position translation debugging (Dingo ↔ Go coordinate mapping)
- Performance tracking with duration fields
- Auto-compaction to keep database size manageable
- Opt-in by default (no logging overhead unless enabled)

## Enabling SQLite Logging

### Via VS Code Settings

1. Open VS Code Settings (Cmd/Ctrl + ,)
2. Search for "Dingo LSP"
3. Enable **"Dingo: Lsp > Sqlite Logging"**
4. (Optional) Set **"Dingo: Lsp > Sqlite Log Path"** to a custom location
   - If empty, defaults to system temp directory
5. Restart the LSP server: Cmd/Ctrl+Shift+P → **"Dingo: Restart Language Server"**

### Via Environment Variable (Manual)

```bash
export DINGO_LSP_SQLITE_LOG=/path/to/dingo-lsp.db
dingo-lsp
```

## Database Location

**Default paths:**
- macOS/Linux: `$TMPDIR/dingo-lsp.db` (typically `/tmp/dingo-lsp.db`)
- Windows: `%TEMP%\dingo-lsp.db`

**Custom path:** Set via VS Code setting or `DINGO_LSP_SQLITE_LOG` environment variable

## Database Schema

### Table: `logs`

```sql
CREATE TABLE logs (
    id      INTEGER PRIMARY KEY,
    ts      INTEGER NOT NULL,       -- Unix timestamp (milliseconds)
    lvl     TEXT NOT NULL,          -- Log level (D/I/W/E)
    comp    TEXT,                   -- Component code
    mth     TEXT,                   -- LSP method code
    dir     TEXT,                   -- Direction code
    rid     INTEGER,                -- Request ID for correlation
    file    TEXT,                   -- Filename (basename only, no path)
    line    INTEGER,                -- Line number
    col     INTEGER,                -- Column number
    dpos    TEXT,                   -- Dingo position "L:C" format
    gpos    TEXT,                   -- Go position "L:C" format
    dur     INTEGER,                -- Duration in milliseconds
    msg     TEXT NOT NULL,          -- Log message
    err     TEXT,                   -- Error details (NULL if no error)
    ctx     TEXT                    -- JSON context blob
);

CREATE INDEX idx_logs_ts ON logs(ts);
CREATE INDEX idx_logs_lvl ON logs(lvl);
CREATE INDEX idx_logs_rid ON logs(rid);
CREATE INDEX idx_logs_file ON logs(file);
CREATE INDEX idx_logs_comp ON logs(comp);
CREATE INDEX idx_logs_mth ON logs(mth);
CREATE INDEX idx_logs_err ON logs(err) WHERE err IS NOT NULL;
```

### Performance Settings

The database uses the following PRAGMA settings for optimal performance:

```sql
PRAGMA journal_mode=WAL;        -- Write-Ahead Logging for concurrency
PRAGMA synchronous=NORMAL;      -- Balance safety vs performance
PRAGMA temp_store=MEMORY;       -- Keep temp tables in RAM
PRAGMA cache_size=2000;         -- ~8MB cache
```

## Field Reference

### Log Levels (`lvl`)

| Code | Level | Description |
|------|-------|-------------|
| `D` | Debug | Detailed diagnostic information |
| `I` | Info | General informational messages |
| `W` | Warn | Warning conditions |
| `E` | Error | Error conditions |

**Example:**
```sql
SELECT * FROM logs WHERE lvl = 'E';  -- Find all errors
```

---

### Components (`comp`)

| Code | Component | Description |
|------|-----------|-------------|
| `ide` | IDE Connection | VS Code ↔ dingo-lsp communication |
| `gpl` | gopls Client | dingo-lsp ↔ gopls communication |
| `trp` | Transpiler | .dingo → .go transpilation |
| `smap` | Source Map Cache | .dmap file loading and caching |
| `hdl` | LSP Handlers | Request handling logic |
| `watch` | File Watcher | File system monitoring |
| `init` | Initialization | Server startup and configuration |
| `lint` | Linter | Dingo linting and refactoring |

**Example:**
```sql
-- Find all transpiler errors
SELECT ts, msg, err FROM logs WHERE comp = 'trp' AND lvl = 'E';

-- Compare gopls vs transpiler activity
SELECT comp, COUNT(*) as events FROM logs
WHERE comp IN ('gpl', 'trp') GROUP BY comp;
```

---

### LSP Methods (`mth`)

| Code | LSP Method | Description |
|------|------------|-------------|
| `C` | completion | Autocomplete suggestions |
| `H` | hover | Hover information |
| `D` | definition | Go to definition |
| `R` | references | Find all references |
| `F` | formatting | Code formatting |
| `A` | codeAction | Quick fixes and refactorings |
| `S` | documentSymbol | File outline/symbols |
| `O` | didOpen | File opened in editor |
| `X` | didClose | File closed in editor |
| `T` | didChange | File content changed |
| `P` | publishDiagnostics | Errors/warnings published |
| `G` | initialize | LSP initialization |
| `I` | initialized | LSP initialized notification |
| `SD` | didSave | File saved |

**Example:**
```sql
-- Track completion request performance
SELECT AVG(dur) as avg_ms, MAX(dur) as max_ms, COUNT(*) as requests
FROM logs WHERE mth = 'C' AND dur IS NOT NULL;

-- Most frequently used LSP features
SELECT mth, COUNT(*) as count FROM logs
WHERE mth IS NOT NULL
GROUP BY mth
ORDER BY count DESC;
```

---

### Directions (`dir`)

| Code | Direction | Description |
|------|-----------|-------------|
| `>` | From IDE | Request received from VS Code |
| `<` | To IDE | Response sent to VS Code |
| `=` | Internal | Internal server event |
| `!` | Error | Error condition |
| `>g` | To gopls | Request forwarded to gopls |
| `<g` | From gopls | Response received from gopls |

**Example:**
```sql
-- Trace request flow: IDE → dingo-lsp → gopls → dingo-lsp → IDE
SELECT ts, dir, comp, msg FROM logs
WHERE rid = 42
ORDER BY ts;

-- Find requests that failed before reaching gopls
SELECT rid, msg FROM logs
WHERE dir = '>' AND rid NOT IN (
    SELECT rid FROM logs WHERE dir = '>g'
);
```

---

### Request ID (`rid`)

Integer correlation ID for tracking requests through the LSP pipeline.

**Request Flow:**
1. VS Code sends request → `dir='>'` with `rid=N`
2. dingo-lsp processes → `dir='='` with `rid=N`
3. Forward to gopls → `dir='>g'` with `rid=N`
4. gopls responds → `dir='<g'` with `rid=N`
5. Send to VS Code → `dir='<'` with `rid=N`

**Example:**
```sql
-- Full request trace with timing
SELECT
    datetime(ts/1000, 'unixepoch', 'localtime') as time,
    dir, comp, mth,
    (ts - LAG(ts) OVER (PARTITION BY rid ORDER BY ts)) as delta_ms,
    msg
FROM logs
WHERE rid = 123
ORDER BY ts;
```

---

### Position Fields

| Field | Type | Format | Description |
|-------|------|--------|-------------|
| `file` | TEXT | `"main.dingo"` | Filename only (no directory path) |
| `line` | INTEGER | `42` | Line number (1-based) |
| `col` | INTEGER | `15` | Column number (0-based or 1-based, depends on LSP) |
| `dpos` | TEXT | `"42:15"` | Dingo source position (line:column) |
| `gpos` | TEXT | `"87:15"` | Generated Go position (line:column) |

**Use cases:**
- `file`/`line`/`col`: General position tracking
- `dpos`/`gpos`: Position translation debugging (when .dmap mapping is active)

**Example:**
```sql
-- Find position translation activity
SELECT file, dpos, gpos, msg FROM logs
WHERE dpos IS NOT NULL AND gpos IS NOT NULL
ORDER BY ts DESC LIMIT 50;

-- Position translations for a specific file
SELECT dpos, gpos, comp, msg FROM logs
WHERE file = 'main.dingo' AND dpos IS NOT NULL;
```

---

### Duration (`dur`)

Duration in milliseconds for operations. Only set for requests/operations with measurable timing.

**Example:**
```sql
-- Find slow operations (>100ms)
SELECT
    datetime(ts/1000, 'unixepoch', 'localtime') as time,
    mth, dur, msg
FROM logs
WHERE dur > 100
ORDER BY dur DESC
LIMIT 20;

-- Average duration by LSP method
SELECT mth, AVG(dur) as avg_ms, MAX(dur) as max_ms, COUNT(*) as count
FROM logs
WHERE mth IS NOT NULL AND dur IS NOT NULL
GROUP BY mth
ORDER BY avg_ms DESC;
```

---

### Error Field (`err`)

Full error details (NULL if no error). Indexed with partial index for fast error queries.

**Example:**
```sql
-- All errors with details
SELECT
    datetime(ts/1000, 'unixepoch', 'localtime') as time,
    comp, msg, err
FROM logs
WHERE err IS NOT NULL
ORDER BY ts DESC;

-- Error frequency by component
SELECT comp, COUNT(*) as error_count
FROM logs
WHERE err IS NOT NULL
GROUP BY comp
ORDER BY error_count DESC;
```

---

### Context Field (`ctx`)

JSON blob for additional context (rarely used). Can store arbitrary structured data.

**Example:**
```sql
-- Find logs with extra context
SELECT ts, msg, ctx FROM logs WHERE ctx IS NOT NULL;

-- Parse JSON (SQLite 3.38+ with JSON1 extension)
SELECT
    msg,
    json_extract(ctx, '$.key') as value
FROM logs
WHERE ctx IS NOT NULL;
```

---

## Common Debug Queries

### 1. Find All Errors

```sql
SELECT
    datetime(ts/1000, 'unixepoch', 'localtime') as time,
    comp, msg, err
FROM logs
WHERE lvl = 'E'
ORDER BY ts DESC
LIMIT 20;
```

**Use case:** Quick overview of recent errors

---

### 2. Trace Request Chain by Request ID

```sql
SELECT
    datetime(ts/1000, 'unixepoch', 'localtime') as time,
    comp, dir, mth, msg
FROM logs
WHERE rid = 123
ORDER BY ts;
```

**Use case:** Follow a single request through the entire LSP pipeline

**Advanced - With timing deltas:**
```sql
SELECT
    datetime(ts/1000, 'unixepoch', 'localtime') as time,
    dir, comp, mth,
    ts - LAG(ts) OVER (PARTITION BY rid ORDER BY ts) as delta_ms,
    msg
FROM logs
WHERE rid = 123
ORDER BY ts;
```

---

### 3. Find Slow Operations

```sql
SELECT
    datetime(ts/1000, 'unixepoch', 'localtime') as time,
    mth, dur, msg
FROM logs
WHERE dur > 100
ORDER BY dur DESC
LIMIT 20;
```

**Use case:** Identify performance bottlenecks

---

### 4. Method Performance Statistics

```sql
SELECT
    mth,
    COUNT(*) as count,
    AVG(dur) as avg_ms,
    MIN(dur) as min_ms,
    MAX(dur) as max_ms,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY dur) as p95_ms
FROM logs
WHERE mth IS NOT NULL AND dur IS NOT NULL
GROUP BY mth
ORDER BY avg_ms DESC;
```

**Use case:** Performance profiling of LSP methods

**Note:** `PERCENTILE_CONT` requires SQLite 3.35+ or use alternative:
```sql
-- Alternative percentile (simpler, approximate)
SELECT mth, AVG(dur) as avg_ms, MAX(dur) as max_ms
FROM logs WHERE mth IS NOT NULL AND dur IS NOT NULL
GROUP BY mth ORDER BY avg_ms DESC;
```

---

### 5. Position Translation Debugging

```sql
SELECT
    file, dpos, gpos, msg
FROM logs
WHERE dpos IS NOT NULL
ORDER BY ts DESC
LIMIT 50;
```

**Use case:** Debug .dingo → .go position mapping issues

**Find mismatches:**
```sql
SELECT file, dpos, gpos, msg
FROM logs
WHERE dpos IS NOT NULL AND gpos IS NOT NULL
  AND dpos != gpos  -- Different positions (expected for transpilation)
ORDER BY ts DESC;
```

---

### 6. gopls Communication Analysis

```sql
-- All gopls requests
SELECT
    datetime(ts/1000, 'unixepoch', 'localtime') as time,
    dir, mth, dur, msg
FROM logs
WHERE comp = 'gpl'
ORDER BY ts DESC
LIMIT 50;

-- gopls response times
SELECT
    mth,
    AVG(dur) as avg_ms,
    MAX(dur) as max_ms,
    COUNT(*) as count
FROM logs
WHERE comp = 'gpl' AND dur IS NOT NULL
GROUP BY mth
ORDER BY avg_ms DESC;
```

**Use case:** Analyze gopls performance and communication patterns

---

### 7. File Activity Timeline

```sql
SELECT
    file,
    COUNT(*) as events,
    SUM(CASE WHEN lvl = 'E' THEN 1 ELSE 0 END) as errors
FROM logs
WHERE file IS NOT NULL
GROUP BY file
ORDER BY events DESC;
```

**Use case:** Which files have the most LSP activity or errors?

---

### 8. Activity Heatmap (by hour)

```sql
SELECT
    strftime('%Y-%m-%d %H:00', datetime(ts/1000, 'unixepoch', 'localtime')) as hour,
    COUNT(*) as events
FROM logs
GROUP BY hour
ORDER BY hour DESC
LIMIT 24;
```

**Use case:** When is the LSP server most active?

---

### 9. Error Correlation

```sql
-- Errors grouped by message pattern
SELECT msg, COUNT(*) as occurrences
FROM logs
WHERE lvl = 'E'
GROUP BY msg
ORDER BY occurrences DESC
LIMIT 10;

-- Files with most errors
SELECT file, COUNT(*) as error_count
FROM logs
WHERE lvl = 'E' AND file IS NOT NULL
GROUP BY file
ORDER BY error_count DESC;
```

**Use case:** Find repeated errors or problematic files

---

### 10. Recent Activity (Last Hour)

```sql
SELECT
    datetime(ts/1000, 'unixepoch', 'localtime') as time,
    lvl, comp, mth, msg
FROM logs
WHERE ts > (strftime('%s', 'now') - 3600) * 1000
ORDER BY ts DESC;
```

**Use case:** What happened recently?

---

## Querying with sqlite3

### Basic Usage

```bash
# Open database
sqlite3 /tmp/dingo-lsp.db

# Run query
sqlite3 /tmp/dingo-lsp.db "SELECT * FROM logs ORDER BY ts DESC LIMIT 10;"

# Pretty output
sqlite3 -column -header /tmp/dingo-lsp.db "SELECT lvl, comp, msg FROM logs LIMIT 10;"
```

### Common sqlite3 Commands

```sql
.headers on          -- Show column headers
.mode column         -- Column-aligned output
.mode table          -- Table-style output (SQLite 3.33+)
.width 20 10 50      -- Set column widths
.timer on            -- Show query execution time
.schema logs         -- Show table schema
.quit                -- Exit
```

### Example Session

```bash
$ sqlite3 /tmp/dingo-lsp.db
SQLite version 3.39.5
sqlite> .headers on
sqlite> .mode column
sqlite> SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg
   ...> FROM logs ORDER BY ts DESC LIMIT 5;
time                  lvl  msg
--------------------  ---  ----------------------------------
2025-12-10 14:32:15  I    Request received: hover
2025-12-10 14:32:14  D    Position translated: 42:15 → 87:15
2025-12-10 14:32:12  I    File opened: main.dingo
2025-12-10 14:32:10  I    LSP initialized
2025-12-10 14:32:08  I    Server started
```

---

## Querying with DuckDB

DuckDB provides richer analytics capabilities and better performance for complex queries.

### Installation

```bash
# macOS
brew install duckdb

# Or download from https://duckdb.org/docs/installation/
```

### Basic Usage

```bash
duckdb /tmp/dingo-lsp.db
```

### Example Queries

```sql
-- Read SQLite database
SELECT * FROM logs ORDER BY ts DESC LIMIT 10;

-- Advanced analytics with DuckDB functions
SELECT
    mth,
    quantile_cont(dur, 0.50) as median_ms,
    quantile_cont(dur, 0.95) as p95_ms,
    quantile_cont(dur, 0.99) as p99_ms
FROM logs
WHERE dur IS NOT NULL
GROUP BY mth
ORDER BY median_ms DESC;

-- Time series analysis
SELECT
    time_bucket(INTERVAL '5 minutes', to_timestamp(ts/1000)) as bucket,
    COUNT(*) as events,
    SUM(CASE WHEN lvl = 'E' THEN 1 ELSE 0 END) as errors
FROM logs
GROUP BY bucket
ORDER BY bucket DESC;
```

---

## Log Rotation and Compaction

### Automatic Compaction

The LSP server automatically compacts the database on startup if:
- Database file size > 10 MB
- Keeps the 50,000 most recent rows
- Runs `VACUUM` to reclaim space

### Manual Compaction

```bash
# Via sqlite3
sqlite3 /tmp/dingo-lsp.db "DELETE FROM logs WHERE id NOT IN (SELECT id FROM logs ORDER BY id DESC LIMIT 50000); VACUUM;"

# Check database size
ls -lh /tmp/dingo-lsp.db
```

### Complete Reset

```bash
# Delete the database file to start fresh
rm /tmp/dingo-lsp.db

# Restart the LSP server
# (database will be recreated automatically)
```

---

## Using Logs with AI Assistants

### Efficient Log Sharing

The compact field codes minimize token usage when pasting logs into AI conversations:

```sql
-- Export compact format for AI analysis
SELECT lvl, comp, mth, dir, msg
FROM logs
WHERE ts > (strftime('%s', 'now') - 3600) * 1000
ORDER BY ts DESC;
```

**Example output (token-efficient):**
```
E | trp | D | > | Failed to transpile: syntax error
W | smap | = | = | Cache miss for main.dingo
I | gpl | H | >g | Forwarding hover request
```

### Example Prompts

- "Show me errors from the last hour"
- "Trace request ID 42 through the system"
- "What files had position translation issues?"
- "Which LSP operations are slowest?"
- "Analyze gopls communication patterns"

### Query Result Template

When sharing logs with AI:

```
Task: [What you're investigating]
Query: [SQL query used]
Results: [Paste query results]
Question: [Specific question about the results]
```

---

## Troubleshooting

### No logs appearing

1. Check if SQLite logging is enabled in VS Code settings
2. Verify the database file exists: `ls -lh /tmp/dingo-lsp.db`
3. Check LSP server is running: `ps aux | grep dingo-lsp`
4. Check VS Code Output panel (View → Output → Dingo Language Server)

### Database locked errors

WAL mode should prevent this, but if it occurs:
- Close all other connections to the database
- Check for stale `.db-shm` or `.db-wal` files
- Restart the LSP server

### Database too large

- Default compaction keeps it under 10 MB
- For more aggressive cleanup: `DELETE FROM logs WHERE ts < [older_timestamp]`
- Or delete the entire file and restart

### Permission errors

- Ensure write access to the database directory
- Default `/tmp` should be writable
- Check file permissions: `ls -l /tmp/dingo-lsp.db`

---

## Implementation Details

### Database Configuration

- **Journal Mode:** WAL (Write-Ahead Logging)
  - Allows concurrent reads during writes
  - Better performance for append-heavy workload
- **Synchronous:** NORMAL
  - Balanced durability vs performance
  - Acceptable risk for debug logs
- **Temp Store:** MEMORY
  - Faster query execution
- **Cache Size:** 2000 pages (~8 MB)

### Write Strategy

- **Synchronous writes** (no async batching)
  - Simpler implementation
  - Acceptable for LSP event volume (<1000 events/min typical)
- **Prepared statement** for inserts
  - Minimal overhead per write
- **Mutex-protected** writes
  - Thread-safe for concurrent LSP handlers

### Schema Design

- **Integer timestamp** (Unix milliseconds)
  - Fast comparisons and sorting
  - Use `datetime(ts/1000, 'unixepoch', 'localtime')` for human-readable
- **Compact TEXT codes** (1-4 chars)
  - Minimal storage overhead
  - Fast string comparisons
- **Partial index on `err`**
  - Only indexes non-NULL errors
  - Saves space, improves error queries
- **Composite indexes**
  - Optimized for common query patterns
  - Trade-off: write overhead vs read performance

---

## Reference: Complete Field Code Tables

### All Component Codes

| Code | Component | Usage |
|------|-----------|-------|
| `ide` | IDE Connection | VS Code communication |
| `gpl` | gopls Client | gopls proxy |
| `trp` | Transpiler | .dingo → .go |
| `smap` | Source Map | .dmap cache |
| `hdl` | LSP Handlers | Request routing |
| `watch` | File Watcher | FS monitoring |
| `init` | Initialization | Server setup |
| `lint` | Linter | Diagnostics |

### All Method Codes

| Code | LSP Method | Category |
|------|------------|----------|
| `C` | completion | Editor feature |
| `H` | hover | Editor feature |
| `D` | definition | Navigation |
| `R` | references | Navigation |
| `F` | formatting | Code action |
| `A` | codeAction | Code action |
| `S` | documentSymbol | Symbols |
| `O` | didOpen | Notification |
| `X` | didClose | Notification |
| `T` | didChange | Notification |
| `P` | publishDiagnostics | Diagnostics |
| `G` | initialize | Lifecycle |
| `I` | initialized | Lifecycle |
| `SD` | didSave | Notification |

### All Direction Codes

| Code | Meaning | Flow |
|------|---------|------|
| `>` | From IDE | VS Code → dingo-lsp |
| `<` | To IDE | dingo-lsp → VS Code |
| `=` | Internal | dingo-lsp internal |
| `!` | Error | Error event |
| `>g` | To gopls | dingo-lsp → gopls |
| `<g` | From gopls | gopls → dingo-lsp |

---

## Version Information

- **Schema Version:** 1.0
- **SQLite Version Required:** 3.31+ (for partial indexes)
- **Optimal SQLite Version:** 3.35+ (for percentile functions)
- **DuckDB Recommended:** 0.9.0+

---

## Additional Resources

- **SQLite Documentation:** https://www.sqlite.org/docs.html
- **DuckDB Documentation:** https://duckdb.org/docs/
- **LSP Specification:** https://microsoft.github.io/language-server-protocol/

---

**Last Updated:** 2025-12-10
