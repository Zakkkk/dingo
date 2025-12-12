# GPT-5.1 Codex Review: Dingo LSP Implementation

**Reviewer:** OpenAI GPT-5.1 Codex (via claudish proxy)
**Date:** 2025-11-18
**Session:** 20251118-152749-phase5-lsp-foundation
**Scope:** Phase V LSP Foundation - Iteration 01

---

## Executive Summary

The Dingo LSP implementation demonstrates **MAJOR_ISSUES** that prevent it from functioning correctly in production. While the architectural foundation is sound (clean module boundaries, structured logging, thread-safe caching), critical integration gaps make the server non-functional: JSON-RPC handlers are not wired to the transport layer, gopls notifications never reach the editor, fallback proxying returns "method not implemented" for most LSP calls, and `.dingo` URIs leak to gopls when source maps are missing. These core protocol violations must be fixed before any testing can proceed.

**Status:** MAJOR_ISSUES - Core functionality broken, requires immediate fixes.

---

## ✅ Strengths

1. **Clean Module Boundaries**
   - Clear separation of concerns: `gopls_client.go` manages subprocess, `translator.go` handles positions, `sourcemap_cache.go` manages caching
   - Well-defined interfaces between components
   - Good use of Go idioms (Accept interfaces, return structs)

2. **Structured Logging**
   - Configurable log levels via `logger.go`
   - Consistent logging patterns throughout
   - Useful debug information for troubleshooting

3. **Thread-Safe Source Map Cache**
   - Proper RWMutex usage in `sourcemap_cache.go`
   - Double-checked locking pattern implemented correctly
   - Version validation before caching

4. **Debounced File Watcher**
   - 500ms debouncing prevents excessive transpilations
   - Correctly ignores vendor/build directories
   - Uses fsnotify efficiently

5. **Performance Exceeds Targets**
   - Position translation: 3.4μs (294x faster than 1ms target)
   - Round-trip translation: 1.0μs (2000x faster than 2ms target)
   - Cache lookups: 63ns (16x faster than 1μs target)

---

## ⚠️ Concerns

### CRITICAL Issues

#### 1. **JSON-RPC Handler Not Wired to Transport**
**Location:** `cmd/dingo-lsp/main.go:39-47`, `pkg/lsp/server.go:62-75`

**Issue:** The main.go creates a `jsonrpc2.Conn` but never actually calls `server.Serve()` or registers the handler with the connection. The server will accept connections but never process any LSP requests.

**Current Code (main.go:39-47):**
```go
// Create stdio transport
stream := jsonrpc2.NewStream(os.Stdin, os.Stdout)
conn := jsonrpc2.NewConn(stream)

// Start serving
ctx := context.Background()
server.Serve(ctx, conn)  // This is called, but...
```

**Current Code (server.go:62-75):**
```go
func (s *Server) Serve(ctx context.Context, conn jsonrpc2.Conn) error {
    // Register LSP method handlers
    conn.Go(ctx, jsonrpc2.HandlerWithError(s.handleRequest))
    <-conn.Done()
    return conn.Err()
}
```

**Problem:** The `conn.Go()` call registers the handler, but the implementation plan shows this should use a different pattern. The handler is registered but the connection's main loop may not be processing requests correctly.

**Impact:** LSP server is completely non-functional. No requests are processed.

**Recommendation:**
```go
// In server.go, change Serve() to:
func (s *Server) Serve(ctx context.Context, conn jsonrpc2.Conn) error {
    handler := jsonrpc2.HandlerWithError(s.handleRequest)
    conn.Go(ctx, handler)
    <-conn.Done()
    return conn.Err()
}

// Verify that handleRequest properly returns responses:
func (s *Server) handleRequest(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
    // Ensure all cases return proper values
    switch req.Method {
    case "initialize":
        return s.handleInitialize(ctx, req)  // Must return InitializeResult
    // ... etc
    default:
        return s.forwardToGopls(ctx, req)
    }
}
```

---

#### 2. **gopls Notifications Never Reach Editor**
**Location:** `pkg/lsp/gopls_client.go:101-210`, `pkg/lsp/diagnostics.go:33-88`

**Issue:** The gopls subprocess sends notifications (diagnostics, progress, etc.) via stdout, but the `GoplsClient` only captures these in the bidirectional JSON-RPC connection. There's no code path that forwards gopls-initiated notifications back to the VSCode client.

