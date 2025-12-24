package semantic

import (
	"fmt"
	goparser "go/parser"
	"go/token"
	"go/types"
	"sync"
	"time"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/typechecker"
	"go.lsp.dev/protocol"
)

// Document holds type information for a single Dingo file
type Document struct {
	URI     protocol.DocumentURI
	Version int32

	// Dingo source
	DingoSource []byte

	// Type information
	SemanticMap *Map
	TypesPkg    *types.Package

	// Build metadata
	BuildTime  time.Time
	BuildError error // Non-nil if last build failed
}

// Manager manages Document instances with debounced rebuilding
type Manager struct {
	mu     sync.RWMutex
	docs   map[string]*Document
	logger Logger

	// Debounce control
	debounceMu     sync.Mutex
	debounceTimers map[string]*time.Timer
	debounceDelay  time.Duration // 500ms default

	// Transpiler function
	transpiler TranspileFunc
}

// TranspileFunc is the signature for transpilation
// Returns transpiled Go code, line mappings, and error
type TranspileFunc func(source []byte, filename string) (TranspileResult, error)

// TranspileResult contains the results of transpilation
type TranspileResult struct {
	GoCode         []byte
	LineMappings   []sourcemap.LineMapping
	ColumnMappings []sourcemap.ColumnMapping // For accurate hover column translation
	DingoFset      *token.FileSet
	DingoFile      string
}

// Logger interface for the manager
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// NewManager creates a Manager with 500ms debounce
func NewManager(logger Logger, transpiler TranspileFunc) *Manager {
	return &Manager{
		docs:           make(map[string]*Document),
		logger:         logger,
		debounceTimers: make(map[string]*time.Timer),
		debounceDelay:  500 * time.Millisecond,
		transpiler:     transpiler,
	}
}

// Get returns the Document for a URI
// Returns cached document immediately (may be stale during typing)
// Triggers async rebuild if source changed
// Note: Document fields are immutable after creation, so reads are safe
// even after we release the lock (we never modify, only replace entire Document)
func (m *Manager) Get(uri string, source []byte) (*Document, error) {
	m.mu.RLock()
	doc, exists := m.docs[uri]
	// Capture document state while holding lock
	// Safe because Document is immutable - we replace entire doc on rebuild
	var cachedSource []byte
	var buildErr error
	if exists {
		cachedSource = doc.DingoSource
		buildErr = doc.BuildError
	}
	m.mu.RUnlock()

	// If document exists and source hasn't changed, return cached version
	if exists && string(cachedSource) == string(source) {
		return doc, buildErr
	}

	// If document doesn't exist or source changed, trigger rebuild
	// but return cached version immediately if available
	if exists {
		// Trigger async rebuild with debounce
		m.Invalidate(uri, source)
		return doc, buildErr
	}

	// No cached version - must build synchronously
	m.rebuild(uri, source)

	m.mu.RLock()
	doc = m.docs[uri]
	if doc != nil {
		buildErr = doc.BuildError
	}
	m.mu.RUnlock()

	if doc == nil {
		return nil, fmt.Errorf("rebuild failed for %s", uri)
	}

	return doc, buildErr
}

// Invalidate marks a document as needing rebuild
// Starts debounce timer (500ms)
func (m *Manager) Invalidate(uri string, source []byte) {
	m.debounceMu.Lock()
	defer m.debounceMu.Unlock()

	// Cancel existing timer if any
	if timer, exists := m.debounceTimers[uri]; exists {
		timer.Stop()
	}

	// Create new debounce timer
	m.debounceTimers[uri] = time.AfterFunc(m.debounceDelay, func() {
		m.rebuild(uri, source)

		// Clean up timer
		m.debounceMu.Lock()
		delete(m.debounceTimers, uri)
		m.debounceMu.Unlock()
	})

	m.logger.Debugf("Scheduled debounced rebuild for %s in %v", uri, m.debounceDelay)
}

// Rebuild forces immediate rebuild (for didSave)
func (m *Manager) Rebuild(uri string, source []byte) {
	m.debounceMu.Lock()
	// Cancel any pending debounced rebuild
	if timer, exists := m.debounceTimers[uri]; exists {
		timer.Stop()
		delete(m.debounceTimers, uri)
	}
	m.debounceMu.Unlock()

	// Perform immediate rebuild
	m.rebuild(uri, source)
}

// rebuild performs the actual rebuild (called after debounce or immediately)
// Steps:
// 1. Transpile Dingo to Go
// 2. Parse Go AST
// 3. Run go/types
// 4. Build SemanticMap via Builder
// 5. Cache Document
func (m *Manager) rebuild(uri string, source []byte) {
	startTime := time.Now()
	m.logger.Debugf("Starting semantic rebuild for %s", uri)

	doc := &Document{
		URI:         protocol.DocumentURI(uri),
		DingoSource: source,
		BuildTime:   startTime,
	}

	// Step 1: Transpile Dingo to Go
	result, err := m.transpiler(source, uri)
	if err != nil {
		m.logger.Warnf("Transpilation failed for %s: %v", uri, err)
		doc.BuildError = fmt.Errorf("transpilation failed: %w", err)
		m.storeDocument(uri, doc)
		return
	}

	// Step 2: Parse Go AST
	goFset := token.NewFileSet()

	goAST, err := goparser.ParseFile(goFset, uri+".go", result.GoCode, goparser.ParseComments)
	if err != nil {
		m.logger.Warnf("Go parsing failed for %s: %v", uri, err)
		doc.BuildError = fmt.Errorf("Go parsing failed: %w", err)
		m.storeDocument(uri, doc)
		return
	}

	// Step 3: Run go/types
	// Extract package name from AST
	pkgName := "main"
	if goAST.Name != nil {
		pkgName = goAST.Name.Name
	}
	checker, err := typechecker.New(goFset, goAST, pkgName)
	if err != nil {
		// Type checking errors are non-fatal - we can still build semantic map
		// with partial type information
		m.logger.Debugf("Type checking completed with errors for %s: %v", uri, err)
	}

	var typesInfo *types.Info
	var typesPkg *types.Package
	if checker != nil {
		typesInfo = checker.Info()
		typesPkg = checker.Package()
	}

	doc.TypesPkg = typesPkg

	// Step 4: Build SemanticMap via Builder
	builder := NewBuilder(
		goAST,
		goFset,
		typesInfo,
		result.LineMappings,
		result.ColumnMappings,
		source,
		result.DingoFset,
		result.DingoFile,
	)

	semanticMap, err := builder.Build()
	if err != nil {
		m.logger.Warnf("Semantic map build failed for %s: %v", uri, err)
		doc.BuildError = fmt.Errorf("semantic map build failed: %w", err)
		m.storeDocument(uri, doc)
		return
	}

	doc.SemanticMap = semanticMap
	doc.BuildError = nil // Clear any previous errors

	// Step 5: Cache Document
	m.storeDocument(uri, doc)

	duration := time.Since(startTime)
	m.logger.Infof("Semantic rebuild completed for %s in %v (%d entities)",
		uri, duration, semanticMap.Count())
}

// storeDocument stores a document in the cache (thread-safe)
func (m *Manager) storeDocument(uri string, doc *Document) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs[uri] = doc
}
