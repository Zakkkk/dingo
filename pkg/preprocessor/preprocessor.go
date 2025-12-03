// Package preprocessor transforms Dingo syntax to valid Go syntax
package preprocessor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/typeloader"
)

// Preprocessor orchestrates multiple feature processors to transform
// Dingo source code into valid Go code with semantic placeholders
type Preprocessor struct {
	source       []byte
	processors   []FeatureProcessor
	oldConfig    *Config        // Deprecated: Legacy preprocessor-specific config (STILL USED for config propagation)
	config       *config.Config // Main Dingo configuration
	legacyConfig *Config        // Legacy config for processor configuration (e.g., MultiValueReturnMode)

	// Package-wide cache (optional, for unqualified import inference)
	// When present, enables early bailout optimization and local function exclusion
	cache *FunctionExclusionCache
}

// TransformMetadata holds metadata about a transformation (NOT final mappings)
// This is emitted by preprocessors and used by Post-AST generator to match AST nodes
type TransformMetadata struct {
	Type            string // "error_prop", "type_annot", "enum", etc.
	OriginalLine    int    // Line in .dingo file
	OriginalColumn  int    // Column in .dingo file
	OriginalLength  int    // Length in .dingo file
	OriginalText    string // Original Dingo syntax (e.g., "?")
	GeneratedMarker string // Unique marker in Go code (e.g., "// dingo:e:0")
	ASTNodeType     string // "CallExpr", "FuncDecl", "IfStmt", etc.
}

// ProcessResult holds the result of preprocessing
// Supports both legacy mappings and new Post-AST metadata
type ProcessResult struct {
	Source   []byte              // Transformed Go source code
	Mappings []Mapping           // LEGACY: For backward compatibility
	Metadata []TransformMetadata // NEW: For Post-AST generation
}

// FeatureProcessor defines the interface for individual feature preprocessors
type FeatureProcessor interface {
	// Name returns the feature name for logging/debugging
	Name() string

	// Process transforms the source code and returns:
	// - transformed source
	// - source mappings for error reporting
	// - error if transformation failed
	Process(source []byte) ([]byte, []Mapping, error)
}

// FeatureProcessorV2 is the new interface that supports metadata emission
// Processors can implement this interface to support metadata-based source map generation
type FeatureProcessorV2 interface {
	FeatureProcessor // Embed the old interface for backward compatibility

	// ProcessV2 transforms the source code and returns a ProcessResult
	// This method supports metadata generation
	ProcessV2(source []byte) (ProcessResult, error)
}

// ImportProvider is an optional interface for processors that need to add imports
type ImportProvider interface {
	// GetNeededImports returns list of import paths that should be added
	GetNeededImports() []string
}

// New creates a new preprocessor with all registered features and default config
func New(source []byte) *Preprocessor {
	return NewWithMainConfig(source, nil)
}

// NewWithConfig creates a new preprocessor with legacy config (deprecated)
// Use NewWithMainConfig instead
func NewWithConfig(source []byte, legacyConfig *Config) *Preprocessor {
	// Convert legacy config to main config
	cfg := config.DefaultConfig()
	if legacyConfig != nil && legacyConfig.MultiValueReturnMode == "single" {
		// Map legacy mode to main config (feature not in main config yet)
	}
	return newWithConfigAndCacheAndLegacy(source, cfg, nil, legacyConfig)
}

// NewWithMainConfig creates a new preprocessor with main Dingo configuration
func NewWithMainConfig(source []byte, cfg *config.Config) *Preprocessor {
	return newWithConfigAndCache(source, cfg, nil)
}

