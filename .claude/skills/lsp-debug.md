# LSP Debug Skill

You are executing the **LSP Debug** skill. This skill helps you diagnose issues with the Dingo LSP server using SQLite structured logs.

## Database Location

Default: `/tmp/dingo-lsp.db`

## Current State

**Note:** Currently the LSP server logs messages as text. Structured fields (comp, mth, dir) are available but not yet populated. Queries below include both:
- **Text-based queries** (search in `msg` field) - work NOW
- **Structured queries** (use field codes) - for future when fields are populated

## Quick Query Reference

### 1. Recent Errors (Start Here)

```bash
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, comp, msg, err FROM logs WHERE lvl='E' ORDER BY ts DESC LIMIT 20"
```

### 2. Last N Log Entries

```bash
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, comp, msg FROM logs ORDER BY ts DESC LIMIT 30"
```

### 3. Activity in Last N Minutes

```bash
# Last 5 minutes
sqlite3 /tmp/dingo-lsp.db "SELECT lvl, comp, mth, msg FROM logs WHERE ts > (strftime('%s', 'now') - 300) * 1000 ORDER BY ts DESC"
```

---

## Text-Based Queries (Work Now)

Since structured fields aren't populated yet, use LIKE searches on the message field:

### Search for Keywords

```bash
# Find hover-related logs
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg FROM logs WHERE msg LIKE '%hover%' ORDER BY ts DESC LIMIT 20"

# Find completion-related logs
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg FROM logs WHERE msg LIKE '%completion%' OR msg LIKE '%Completion%' ORDER BY ts DESC LIMIT 20"

# Find definition/goto logs
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg FROM logs WHERE msg LIKE '%definition%' OR msg LIKE '%Definition%' ORDER BY ts DESC LIMIT 20"

# Find gopls-related logs
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg FROM logs WHERE msg LIKE '%gopls%' ORDER BY ts DESC LIMIT 20"

# Find transpile-related logs
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg FROM logs WHERE msg LIKE '%transpil%' ORDER BY ts DESC LIMIT 20"

# Find code action logs
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg FROM logs WHERE msg LIKE '%CodeAction%' OR msg LIKE '%code action%' ORDER BY ts DESC LIMIT 20"

# Find diagnostic/error publishing logs
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg FROM logs WHERE msg LIKE '%diagnostic%' OR msg LIKE '%Diagnostic%' ORDER BY ts DESC LIMIT 20"
```

### Find Specific File Activity

```bash
# Replace 'myfile' with actual filename
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg FROM logs WHERE msg LIKE '%myfile.dingo%' ORDER BY ts DESC LIMIT 30"
```

### Find Position/Translation Logs

```bash
sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, lvl, msg FROM logs WHERE msg LIKE '%position%' OR msg LIKE '%translat%' OR msg LIKE '%→%' ORDER BY ts DESC LIMIT 30"
```

---

## Debug Scenarios

### Scenario 1: "Nothing is working"

**Diagnosis:** Check if LSP server is receiving requests

```sql
-- Check recent activity
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, dir, mth, msg
FROM logs
WHERE ts > (strftime('%s', 'now') - 60) * 1000
ORDER BY ts DESC;
```

**Interpretation:**
- No rows = LSP server not receiving requests (check VS Code connection)
- Only `>` direction = requests arriving but no responses
- `>` and `<` = full round-trip working

---

### Scenario 2: "Hover/completion not working"

**Diagnosis:** Trace a specific LSP method

```sql
-- Hover requests (H = hover)
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, dir, comp, msg, err
FROM logs
WHERE mth = 'H'
ORDER BY ts DESC LIMIT 20;

-- Completion requests (C = completion)
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, dir, comp, msg, err
FROM logs
WHERE mth = 'C'
ORDER BY ts DESC LIMIT 20;
```

**Method codes:** C=completion, H=hover, D=definition, R=references, A=codeAction, F=formatting

---

### Scenario 3: "Position is wrong" (Go to definition jumps to wrong line)

**Diagnosis:** Check position translation

```sql
-- Find position translations
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time,
       file, dpos, gpos, msg
FROM logs
WHERE dpos IS NOT NULL OR gpos IS NOT NULL
ORDER BY ts DESC LIMIT 30;

-- Specific file
SELECT dpos, gpos, msg
FROM logs
WHERE file LIKE '%myfile.dingo%' AND dpos IS NOT NULL
ORDER BY ts DESC;
```

**Interpretation:**
- `dpos` = Dingo source position (line:col)
- `gpos` = Generated Go position (line:col)
- Missing translation = .dmap file issue

---

### Scenario 4: "Slow performance"

**Diagnosis:** Find slow operations

```sql
-- Operations taking >100ms
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time,
       mth, dur, comp, msg
FROM logs
WHERE dur > 100
ORDER BY dur DESC LIMIT 20;

-- Average duration by method
SELECT mth,
       COUNT(*) as count,
       AVG(dur) as avg_ms,
       MAX(dur) as max_ms
FROM logs
WHERE dur IS NOT NULL AND mth IS NOT NULL
GROUP BY mth
ORDER BY avg_ms DESC;
```