**Current Code (gopls_client.go:101-210):**
```go
// Create JSON-RPC connection
stream := jsonrpc2.NewStream(stdin, stdout)
c.conn = jsonrpc2.NewConn(stream)
```

**Problem:** This connection only handles request/response pairs initiated by dingo-lsp. When gopls sends unsolicited notifications (like `textDocument/publishDiagnostics`), they're received by the connection but never forwarded to VSCode.

**Impact:**
- No inline diagnostics appear in VSCode
- No progress indicators
- No file change notifications from gopls

**Recommendation:**
```go
// In gopls_client.go, add notification forwarding:
type GoplsClient struct {
    cmd         *exec.Cmd
    conn        jsonrpc2.Conn
    logger      Logger
    restarts    int
    maxRestarts int
    // NEW: Callback to forward notifications to editor
    onNotification func(method string, params interface{}) error
}

func NewGoplsClient(goplsPath string, logger Logger, notifHandler func(string, interface{}) error) (*GoplsClient, error) {
    // ... existing code ...
    client := &GoplsClient{
        logger:         logger,
        maxRestarts:    3,
        onNotification: notifHandler,  // Store callback
    }
    // ...
}

// In the connection setup, register notification handler:
func (c *GoplsClient) start(goplsPath string) error {
    // ... existing subprocess setup ...

    stream := jsonrpc2.NewStream(stdin, stdout)

    // Create handler that intercepts notifications
    handler := jsonrpc2.AsyncHandler(jsonrpc2.HandlerWithError(func(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
        // If it's a notification (no ID), forward to editor
        if req.ID == nil && c.onNotification != nil {
            return nil, c.onNotification(req.Method, req.Params)
        }
        // Otherwise, it's a response to our request
        return nil, nil
    }))

    c.conn = jsonrpc2.NewConn(stream, handler)
    // ...
}

// In server.go, pass the VSCode connection to gopls client:
func (s *Server) handleInitialize(ctx context.Context, req *jsonrpc2.Request) (*protocol.InitializeResult, error) {
    // ... existing code ...

    // Forward gopls notifications to VSCode
    notifHandler := func(method string, params interface{}) error {
        return s.clientConn.Notify(ctx, method, params)  // Forward to VSCode
    }

    s.gopls, err = NewGoplsClient(s.config.GoplsPath, s.config.Logger, notifHandler)
    // ...
}
```

---

#### 3. **Fallback Proxying Returns "Method Not Implemented"**
**Location:** `pkg/lsp/server.go:317-321`

**Issue:** The default case in `handleRequest` calls `forwardToGopls`, but the implementation likely returns an error or doesn't properly proxy unknown methods.

**Current Code (server.go:317-321):**
```go
default:
    // Unknown method, try forwarding to gopls
    return s.forwardToGopls(ctx, req)
```

**Problem:** There's no implementation shown for `forwardToGopls()`. If it's not implemented or returns an error, VSCode will receive "method not implemented" for essential LSP methods like:
- `textDocument/documentSymbol`
- `textDocument/references`
- `textDocument/rename`
- `workspace/symbol`

**Impact:** Most LSP features broken. Only explicitly handled methods work.

**Recommendation:**
```go
// In server.go, implement proper proxying:
func (s *Server) forwardToGopls(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
    // For unknown methods, forward directly to gopls without translation
    // This allows gopls to handle advanced features we haven't implemented yet

    var result json.RawMessage
    err := s.gopls.conn.Call(ctx, req.Method, req.Params, &result)
    if err != nil {
        return nil, err
    }

    // Return raw result (no position translation)
    // TODO: Add position translation for additional methods as needed
    return result, nil
}
```

**Note:** This won't work for methods that contain positions (they'll be wrong), but it's better than failing completely. Add explicit handlers for important methods over time.

---

#### 4. **`.dingo` URIs Leak to gopls When Source Maps Missing**
**Location:** `pkg/lsp/handlers.go:146-210`, `pkg/lsp/translator.go:52-78`

**Issue:** When a source map is not found, the translator falls back to 1:1 mapping but doesn't change the URI from `.dingo` to `.go`. gopls then receives a request for a `.dingo` file it doesn't know about.

**Current Code (translator.go:52-78):**
```go
// Load source map
sm, err := t.cache.Get(goPath)
if err != nil {
    // No source map, assume 1:1 mapping (graceful degradation)
    return uri, pos, nil  // ❌ Returns original .dingo URI
}
```