// NewWithTypeLoading creates a preprocessor with dynamic type loading
// Uses BuildCache for efficient multi-file builds
// Fails fast on any type loading error with clear error message
func NewWithTypeLoading(source []byte, cfg *config.Config, cache *typeloader.BuildCache) (*Preprocessor, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Stage 0: Extract imports from source
	imports, err := typeloader.ExtractImports(source)
	if err != nil {
		return nil, fmt.Errorf("failed to extract imports: %w", err)
	}

	// Stage 0: Load types using cache (or create new loader if no cache)
	var loadResult *typeloader.LoadResult
	if cache != nil {
		loadResult, err = cache.LoadImports(imports)
		if err != nil {
			// Fail fast with clear error
			return nil, fmt.Errorf("type loading failed: %w", err)
		}
	} else {
		// No cache - create loader and load directly
		loader := typeloader.NewLoader(typeloader.LoaderConfig{
			WorkingDir: "", // Empty string defaults to current directory
			FailFast:   true,
		})
		loadResult, err = loader.LoadFromImports(imports)
		if err != nil {
			return nil, fmt.Errorf("type loading failed: %w", err)
		}
	}

	// Parse local functions and merge into result
	localParser := &typeloader.LocalFuncParser{}
	localFuncs, _ := localParser.ParseLocalFunctions(source)
	if localFuncs != nil {
		// Merge local functions into loadResult
		if loadResult.LocalFunctions == nil {
			loadResult.LocalFunctions = make(map[string]*typeloader.FunctionSignature)
		}
		for k, v := range localFuncs {
			loadResult.LocalFunctions[k] = v
		}
	}

	// Create preprocessor with standard configuration
	p := newWithConfigAndCache(source, cfg, nil)

	// Wire LoadResult to ReturnDetector in ErrorPropASTProcessor
	// Need to find the ErrorPropASTProcessor in the preprocessor's processor list
	for _, proc := range p.processors {
		if errorPropProc, ok := proc.(*ErrorPropASTProcessor); ok {
			// Create ReturnDetector with nil analyzer (will use heuristics)
			returnDetector := NewReturnDetector(nil)
			// Wire the loadResult to the detector
			returnDetector.SetLoadResult(loadResult)
			// Set the detector on the processor
			errorPropProc.SetReturnDetector(returnDetector)
			break
		}
	}

	return p, nil
}

// newWithConfigAndCache is the internal constructor that accepts an optional cache
func newWithConfigAndCache(source []byte, cfg *config.Config, cache *FunctionExclusionCache) *Preprocessor {
	return newWithConfigAndCacheAndLegacy(source, cfg, cache, nil)
}

// newWithConfigAndCacheAndLegacy is the internal constructor with legacy config support
func newWithConfigAndCacheAndLegacy(source []byte, cfg *config.Config, cache *FunctionExclusionCache, legacyConfig *Config) *Preprocessor {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Use the new two-pass configuration from pass_config.go
	// This ensures consistent ordering: Structural → Lambda → Expression
	passConfig := NewDefaultPassConfig()
	processors := passConfig.AllProcessorsWithLambda()

	// If legacy config is provided, replace the ErrorPropASTProcessor with configured version
	if legacyConfig != nil {
		// Find and replace ErrorPropASTProcessor in the processors list
		for i, proc := range processors {
			if proc.Name() == "error_propagation_ast" {
				processors[i] = NewErrorPropASTProcessorWithConfig(legacyConfig)
				break
			}
		}
	}

	// REMOVED: KeywordProcessor - REPLACED by DingoPreParser (position 0)
	// DingoPreParser handles let declarations with full AST-based parsing
	// processors = append(processors, NewKeywordProcessor())

	// 13. Unqualified imports (ReadFile → os.ReadFile) - requires cache
	//     MIGRATED TO AST: Uses proper AST-based parsing instead of regex
	if cache != nil {
		processors = append(processors, NewUnqualifiedImportProcessorAST(cache))
	}

	return &Preprocessor{
		source:       source,
		config:       cfg,
		oldConfig:    nil, // No longer used
		legacyConfig: legacyConfig,
		processors:   processors,
		cache:        cache,
	}
}