---

### Scenario 5: "gopls communication issues"

**Diagnosis:** Check gopls proxy

```sql
-- All gopls communication
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time,
       dir, mth, dur, msg, err
FROM logs
WHERE comp = 'gpl'
ORDER BY ts DESC LIMIT 30;

-- gopls errors only
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, msg, err
FROM logs
WHERE comp = 'gpl' AND (lvl = 'E' OR err IS NOT NULL)
ORDER BY ts DESC;
```

**Direction codes:** `>g` = to gopls, `<g` = from gopls

---

### Scenario 6: "Trace a single request"

**Diagnosis:** Follow request by ID

```sql
-- Find recent request IDs
SELECT DISTINCT rid, mth, MIN(ts) as start_ts
FROM logs
WHERE rid IS NOT NULL
GROUP BY rid
ORDER BY start_ts DESC LIMIT 10;

-- Trace specific request (replace 123 with actual rid)
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time,
       dir, comp, mth, msg
FROM logs
WHERE rid = 123
ORDER BY ts;
```

---

### Scenario 7: "Transpilation errors"

**Diagnosis:** Check transpiler component

```sql
-- Transpiler activity
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time,
       lvl, msg, err
FROM logs
WHERE comp = 'trp'
ORDER BY ts DESC LIMIT 30;

-- Transpiler errors only
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, file, msg, err
FROM logs
WHERE comp = 'trp' AND lvl = 'E'
ORDER BY ts DESC;
```

---

### Scenario 8: "File watcher issues"

**Diagnosis:** Check file monitoring

```sql
-- File watcher activity
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time,
       file, msg
FROM logs
WHERE comp = 'watch'
ORDER BY ts DESC LIMIT 30;
```

---

### Scenario 9: "Diagnostics (errors/warnings) not showing"

**Diagnosis:** Check publishDiagnostics

```sql
-- Diagnostics publishing
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time,
       file, msg
FROM logs
WHERE mth = 'P'
ORDER BY ts DESC LIMIT 30;

-- Linter activity
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time,
       lvl, msg, err
FROM logs
WHERE comp = 'lint'
ORDER BY ts DESC LIMIT 30;
```

---

### Scenario 10: "Compare what happened vs what should happen"

**Diagnosis:** Full timeline for a file

```sql
-- All activity for a specific file
SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time,
       lvl, comp, mth, dir, msg
FROM logs
WHERE file LIKE '%filename.dingo%'
ORDER BY ts DESC LIMIT 50;
```

---

## Field Reference

### Log Levels (`lvl`)
| Code | Meaning |
|------|---------|
| D | Debug |
| I | Info |
| W | Warning |
| E | Error |

### Components (`comp`)
| Code | Component |
|------|-----------|
| ide | VS Code connection |
| gpl | gopls proxy |
| trp | Transpiler |
| smap | Source map cache |
| hdl | LSP handlers |
| watch | File watcher |
| init | Initialization |
| lint | Linter |

### Methods (`mth`)
| Code | LSP Method |
|------|------------|
| C | completion |
| H | hover |
| D | definition |
| R | references |
| F | formatting |
| A | codeAction |
| S | documentSymbol |
| O | didOpen |
| X | didClose |
| T | didChange |
| P | publishDiagnostics |
| SD | didSave |

### Directions (`dir`)
| Code | Meaning |
|------|---------|
| > | From VS Code |
| < | To VS Code |
| = | Internal |
| ! | Error |
| >g | To gopls |
| <g | From gopls |

---

## Utility Commands

### Database Stats
```bash
sqlite3 /tmp/dingo-lsp.db "SELECT COUNT(*) as total_rows, MIN(datetime(ts/1000, 'unixepoch', 'localtime')) as oldest, MAX(datetime(ts/1000, 'unixepoch', 'localtime')) as newest FROM logs"
```

### Clear Old Logs (keep last 10000)
```bash
sqlite3 /tmp/dingo-lsp.db "DELETE FROM logs WHERE id NOT IN (SELECT id FROM logs ORDER BY id DESC LIMIT 10000); VACUUM;"
```

### Reset Database
```bash
rm /tmp/dingo-lsp.db
# Restart LSP server (Cmd+Shift+P → "Dingo: Restart Language Server")
```

### Export for AI Analysis
```bash
sqlite3 /tmp/dingo-lsp.db "SELECT lvl, comp, mth, dir, msg FROM logs ORDER BY ts DESC LIMIT 100" > /tmp/lsp-logs.txt
```

---

## Usage Pattern

When user reports an LSP issue:

1. **Start with recent errors:**
   ```bash
   sqlite3 /tmp/dingo-lsp.db "SELECT datetime(ts/1000, 'unixepoch', 'localtime') as time, comp, msg, err FROM logs WHERE lvl='E' ORDER BY ts DESC LIMIT 10"
   ```

2. **Check relevant component** based on symptom

3. **Trace request flow** if needed using rid

4. **Report findings** with specific log entries as evidence