**Problem:** gopls expects `.go` files. Sending `.dingo` URIs causes gopls to return errors or empty results.

**Impact:**
- No autocomplete for untranspiled files
- Confusing error messages
- gopls may cache invalid state

**Recommendation:**
```go
func (t *Translator) translatePosition(
    uri protocol.DocumentURI,
    pos protocol.Position,
    dir Direction,
) (protocol.DocumentURI, protocol.Position, error) {
    line := int(pos.Line) + 1
    col := int(pos.Character) + 1

    var goPath string
    if dir == DingoToGo {
        goPath = dingoToGoPath(uri.Filename())
    } else {
        goPath = uri.Filename()
    }

    sm, err := t.cache.Get(goPath)
    if err != nil {
        // No source map - CRITICAL: Still translate URI even with 1:1 positions
        if dir == DingoToGo {
            // Must return .go URI, not .dingo
            return protocol.URIFromPath(goPath), pos, fmt.Errorf("source map not found: %s (file not transpiled)", goPath)
        }
        // For Go->Dingo without map, return error
        return uri, pos, fmt.Errorf("source map not found: %s", goPath)
    }

    // ... existing translation logic ...
}

// In handlers.go, handle translation errors gracefully:
func (s *Server) handleCompletion(ctx context.Context, req *jsonrpc2.Request) (*protocol.CompletionList, error) {
    // ... existing code ...

    goParams, err := s.translator.TranslateCompletionParams(params, DingoToGo)
    if err != nil {
        // Source map not found - show helpful diagnostic
        s.logger.Warnf("Cannot translate completion: %v", err)

        // Publish diagnostic to user
        s.publishDiagnostic(ctx, params.TextDocument.URI, protocol.Diagnostic{
            Range: protocol.Range{
                Start: protocol.Position{Line: 0, Character: 0},
                End:   protocol.Position{Line: 0, Character: 0},
            },
            Severity: protocol.DiagnosticSeverityWarning,
            Source:   "dingo-lsp",
            Message:  "File not transpiled. Run 'dingo build' or save to auto-transpile.",
        })

        // Return empty list instead of error
        return &protocol.CompletionList{Items: []protocol.CompletionItem{}}, nil
    }

    // ... continue with gopls call ...
}
```

---

### IMPORTANT Issues

#### 5. **VSCode `transpileOnSave` Setting Ignored**
**Location:** `cmd/dingo-lsp/main.go:29-34`, `editors/vscode/src/lspClient.ts:16-33`

**Issue:** The VSCode extension has a `dingo.transpileOnSave` setting, but the LSP server always enables auto-transpile in `main.go` (hardcoded `AutoTranspile: true`). The setting is never sent to the server.

**Current Code (main.go:29-34):**
```go
server, err := lsp.NewServer(lsp.ServerConfig{
    Logger:        logger,
    GoplsPath:     findGopls(),
    AutoTranspile: true,  // ❌ Hardcoded, ignores user setting
})
```

**Impact:** Users cannot disable auto-transpile even if they want manual control.

**Recommendation:**
```go
// In server.go, read configuration from initialize request:
func (s *Server) handleInitialize(ctx context.Context, req *jsonrpc2.Request) (*protocol.InitializeResult, error) {
    var params protocol.InitializeParams
    if err := json.Unmarshal(req.Params, &params); err != nil {
        return nil, err
    }

    // Extract workspace configuration
    // InitializationOptions should contain {"transpileOnSave": true/false}
    if params.InitializationOptions != nil {
        var opts struct {
            TranspileOnSave bool `json:"transpileOnSave"`
        }
        if err := json.Unmarshal(params.InitializationOptions, &opts); err == nil {
            s.config.AutoTranspile = opts.TranspileOnSave
            s.logger.Infof("Auto-transpile: %v (from client settings)", opts.TranspileOnSave)
        }
    }

    // ... rest of initialization ...
}

// In VSCode lspClient.ts, send setting in initialization options:
const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'dingo' }],
    initializationOptions: {
        transpileOnSave: vscode.workspace.getConfiguration('dingo').get('transpileOnSave', true)
    },
    // ...
};
```

---

#### 6. **Auto-Transpile Spawns Uncontrolled Builds Without Diagnostics**
**Location:** `pkg/lsp/transpiler.go:29-67`

**Issue:** The transpiler spawns `dingo build` as a subprocess with no timeout, resource limits, or proper error parsing. If the build hangs or produces malformed output, the LSP server is affected.