// ProcessWithMetadata runs all feature processors and returns both legacy mappings and metadata
// ARCHITECTURE: Two-Pass Processing
//
//	Pass 1: Structural transforms (enum, tuple, pattern matching) - change code structure
//	Lambda: Between passes - processes body content with injected expression processors
//	Pass 2: Expression transforms (?, ?., ??, ? :) - within expressions
func (p *Preprocessor) ProcessWithMetadata() (string, *SourceMap, []TransformMetadata, error) {
	// Early bailout optimization (GPT-5.1): If cache indicates no unqualified imports
	// in this package, skip expensive symbol resolution for unqualified import processors
	skipUnqualifiedProcessing := false
	if p.cache != nil && !p.cache.HasUnqualifiedImports() {
		// This package has no unqualified stdlib calls, skip that processing
		skipUnqualifiedProcessing = true
	}
	_ = skipUnqualifiedProcessing // TODO: Use when UnqualifiedImportProcessor is integrated

	result := p.source
	sourceMap := NewSourceMap()
	allMetadata := []TransformMetadata{}
	neededImports := []string{}

	// Get two-pass configuration
	config := NewDefaultPassConfig()

	// Apply legacy config to expression processors (if provided)
	if p.legacyConfig != nil {
		// Replace ErrorPropASTProcessor with configured version
		for i, proc := range config.Expression {
			if proc.Name() == "error_propagation_ast" {
				config.Expression[i] = NewErrorPropASTProcessorWithConfig(p.legacyConfig)
				break
			}
		}
	}

	// Wire ReturnDetector into ErrorPropASTProcessor for accurate return type detection
	// The processor is at index 5 (last one) in expression processors
	// ReturnDetector enables error-only vs (value, error) distinction
	if errorPropProc, ok := config.Expression[5].(*ErrorPropASTProcessor); ok {
		// Create ReturnDetector with nil analyzer (will use heuristics-based detection)
		// Type-based analysis requires valid Go source, which we don't have yet
		// Heuristics cover common stdlib patterns (os.Open, rows.Scan, etc.)
		returnDetector := NewReturnDetector(nil)
		errorPropProc.SetReturnDetector(returnDetector)
	}

	// Get body processors for lambda injection (expression processors that implement BodyProcessor)
	bodyProcessors := config.GetBodyProcessors()

	// Create lambda processor with injected body processors (decoupled architecture)
	lambdaProc := NewLambdaASTProcessorWithBodyProcessors(bodyProcessors)

	// Helper function to process with a single processor
	processOne := func(proc FeatureProcessor, input []byte) ([]byte, error) {
		// Check if processor implements V2 interface
		if procV2, ok := proc.(FeatureProcessorV2); ok {
			// Use new ProcessV2 method
			procResult, err := procV2.ProcessV2(input)
			if err != nil {
				return nil, fmt.Errorf("%s preprocessing failed: %w", proc.Name(), err)
			}

			// Merge mappings (legacy support)
			for _, m := range procResult.Mappings {
				sourceMap.AddMapping(m)
			}

			// Collect metadata
			allMetadata = append(allMetadata, procResult.Metadata...)

			return procResult.Source, nil
		} else {
			// Fall back to legacy Process method
			processed, mappings, err := proc.Process(input)
			if err != nil {
				return nil, fmt.Errorf("%s preprocessing failed: %w", proc.Name(), err)
			}

			// Merge mappings
			for _, m := range mappings {
				sourceMap.AddMapping(m)
			}

			return processed, nil
		}
	}

	// ========== PASS 1: Structural Transforms ==========
	// These transforms change code structure (enums → structs, tuples → multiple vars, etc.)
	for _, proc := range config.Structural {
		var err error
		result, err = processOne(proc, result)
		if err != nil {
			return "", nil, nil, fmt.Errorf("pass 1: %w", err)
		}

		// Collect needed imports if processor implements ImportProvider
		if importProvider, ok := proc.(ImportProvider); ok {
			imports := importProvider.GetNeededImports()
			neededImports = append(neededImports, imports...)
		}
	}

	// ========== LAMBDA PROCESSING (Between Passes) ==========
	// Lambda processor runs between passes and internally applies expression transforms to bodies
	// This ensures lambda body expressions are processed uniformly with outer code
	var err error
	// Cast to FeatureProcessor interface
	var lambdaProcInterface FeatureProcessor = lambdaProc
	result, err = processOne(lambdaProcInterface, result)
	if err != nil {
		return "", nil, nil, fmt.Errorf("lambda processing: %w", err)
	}

	// Collect imports from lambda processor
	if importProvider, ok := lambdaProcInterface.(ImportProvider); ok {
		imports := importProvider.GetNeededImports()
		neededImports = append(neededImports, imports...)
	}

	// ========== TUPLE PROCESSING (After Lambda, Before Expression) ==========
	// Tuple processor runs after Lambda to avoid matching lambda param lists as tuples
	// Handles: let (a, b) = expr → destructuring, (x, y) → tuple literals
	tupleProc := NewTupleProcessor()
	var tupleProcInterface FeatureProcessor = tupleProc
	result, err = processOne(tupleProcInterface, result)
	if err != nil {
		return "", nil, nil, fmt.Errorf("tuple processing: %w", err)
	}

	// Collect imports from tuple processor
	if importProvider, ok := tupleProcInterface.(ImportProvider); ok {
		imports := importProvider.GetNeededImports()
		neededImports = append(neededImports, imports...)
	}

	// ========== PASS 2: Expression Transforms ==========
	// These transforms operate within expressions (error prop, safe nav, null coalesce, ternary)
	// They process code OUTSIDE lambda bodies (lambda body expressions already processed)
	for _, proc := range config.Expression {
		result, err = processOne(proc, result)
		if err != nil {
			return "", nil, nil, fmt.Errorf("pass 2: %w", err)
		}

		// Collect needed imports if processor implements ImportProvider
		if importProvider, ok := proc.(ImportProvider); ok {
			imports := importProvider.GetNeededImports()
			neededImports = append(neededImports, imports...)
		}
	}

	// ========== OPTIONAL: Unqualified Imports ==========
	// Unqualified imports (ReadFile → os.ReadFile) - only when cache is available
	// This runs LAST after all other transforms (needs final AST structure)
	if p.cache != nil {
		unqualProc := NewUnqualifiedImportProcessorAST(p.cache)
		var unqualProcInterface FeatureProcessor = unqualProc
		result, err = processOne(unqualProcInterface, result)
		if err != nil {
			return "", nil, nil, fmt.Errorf("unqualified imports: %w", err)
		}

		// Collect imports from unqualified processor
		if importProvider, ok := unqualProcInterface.(ImportProvider); ok {
			imports := importProvider.GetNeededImports()
			neededImports = append(neededImports, imports...)
		}
	}

	// Convert metadata to legacy mappings for backward compatibility
	// This allows tests and tools that expect mappings to continue working
	// while we migrate to the new metadata-based approach
	if len(allMetadata) > 0 {
		legacyMappings := convertMetadataToMappings(allMetadata, result)
		for _, m := range legacyMappings {
			sourceMap.AddMapping(m)
		}
	}

	// Inject all needed imports at the end (after all transformations complete)
	if len(neededImports) > 0 {
		var importInsertLine, importBlockEndLine int
		var err error
		// CRITICAL FIX: Get both import start and end lines for accurate shifting
		result, importInsertLine, importBlockEndLine, err = injectImportsWithPosition(result, neededImports)
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to inject imports: %w", err)
		}

		// Calculate how many lines the import block occupies
		// importInsertLine is where imports are inserted (after package declaration)
		// importBlockEndLine is where imports end (last line of import block)
		// CRITICAL FIX: Only apply adjustment if imports were actually added
		if importInsertLine > 0 && importBlockEndLine > 0 {
			importBlockSize := importBlockEndLine - importInsertLine + 1

			// Adjust all source mappings to account for added import lines
			adjustMappingsForImports(sourceMap, importBlockSize, importInsertLine)

			// TODO: Adjust metadata line numbers
			// This will be needed when we integrate metadata-based source maps
		}
	}

	return string(result), sourceMap, allMetadata, nil
}

// Process runs all feature processors in sequence and combines source maps
// This is the legacy method that returns only source maps (for backward compatibility)
func (p *Preprocessor) Process() (string, *SourceMap, error) {
	// Delegate to ProcessWithMetadata and discard metadata
	result, sourceMap, _, err := p.ProcessWithMetadata()
	return result, sourceMap, err
}

// DEPRECATED: Old Process implementation kept for reference during migration
// Will be removed after all callers migrate to ProcessWithMetadata
func (p *Preprocessor) processLegacy() (string, *SourceMap, error) {
	// Early bailout optimization (GPT-5.1): If cache indicates no unqualified imports
	// in this package, skip expensive symbol resolution for unqualified import processors
	skipUnqualifiedProcessing := false
	if p.cache != nil && !p.cache.HasUnqualifiedImports() {
		// This package has no unqualified stdlib calls, skip that processing
		skipUnqualifiedProcessing = true
	}
	_ = skipUnqualifiedProcessing // TODO: Use when UnqualifiedImportProcessor is integrated

	result := p.source
	sourceMap := NewSourceMap()
	neededImports := []string{}

	// Run each processor in sequence
	for _, proc := range p.processors {
		processed, mappings, err := proc.Process(result)
		if err != nil {
			return "", nil, fmt.Errorf("%s preprocessing failed: %w", proc.Name(), err)
		}

		// Update result
		result = processed

		// Merge mappings
		for _, m := range mappings {
			sourceMap.AddMapping(m)
		}

		// Collect needed imports if processor implements ImportProvider
		if importProvider, ok := proc.(ImportProvider); ok {
			imports := importProvider.GetNeededImports()
			neededImports = append(neededImports, imports...)
		}
	}

	// Inject all needed imports at the end (after all transformations complete)
	if len(neededImports) > 0 {
		var importInsertLine, importBlockEndLine int
		var err error
		// CRITICAL FIX: Get both import start and end lines for accurate shifting
		result, importInsertLine, importBlockEndLine, err = injectImportsWithPosition(result, neededImports)
		if err != nil {
			return "", nil, fmt.Errorf("failed to inject imports: %w", err)
		}

		// Calculate how many lines the import block occupies
		// importInsertLine is where imports are inserted (after package declaration)
		// importBlockEndLine is where imports end (last line of import block)
		// CRITICAL FIX: Only apply adjustment if imports were actually added
		if importInsertLine > 0 && importBlockEndLine > 0 {
			// Calculate the number of lines added by the import block
			//
			// Example - multi-line import:
			// BEFORE import injection (preprocessed code):
			//   Line 1: package main
			//   Line 2: [blank]
			//   Line 3: func readConfig(...) {
			//   Line 4:     tmp, err := os.ReadFile(path)  ← mapping says gen_line=4
			//
			// AFTER import injection:
			//   Line 1: package main
			//   Line 2: [blank]
			//   Line 3: import (             ← importInsertLine is BEFORE this (line 2)
			//   Line 4:     "os"
			//   Line 5: )                    ← importBlockEndLine = 5
			//   Line 6: [blank line added by go/printer]
			//   Line 7: func readConfig(...) {
			//   Line 8:     tmp, err := os.ReadFile(path)  ← should be gen_line=8
			//
			// Calculation:
			//   importInsertLine = 2 (line after package, before imports start)
			//   importBlockEndLine = 5 (last line of import block)
			//   Shift needed = 8 - 4 = 4 lines
			//   Formula: importBlockEndLine - importInsertLine + 1 = 5 - 2 + 1 = 4 ✓
			//
			// The +1 accounts for the blank line that go/printer adds after the import block
			importBlockSize := importBlockEndLine - importInsertLine + 1

			// Adjust all source mappings to account for added import lines
			adjustMappingsForImports(sourceMap, importBlockSize, importInsertLine)
		}
	}

	return string(result), sourceMap, nil
}