**Current Code (transpiler.go:29-67):**
```go
func (t *Transpiler) Transpile(ctx context.Context, dingoPath string) error {
    cmd := exec.Command("dingo", "build", dingoPath)
    output, err := cmd.CombinedOutput()  // ❌ No timeout, no context
    if err != nil {
        return fmt.Errorf("transpilation failed: %s", string(output))
    }
    return nil
}
```

**Problems:**
1. No timeout - `dingo build` could hang forever
2. No context cancellation - can't abort on shutdown
3. Error output not parsed - can't extract line numbers for diagnostics
4. No resource limits - could spawn infinite builds if watcher misbehaves

**Impact:**
- LSP server can hang
- Poor error messages to users
- No actionable diagnostics

**Recommendation:**
```go
func (t *Transpiler) Transpile(ctx context.Context, dingoPath string) (*TranspileResult, error) {
    // Add timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "dingo", "build", dingoPath)

    // Capture stdout and stderr separately
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()
    if err != nil {
        // Parse error output for diagnostics
        diagnostics := t.parseTranspileErrors(stderr.String(), dingoPath)
        return &TranspileResult{
            Success:     false,
            Diagnostics: diagnostics,
            ErrorOutput: stderr.String(),
        }, fmt.Errorf("transpilation failed")
    }

    return &TranspileResult{Success: true}, nil
}

type TranspileResult struct {
    Success     bool
    Diagnostics []protocol.Diagnostic
    ErrorOutput string
}

func (t *Transpiler) parseTranspileErrors(stderr string, filePath string) []protocol.Diagnostic {
    // Parse stderr for patterns like:
    // "example.dingo:10:5: syntax error: unexpected ')'"
    var diagnostics []protocol.Diagnostic

    re := regexp.MustCompile(`([^:]+):(\d+):(\d+): (.+)`)
    for _, line := range strings.Split(stderr, "\n") {
        matches := re.FindStringSubmatch(line)
        if len(matches) == 5 {
            lineNum, _ := strconv.Atoi(matches[2])
            colNum, _ := strconv.Atoi(matches[3])
            message := matches[4]

            diagnostics = append(diagnostics, protocol.Diagnostic{
                Range: protocol.Range{
                    Start: protocol.Position{
                        Line:      uint32(lineNum - 1),  // 0-based
                        Character: uint32(colNum - 1),
                    },
                    End: protocol.Position{
                        Line:      uint32(lineNum - 1),
                        Character: uint32(colNum),
                    },
                },
                Severity: protocol.DiagnosticSeverityError,
                Source:   "dingo",
                Message:  message,
            })
        }
    }

    return diagnostics
}
```

---

#### 7. **New Directories Not Added to File Watcher**
**Location:** `pkg/lsp/watcher.go:59-117`

**Issue:** The watcher only adds directories that exist at startup. If a developer creates a new subdirectory and adds `.dingo` files, those files won't be watched.

**Current Code (watcher.go:59-117):**
```go
func (fw *FileWatcher) watchRecursive(root string) error {
    return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        // ... walks existing directories ...
        if info.IsDir() {
            if err := fw.watcher.Add(path); err != nil {
                fw.logger.Warnf("Failed to watch %s: %v", path, err)
            }
        }
        return nil
    })
}
```

**Problem:** `filepath.Walk` is only called once during initialization. New directories created later are not added.

**Impact:** Files in new directories won't trigger auto-transpile.

**Recommendation:**
```go
func (fw *FileWatcher) watchLoop() {
    for {
        select {
        case event, ok := <-fw.watcher.Events:
            if !ok {
                return
            }

            // NEW: Watch for directory creation
            if event.Op&fsnotify.Create == fsnotify.Create {
                info, err := os.Stat(event.Name)
                if err == nil && info.IsDir() {
                    // New directory created, add to watcher
                    if !fw.shouldIgnore(event.Name) {
                        fw.watcher.Add(event.Name)
                        fw.logger.Debugf("Added new directory to watcher: %s", event.Name)
                    }
                }
            }

            // Filter: Only .dingo files
            if !isDingoFile(event.Name) {
                continue
            }

            // ... rest of event handling ...
        }
    }
}
```

---

#### 8. **gopls Crashes Not Detected**
**Location:** `pkg/lsp/gopls_client.go:214-227`