// ProcessBytes is like Process but returns bytes
func (p *Preprocessor) ProcessBytes() ([]byte, *SourceMap, error) {
	str, sm, err := p.Process()
	if err != nil {
		return nil, nil, err
	}
	return []byte(str), sm, nil
}

// GetCache returns the function exclusion cache (if present)
// Returns nil if preprocessor was created without a cache
func (p *Preprocessor) GetCache() *FunctionExclusionCache {
	return p.cache
}

// HasCache returns true if this preprocessor has a package-wide cache
func (p *Preprocessor) HasCache() bool {
	return p.cache != nil
}

// injectImportsWithPosition adds needed imports using TEXT-BASED manipulation
// CRITICAL: This preserves ALL comments including free-floating markers (// dingo:E:N)
// AST-based approaches strip comments, breaking source map generation
// Returns: modified source, import block start line (1-based), import block end line (1-based), and error
func injectImportsWithPosition(source []byte, needed []string) ([]byte, int, int, error) {
	if len(needed) == 0 {
		return source, 0, 0, nil
	}

	// Deduplicate and sort needed imports
	importMap := make(map[string]bool)
	for _, pkg := range needed {
		importMap[pkg] = true
	}

	code := string(source)
	lines := strings.Split(code, "\n")

	// Find package declaration
	packageLineIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			packageLineIdx = i
			break
		}
	}
	if packageLineIdx == -1 {
		return nil, 0, 0, fmt.Errorf("no package declaration found")
	}

	// Find existing import block (if any)
	importBlockStart, importBlockEnd := findImportBlock(lines)

	// Parse existing imports to avoid duplicates
	if importBlockStart >= 0 {
		for i := importBlockStart + 1; i < importBlockEnd; i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || trimmed == ")" {
				continue
			}
			// Extract import path: "path" or _ "path" or alias "path"
			importPath := extractImportPath(trimmed)
			if importPath != "" {
				delete(importMap, importPath)
			}
		}
	}

	// If no new imports needed, return original
	if len(importMap) == 0 {
		return source, 0, 0, nil
	}

	// Convert to sorted slice
	finalImports := make([]string, 0, len(importMap))
	for pkg := range importMap {
		finalImports = append(finalImports, pkg)
	}
	sort.Strings(finalImports)

	var result string
	var insertLine, endLine int

	if importBlockStart >= 0 {
		// Merge with existing import block
		result, insertLine, endLine = mergeImports(lines, finalImports, importBlockStart, importBlockEnd)
	} else {
		// Create new import block after package
		result, insertLine, endLine = insertImports(lines, finalImports, packageLineIdx)
	}

	return []byte(result), insertLine, endLine, nil
}