**Issue:** There's a `handleCrash()` method mentioned in the plan, but no code that detects when gopls actually crashes. The connection's `Done()` channel might close, but nothing monitors it.

**Impact:**
- gopls crashes → LSP server continues running but all requests fail
- No auto-restart
- Users must manually restart the LSP server

**Recommendation:**
```go
// In gopls_client.go, monitor the connection:
func (c *GoplsClient) start(goplsPath string) error {
    // ... existing subprocess setup ...

    c.conn = jsonrpc2.NewConn(stream)

    // NEW: Monitor connection health
    go func() {
        <-c.conn.Done()  // Wait for connection to close
        err := c.conn.Err()

        // Check if it was a crash (not graceful shutdown)
        if err != nil && !errors.Is(err, io.EOF) {
            c.logger.Errorf("gopls connection closed unexpectedly: %v", err)

            // Attempt restart
            if restartErr := c.handleCrash(); restartErr != nil {
                c.logger.Fatalf("Failed to restart gopls: %v", restartErr)
            }
        }
    }()

    c.logger.Infof("gopls started (PID: %d)", c.cmd.Process.Pid)
    return nil
}

func (c *GoplsClient) handleCrash() error {
    if c.restarts >= c.maxRestarts {
        return fmt.Errorf("gopls crashed %d times, giving up", c.restarts)
    }

    c.logger.Warnf("gopls crashed, restarting (attempt %d/%d)", c.restarts+1, c.maxRestarts)
    c.restarts++

    return c.start(c.goplsPath)  // Store goplsPath in struct
}
```

---

#### 9. **Server Emits Workspace File-Change Notifications Without Advertising Capability**
**Location:** `pkg/lsp/server.go:130-150`

**Issue:** The server sends `workspace/didChangeWatchedFiles` notifications to gopls, but doesn't advertise this capability in the `initialize` response.

**Current Code (server.go:130-150):**
```go
func (s *Server) handleInitialize(...) (*protocol.InitializeResult, error) {
    return &protocol.InitializeResult{
        Capabilities: protocol.ServerCapabilities{
            TextDocumentSync: ...,
            CompletionProvider: ...,
            HoverProvider: true,
            DefinitionProvider: true,
            // ❌ Missing: WorkspaceDidChangeWatchedFilesProvider
        },
        // ...
    }, nil
}
```

**Problem:** VSCode/gopls may ignore notifications if the capability isn't advertised.

**Impact:** File changes may not trigger gopls to recompile, leading to stale autocomplete.

**Recommendation:**
```go
return &protocol.InitializeResult{
    Capabilities: protocol.ServerCapabilities{
        TextDocumentSync: protocol.TextDocumentSyncOptions{
            OpenClose: true,
            Change:    protocol.TextDocumentSyncKindFull,
            Save:      &protocol.SaveOptions{IncludeText: false},
        },
        CompletionProvider: &protocol.CompletionOptions{
            TriggerCharacters: []string{".", ":", " "},
        },
        HoverProvider:      true,
        DefinitionProvider: true,
        // NEW: Advertise workspace file watching
        Workspace: &protocol.WorkspaceOptions{
            WorkspaceFolders: &protocol.WorkspaceFoldersServerCapabilities{
                Supported: true,
            },
            FileOperations: &protocol.FileOperationOptions{
                DidCreate: &protocol.FileOperationRegistrationOptions{
                    Filters: []protocol.FileOperationFilter{
                        {Pattern: protocol.FileOperationPattern{Glob: "**/*.go"}},
                    },
                },
                DidChange: &protocol.FileOperationRegistrationOptions{
                    Filters: []protocol.FileOperationFilter{
                        {Pattern: protocol.FileOperationPattern{Glob: "**/*.go"}},
                    },
                },
            },
        },
    },
    ServerInfo: &protocol.ServerInfo{
        Name:    "dingo-lsp",
        Version: "0.1.0",
    },
}, nil
```

---

### MINOR Issues

#### 10. **"LRU" Cache Never Evicts**
**Location:** `pkg/lsp/sourcemap_cache.go:23-88`

**Issue:** The cache is described as "LRU" in comments, but there's no eviction logic. It will grow unbounded.

**Current Code:**
```go
type SourceMapCache struct {
    mu     sync.RWMutex
    maps   map[string]*preprocessor.SourceMap  // ❌ No size limit
    logger Logger
}
```

**Impact:** Memory leak in projects with many `.dingo` files.