// findImportBlock locates an existing import block in the source
// Returns start and end line indices (0-based), or -1, -1 if not found
func findImportBlock(lines []string) (start, end int) {
	inImport := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import (") {
			start = i
			inImport = true
		} else if inImport && trimmed == ")" {
			end = i
			return start, end
		} else if strings.HasPrefix(trimmed, "import ") && !strings.Contains(trimmed, "(") {
			// Single import: import "path"
			return i, i
		}
	}
	return -1, -1
}

// extractImportPath extracts the import path from an import line
// Handles: "path", _ "path", alias "path"
func extractImportPath(line string) string {
	// Remove leading/trailing whitespace
	line = strings.TrimSpace(line)

	// Find the quoted string
	startQuote := strings.Index(line, `"`)
	if startQuote == -1 {
		return ""
	}
	endQuote := strings.LastIndex(line, `"`)
	if endQuote <= startQuote {
		return ""
	}

	return line[startQuote+1 : endQuote]
}

// mergeImports adds new imports to an existing import block
func mergeImports(lines []string, newImports []string, start, end int) (string, int, int) {
	// Special case: single-line import (start == end)
	// Convert: import "path" → import ( "path" "new" )
	if start == end {
		// Extract existing import path from the single-line import
		existingImportLine := strings.TrimSpace(lines[start])
		existingPath := extractImportPath(existingImportLine)

		// Build new import block with existing + new imports (deduplicate)
		importSet := make(map[string]bool)
		if existingPath != "" {
			importSet[existingPath] = true
		}
		for _, imp := range newImports {
			importSet[imp] = true
		}

		// Convert to sorted slice
		allImports := make([]string, 0, len(importSet))
		for imp := range importSet {
			allImports = append(allImports, imp)
		}
		sort.Strings(allImports)

		// Build result
		result := make([]string, 0, len(lines)+len(allImports)+1)

		// Copy lines before single import
		result = append(result, lines[:start]...)

		// Replace single import with import block
		result = append(result, "import (")
		for _, imp := range allImports {
			result = append(result, fmt.Sprintf("\t\"%s\"", imp))
		}
		result = append(result, ")")

		// Copy lines after single import
		result = append(result, lines[start+1:]...)

		// Calculate positions (1-based for return)
		insertLine := start + 2                    // Line after "import ("
		endLine := start + 1 + len(allImports) + 1 // Closing )

		return strings.Join(result, "\n"), insertLine, endLine
	}

	// Normal case: multi-line import block
	// Build new lines with merged imports
	result := make([]string, 0, len(lines)+len(newImports))

	// Copy lines before import block
	result = append(result, lines[:end]...)

	// Add new imports before closing )
	for _, imp := range newImports {
		result = append(result, fmt.Sprintf("\t\"%s\"", imp))
	}

	// Copy lines from closing ) onward
	result = append(result, lines[end:]...)

	// Calculate positions (1-based for return)
	insertLine := start + 2              // Line after "import ("
	endLine := end + len(newImports) + 1 // Adjusted closing )

	return strings.Join(result, "\n"), insertLine, endLine
}