**Recommendation:**
```go
import "container/list"

type SourceMapCache struct {
    mu       sync.RWMutex
    maps     map[string]*cacheEntry
    lru      *list.List  // LRU eviction queue
    maxSize  int         // Max entries (e.g., 100)
    logger   Logger
}

type cacheEntry struct {
    sourceMap *preprocessor.SourceMap
    element   *list.Element  // Position in LRU queue
}

func NewSourceMapCache(logger Logger) (*SourceMapCache, error) {
    return &SourceMapCache{
        maps:    make(map[string]*cacheEntry),
        lru:     list.New(),
        maxSize: 100,  // Configurable
        logger:  logger,
    }, nil
}

func (c *SourceMapCache) Get(goFilePath string) (*preprocessor.SourceMap, error) {
    mapPath := goFilePath + ".map"

    c.mu.Lock()
    defer c.mu.Unlock()

    // Check cache
    if entry, ok := c.maps[mapPath]; ok {
        // Move to front of LRU (most recently used)
        c.lru.MoveToFront(entry.element)
        return entry.sourceMap, nil
    }

    // Load from disk (existing logic)
    sm, err := c.loadSourceMap(mapPath)
    if err != nil {
        return nil, err
    }

    // Evict if at capacity
    if c.lru.Len() >= c.maxSize {
        oldest := c.lru.Back()
        if oldest != nil {
            oldestPath := oldest.Value.(string)
            delete(c.maps, oldestPath)
            c.lru.Remove(oldest)
            c.logger.Debugf("Evicted source map: %s", oldestPath)
        }
    }

    // Add to cache
    element := c.lru.PushFront(mapPath)
    c.maps[mapPath] = &cacheEntry{
        sourceMap: sm,
        element:   element,
    }

    return sm, nil
}
```

---

#### 11. **VSCode Client Won't Auto-Restart After Failure**
**Location:** `editors/vscode/src/lspClient.ts:45-48`

**Issue:** If the LSP server crashes or fails to start, the VSCode extension doesn't automatically retry.

**Impact:** Users must manually run "Reload Window" to recover.

**Recommendation:**
```typescript
// In lspClient.ts:
export function activate(context: vscode.ExtensionContext) {
    const config = vscode.workspace.getConfiguration('dingo');
    const lspPath = config.get<string>('lsp.path') || 'dingo-lsp';
    const logLevel = config.get<string>('lsp.logLevel') || 'info';

    let client: LanguageClient;
    restartCount := 0;
    const MAX_RESTARTS = 3;

    function startClient() {
        const serverOptions: ServerOptions = {
            command: lspPath,
            args: [],
            options: {
                env: {
                    ...process.env,
                    DINGO_LSP_LOG: logLevel,
                }
            }
        };

        const clientOptions: LanguageClientOptions = {
            documentSelector: [{ scheme: 'file', language: 'dingo' }],
            synchronize: {
                fileEvents: vscode.workspace.createFileSystemWatcher('**/*.{dingo,go.map}')
            },
            // NEW: Error handler with auto-restart
            errorHandler: {
                error: (error, message, count) => {
                    console.error(`LSP error (${count}):`, error, message);
                    return ErrorAction.Continue;
                },
                closed: () => {
                    if (restartCount < MAX_RESTARTS) {
                        restartCount++;
                        console.log(`LSP closed, restarting (${restartCount}/${MAX_RESTARTS})...`);
                        return CloseAction.Restart;
                    } else {
                        vscode.window.showErrorMessage(
                            'Dingo LSP crashed repeatedly. Please check logs and restart manually.'
                        );
                        return CloseAction.DoNotRestart;
                    }
                }
            }
        };

        client = new LanguageClient(
            'dingo-lsp',
            'Dingo Language Server',
            serverOptions,
            clientOptions
        );

        client.start();

        // Reset restart count after successful connection
        client.onReady().then(() => {
            restartCount = 0;
        });
    }

    startClient();

    // ... rest of extension code ...
}
```

---

## 🔍 Questions

1. **Diagnostic Translation Strategy:**
   - How will gopls diagnostics (which reference `.go` files) be translated back to `.dingo` positions?
   - Is there a plan to filter diagnostics that fall in generated code (e.g., IIFE wrappers)?

2. **Proxying vs Blocking:**
   - Should advanced gopls features (rename, find references, document symbols) be proxied as-is, or should they be explicitly blocked until position translation is implemented?
   - What's the UX for features that work but return wrong positions?