// insertImports creates a new import block after the package declaration
func insertImports(lines []string, imports []string, packageLine int) (string, int, int) {
	// Build import block
	importBlock := []string{
		"",
		"import (",
	}
	for _, imp := range imports {
		importBlock = append(importBlock, fmt.Sprintf("\t\"%s\"", imp))
	}
	importBlock = append(importBlock, ")")

	// Insert after package line
	result := make([]string, 0, len(lines)+len(importBlock))
	result = append(result, lines[:packageLine+1]...)
	result = append(result, importBlock...)
	result = append(result, lines[packageLine+1:]...)

	// Calculate positions (1-based for return)
	insertLine := packageLine + 2                 // Line after package declaration
	endLine := packageLine + 1 + len(importBlock) // Last line of import block

	return strings.Join(result, "\n"), insertLine, endLine
}

// adjustMappingsForImports shifts mapping line numbers to account for added imports
// CRITICAL FIX: Shifts mappings for lines AFTER the import insertion point
func adjustMappingsForImports(sourceMap *SourceMap, numImportLines int, importInsertionLine int) {
	for i := range sourceMap.Mappings {
		// CRITICAL FIX: Only shift mappings for lines AFTER import insertion
		//
		// importInsertionLine is the line number (1-based) where imports are inserted
		// (typically line 2 or 3, right after the package declaration).
		//
		// We use > (not >=) to exclude the insertion line itself. Mappings AT the
		// insertion line are for package-level declarations BEFORE the imports, and
		// should NOT be shifted.
		//
		// Example:
		//   Line 1: package main
		//   Line 2: [IMPORTS INSERTED HERE] ← importInsertionLine = 2
		//   Line 3: func foo() { ... } (shifts to line 7 if 4-line import block added)
		//
		// Mappings with GeneratedLine=1 or 2 stay as-is.
		// Mappings with GeneratedLine=3+ are shifted by numImportLines.
		if sourceMap.Mappings[i].GeneratedLine > importInsertionLine {
			sourceMap.Mappings[i].GeneratedLine += numImportLines
		}
	}
}

// convertMetadataToMappings converts TransformMetadata to legacy Mapping structs
// for backward compatibility with tests and tools that expect source mappings.
//
// Strategy:
// - Scan generated source to find marker line numbers (e.g., "// dingo:e:0")
// - For each metadata entry, create a mapping from original to generated line
// - Metadata contains: OriginalLine, GeneratedMarker
// - We need to find: GeneratedLine (by scanning for marker)
func convertMetadataToMappings(metadata []TransformMetadata, generatedSource []byte) []Mapping {
	if len(metadata) == 0 {
		return nil
	}

	// Build marker-to-line-number map by scanning generated source
	markerLines := make(map[string]int)
	lines := strings.Split(string(generatedSource), "\n")
	for lineNum, line := range lines {
		// Look for markers like "// dingo:e:0", "// dingo:s:1", etc.
		if idx := strings.Index(line, "// dingo:"); idx != -1 {
			marker := strings.TrimSpace(line[idx:])
			markerLines[marker] = lineNum + 1 // Convert to 1-based line number
		}
	}

	// Convert metadata to mappings
	var mappings []Mapping
	for _, meta := range metadata {
		if meta.GeneratedMarker == "" {
			// No marker, can't create mapping
			continue
		}

		generatedLine, found := markerLines[meta.GeneratedMarker]
		if !found {
			// Marker not found in source, skip
			continue
		}

		// Create mapping from original line to generated line
		mapping := Mapping{
			OriginalLine:    meta.OriginalLine,
			OriginalColumn:  meta.OriginalColumn,
			GeneratedLine:   generatedLine,
			GeneratedColumn: 1, // Column info not preserved in markers, use 1
			Length:          meta.OriginalLength,
			Name:            meta.Type, // Use Type as Name for debugging
		}
		mappings = append(mappings, mapping)
	}

	// Sort mappings by generated line for consistency
	sort.Slice(mappings, func(i, j int) bool {
		if mappings[i].GeneratedLine == mappings[j].GeneratedLine {
			return mappings[i].OriginalLine < mappings[j].OriginalLine
		}
		return mappings[i].GeneratedLine < mappings[j].GeneratedLine
	})

	return mappings
}