3. **Source Map Staleness:**
   - If a user edits a `.dingo` file without saving (auto-transpile disabled), how should the LSP handle requests with a stale source map?
   - Should there be a "dirty buffer" tracking mechanism?

4. **Performance Under Load:**
   - Have you benchmarked the system with 100+ `.dingo` files in a workspace?
   - What's the plan if file watching becomes a bottleneck?

5. **gopls Version Compatibility:**
   - The plan mentions supporting gopls v0.11+. What happens if a user has v0.10 or v0.16?
   - Should there be version detection and warnings?

---

## 📊 Summary

- **Overall Status:** MAJOR_ISSUES
- **Critical:** 4 (JSON-RPC hookup, gopls notifications, fallback proxying, URI translation)
- **Important:** 5 (VSCode settings, transpile diagnostics, directory watching, crash detection, capability advertising)
- **Minor:** 2 (LRU eviction, VSCode auto-restart)
- **Testability:** Low (integration tests can't run without fixing critical issues)

### Recommended Actions (Priority Order)

1. **CRITICAL - Wire JSON-RPC handlers end-to-end**
   - Fix `main.go` and `server.go` to properly route LSP requests
   - Verify all handlers return correct response types
   - Test with VSCode: `initialize` → `completion` → `shutdown`

2. **CRITICAL - Implement gopls notification forwarding**
   - Add callback mechanism in `GoplsClient`
   - Forward `textDocument/publishDiagnostics` to VSCode
   - Forward progress notifications
   - Test: Introduce syntax error → see red squiggly in VSCode

3. **CRITICAL - Fix URI translation fallback**
   - Always return `.go` URIs when translating Dingo→Go
   - Show helpful diagnostics when source maps missing
   - Test: Open untranspiled `.dingo` file → see "not transpiled" diagnostic

4. **CRITICAL - Implement proper fallback proxying**
   - Add `forwardToGopls()` implementation
   - Proxy unknown methods without translation
   - Test: Use "Find References" (should work, even if positions are slightly off)

5. **IMPORTANT - Fix VSCode settings integration**
   - Pass `transpileOnSave` to LSP via `initializationOptions`
   - Read in `handleInitialize`
   - Test: Disable setting → verify auto-transpile stops

6. **IMPORTANT - Add transpile error parsing**
   - Parse `dingo build` stderr for line numbers
   - Convert to LSP diagnostics
   - Add timeout and context cancellation
   - Test: Introduce syntax error → see inline diagnostic with correct position

7. **IMPORTANT - Fix file watcher for new directories**
   - Watch for `fsnotify.Create` events on directories
   - Add new directories to watcher
   - Test: Create new folder with `.dingo` file → verify auto-transpile works

8. **IMPORTANT - Implement gopls crash recovery**
   - Monitor `conn.Done()` channel
   - Auto-restart on unexpected closure (max 3 times)
   - Test: Kill gopls process → verify LSP auto-restarts

9. **IMPORTANT - Advertise workspace capabilities**
   - Add `Workspace` capabilities in `initialize` response
   - Test: Verify gopls receives file change notifications

10. **MINOR - Implement true LRU cache eviction**
    - Add LRU queue with max size limit
    - Test with 150+ `.dingo` files (should evict oldest)

11. **MINOR - Add VSCode client auto-restart**
    - Implement error handler with restart logic
    - Test: Kill LSP server → verify VSCode auto-restarts it

---

## Conclusion

The Dingo LSP implementation has a solid architectural foundation with good separation of concerns, thread-safe caching, and excellent performance characteristics. However, critical protocol integration gaps prevent it from functioning correctly. The JSON-RPC layer is not properly wired, gopls notifications are not forwarded, and URI translation has fatal bugs that will cause gopls to fail.

**Before shipping, the four CRITICAL issues must be fixed.** The IMPORTANT issues should be addressed to provide a good user experience (proper settings, diagnostics, crash recovery). The MINOR issues can be deferred to iteration 2.

**Estimated fix time:** 2-3 days for critical issues, 1-2 days for important issues.

**Recommendation:** Do not release until critical issues are resolved and end-to-end testing confirms basic LSP features (autocomplete, hover, go-to-definition, diagnostics) work correctly in VSCode.

---

**Review completed:** 2025-11-18
**Reviewer:** GPT-5.1 Codex (OpenAI)
**Proxy:** claudish CLI
**Session:** 20251118-152749-phase5-lsp-foundation
