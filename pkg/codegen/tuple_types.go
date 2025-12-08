package codegen

import (
	"bytes"
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// TupleTypeResolver uses go/types to resolve tuple marker functions to generic types.
//
// This is Pass 2 of the two-pass tuple pipeline:
// - Pass 1 (tuple.go): Transform tuple syntax to markers (no type info needed)
// - Pass 2 (this file): Use go/types to resolve markers to generic tuple types
//
// Uses generic tuple types from runtime/tuples package:
//   - tuples.Tuple2[A, B], tuples.Tuple3[A, B, C], etc.
//   - Field names: First, Second, Third, Fourth, Fifth, Sixth, Seventh, Eighth, Ninth, Tenth
//
// Responsibilities:
// 1. Parse marker-infused Go source with go/parser
// 2. Type-check with go/types to get TypeInfo
// 3. Resolve marker function calls to generic struct literals
// 4. Resolve marker destructuring to field access
// 5. Track type aliases to use alias names in generated code
// 6. Generate source mappings
type TupleTypeResolver struct {
	info        *types.Info
	fset        *token.FileSet
	typeAliases map[string]string // alias name → generic type signature (e.g., "Point2D" → "tuples.Tuple2[float64, float64]")
	tmpCounter  int               // Counter for unique tmp variable names per scope
	needsImport bool              // Whether runtime/tuples import is needed
}

// TupleFieldNames maps tuple indices to human-readable field names
var TupleFieldNames = []string{
	"First", "Second", "Third", "Fourth", "Fifth",
	"Sixth", "Seventh", "Eighth", "Ninth", "Tenth",
}

// NewTupleTypeResolver creates a resolver from type-checked Go source.
//
// The src must be the output of Pass 1 (with marker functions).
// This function will parse it with go/parser and type-check with go/types.
func NewTupleTypeResolver(src []byte) (*TupleTypeResolver, error) {
	fset := token.NewFileSet()

	// Parse the marker-infused source
	file, err := goparser.ParseFile(fset, "tuple.go", src, goparser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse marker source: %w", err)
	}

	// Type-check the parsed file
	// We ignore errors from undefined marker functions - we only need type info for arguments
	conf := types.Config{
		Importer: nil,
		Error: func(err error) {
			// Ignore errors - marker functions won't be defined
		},
	}

	info := &types.Info{
		Types: make(map[goast.Expr]types.TypeAndValue),
		Defs:  make(map[*goast.Ident]types.Object),
		Uses:  make(map[*goast.Ident]types.Object),
	}

	pkg, _ := conf.Check("tuple", fset, []*goast.File{file}, info)

	if pkg == nil {
		info = &types.Info{
			Types: make(map[goast.Expr]types.TypeAndValue),
		}
	}

	return &TupleTypeResolver{
		info:        info,
		fset:        fset,
		typeAliases: make(map[string]string),
	}, nil
}

// Resolve processes marker-infused source and generates final Go code.
//
// Input: Source with markers from Pass 1
// Output: Final Go code with generic tuple types from runtime/tuples package
//
// Uses byte-level replacement to preserve comments exactly as they appear
// in the source. The AST is only used for type information, not for output.
func (r *TupleTypeResolver) Resolve(src []byte) (ast.CodeGenResult, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "tuple.go", src, 0) // No comments needed for AST
	if err != nil {
		return ast.CodeGenResult{}, fmt.Errorf("failed to parse source: %w", err)
	}

	// Collect all marker replacements (position -> replacement string)
	replacements := r.collectMarkerReplacements(fset, file, src)

	// Sort replacements by position descending (transform from end to avoid offset shifts)
	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start > replacements[j].start
	})

	// Apply byte-level replacements
	result := src
	for _, repl := range replacements {
		newResult := make([]byte, 0, len(result)-(repl.end-repl.start)+len(repl.replacement))
		newResult = append(newResult, result[:repl.start]...)
		newResult = append(newResult, repl.replacement...)
		newResult = append(newResult, result[repl.end:]...)
		result = newResult
	}

	// Add tuples import if needed (byte-level)
	if r.needsImport {
		result = addTuplesImportBytes(result)
	}

	return ast.CodeGenResult{
		Output:   result,
		Mappings: []ast.SourceMapping{},
	}, nil
}

// markerReplacement represents a marker to be replaced with its position and replacement text.
type markerReplacement struct {
	start       int
	end         int
	replacement []byte
}

// collectMarkerReplacements walks the AST and collects all marker replacements.
func (r *TupleTypeResolver) collectMarkerReplacements(fset *token.FileSet, file *goast.File, src []byte) []markerReplacement {
	var replacements []markerReplacement

	// Walk all declarations looking for markers
	for _, decl := range file.Decls {
		r.collectDeclReplacements(fset, decl, src, &replacements)
	}

	return replacements
}

// collectDeclReplacements collects marker replacements within a declaration.
func (r *TupleTypeResolver) collectDeclReplacements(fset *token.FileSet, decl goast.Decl, src []byte, replacements *[]markerReplacement) {
	switch d := decl.(type) {
	case *goast.FuncDecl:
		if d.Body != nil {
			r.collectStmtListReplacements(fset, d.Body.List, src, replacements)
		}
	case *goast.GenDecl:
		for _, spec := range d.Specs {
			if vs, ok := spec.(*goast.ValueSpec); ok {
				for _, val := range vs.Values {
					r.collectExprReplacements(fset, val, src, replacements)
				}
			}
		}
	}
}

// collectStmtListReplacements collects marker replacements from a statement list.
func (r *TupleTypeResolver) collectStmtListReplacements(fset *token.FileSet, stmts []goast.Stmt, src []byte, replacements *[]markerReplacement) {
	for _, stmt := range stmts {
		r.collectStmtReplacements(fset, stmt, src, replacements)
	}
}

// collectStmtReplacements collects marker replacements from a single statement.
func (r *TupleTypeResolver) collectStmtReplacements(fset *token.FileSet, stmt goast.Stmt, src []byte, replacements *[]markerReplacement) {
	switch s := stmt.(type) {
	case *goast.ExprStmt:
		r.collectExprReplacements(fset, s.X, src, replacements)
	case *goast.AssignStmt:
		// Check for destructure marker: _ = __tupleDest*__(...)
		if len(s.Lhs) == 1 && len(s.Rhs) == 1 {
			if lhsIdent, ok := s.Lhs[0].(*goast.Ident); ok && lhsIdent.Name == "_" {
				if call, ok := s.Rhs[0].(*goast.CallExpr); ok {
					if fnIdent, ok := call.Fun.(*goast.Ident); ok && strings.HasPrefix(fnIdent.Name, "__tupleDest") {
						// This is a destructure marker - generate replacement for whole statement
						repl := r.generateDestructureReplacement(fset, s, call, src)
						if repl != nil {
							*replacements = append(*replacements, *repl)
							return // Don't recurse into this statement
						}
					}
				}
			}
		}
		// Regular assignment - check RHS for literal markers
		for _, rhs := range s.Rhs {
			r.collectExprReplacements(fset, rhs, src, replacements)
		}
	case *goast.ReturnStmt:
		for _, result := range s.Results {
			r.collectExprReplacements(fset, result, src, replacements)
		}
	case *goast.IfStmt:
		if s.Init != nil {
			r.collectStmtReplacements(fset, s.Init, src, replacements)
		}
		r.collectExprReplacements(fset, s.Cond, src, replacements)
		if s.Body != nil {
			r.collectStmtListReplacements(fset, s.Body.List, src, replacements)
		}
		if s.Else != nil {
			r.collectStmtReplacements(fset, s.Else, src, replacements)
		}
	case *goast.BlockStmt:
		r.collectStmtListReplacements(fset, s.List, src, replacements)
	case *goast.ForStmt:
		if s.Init != nil {
			r.collectStmtReplacements(fset, s.Init, src, replacements)
		}
		if s.Cond != nil {
			r.collectExprReplacements(fset, s.Cond, src, replacements)
		}
		if s.Post != nil {
			r.collectStmtReplacements(fset, s.Post, src, replacements)
		}
		if s.Body != nil {
			r.collectStmtListReplacements(fset, s.Body.List, src, replacements)
		}
	case *goast.RangeStmt:
		r.collectExprReplacements(fset, s.X, src, replacements)
		if s.Body != nil {
			r.collectStmtListReplacements(fset, s.Body.List, src, replacements)
		}
	case *goast.SwitchStmt:
		if s.Init != nil {
			r.collectStmtReplacements(fset, s.Init, src, replacements)
		}
		if s.Tag != nil {
			r.collectExprReplacements(fset, s.Tag, src, replacements)
		}
		if s.Body != nil {
			r.collectStmtListReplacements(fset, s.Body.List, src, replacements)
		}
	case *goast.CaseClause:
		for _, expr := range s.List {
			r.collectExprReplacements(fset, expr, src, replacements)
		}
		r.collectStmtListReplacements(fset, s.Body, src, replacements)
	}
}

// collectExprReplacements collects marker replacements from an expression.
func (r *TupleTypeResolver) collectExprReplacements(fset *token.FileSet, expr goast.Expr, src []byte, replacements *[]markerReplacement) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *goast.CallExpr:
		// Check if this is a tuple literal marker
		if ident, ok := e.Fun.(*goast.Ident); ok {
			if strings.HasPrefix(ident.Name, "__tuple") && !strings.Contains(ident.Name, "Dest") && !strings.Contains(ident.Name, "Type") {
				// This is a literal marker - generate replacement
				repl := r.generateLiteralReplacement(fset, e, src)
				if repl != nil {
					*replacements = append(*replacements, *repl)
					return // Don't recurse - nested markers are handled in replacement generation
				}
			}
		}
		// Not a marker - recurse into arguments
		for _, arg := range e.Args {
			r.collectExprReplacements(fset, arg, src, replacements)
		}
		r.collectExprReplacements(fset, e.Fun, src, replacements)

	case *goast.BinaryExpr:
		r.collectExprReplacements(fset, e.X, src, replacements)
		r.collectExprReplacements(fset, e.Y, src, replacements)

	case *goast.UnaryExpr:
		r.collectExprReplacements(fset, e.X, src, replacements)

	case *goast.ParenExpr:
		r.collectExprReplacements(fset, e.X, src, replacements)

	case *goast.IndexExpr:
		r.collectExprReplacements(fset, e.X, src, replacements)
		r.collectExprReplacements(fset, e.Index, src, replacements)

	case *goast.SelectorExpr:
		r.collectExprReplacements(fset, e.X, src, replacements)

	case *goast.SliceExpr:
		r.collectExprReplacements(fset, e.X, src, replacements)
		if e.Low != nil {
			r.collectExprReplacements(fset, e.Low, src, replacements)
		}
		if e.High != nil {
			r.collectExprReplacements(fset, e.High, src, replacements)
		}
		if e.Max != nil {
			r.collectExprReplacements(fset, e.Max, src, replacements)
		}

	case *goast.CompositeLit:
		for _, elt := range e.Elts {
			r.collectExprReplacements(fset, elt, src, replacements)
		}

	case *goast.KeyValueExpr:
		r.collectExprReplacements(fset, e.Key, src, replacements)
		r.collectExprReplacements(fset, e.Value, src, replacements)

	case *goast.FuncLit:
		if e.Body != nil {
			r.collectStmtListReplacements(fset, e.Body.List, src, replacements)
		}
	}
}

// generateLiteralReplacement generates the replacement for a tuple literal marker.
func (r *TupleTypeResolver) generateLiteralReplacement(fset *token.FileSet, call *goast.CallExpr, src []byte) *markerReplacement {
	r.needsImport = true

	// Get byte positions
	start := fset.Position(call.Pos()).Offset
	end := fset.Position(call.End()).Offset

	// Get types of arguments
	var elemTypes []types.Type
	for _, arg := range call.Args {
		var typ types.Type
		typeAndVal, ok := r.info.Types[arg]
		if ok && typeAndVal.Type != nil {
			typ = typeAndVal.Type
		} else {
			typ = r.inferLiteralType(arg)
		}
		elemTypes = append(elemTypes, typ)
	}

	// Build the replacement string
	arity := len(call.Args)
	var buf strings.Builder

	// Generic type: tuples.Tuple{N}[type1, type2, ...]
	buf.WriteString(fmt.Sprintf("tuples.Tuple%d[", arity))
	for i, t := range elemTypes {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(typeToGoString(t))
	}
	buf.WriteString("]{")

	// Fields: First: val1, Second: val2, ...
	for i, arg := range call.Args {
		if i > 0 {
			buf.WriteString(", ")
		}
		fieldName := TupleFieldNames[i]
		buf.WriteString(fieldName)
		buf.WriteString(": ")

		// Get the argument source, handling nested markers recursively
		argStart := fset.Position(arg.Pos()).Offset
		argEnd := fset.Position(arg.End()).Offset
		argSrc := string(src[argStart:argEnd])

		// Check if arg is a nested marker
		if nestedCall, ok := arg.(*goast.CallExpr); ok {
			if nestedIdent, ok := nestedCall.Fun.(*goast.Ident); ok {
				if strings.HasPrefix(nestedIdent.Name, "__tuple") && !strings.Contains(nestedIdent.Name, "Dest") {
					// Recursively generate nested replacement
					nestedRepl := r.generateLiteralReplacement(fset, nestedCall, src)
					if nestedRepl != nil {
						argSrc = string(nestedRepl.replacement)
					}
				}
			}
		}

		buf.WriteString(argSrc)
	}
	buf.WriteByte('}')

	return &markerReplacement{
		start:       start,
		end:         end,
		replacement: []byte(buf.String()),
	}
}

// generateDestructureReplacement generates the replacement for a destructure marker statement.
func (r *TupleTypeResolver) generateDestructureReplacement(fset *token.FileSet, stmt *goast.AssignStmt, call *goast.CallExpr, src []byte) *markerReplacement {
	if len(call.Args) < 2 {
		return nil
	}

	// Get byte positions for the whole statement: _ = __tupleDest*__(...)
	start := fset.Position(stmt.Pos()).Offset
	end := fset.Position(stmt.End()).Offset

	// Last arg is the expression being destructured
	valueExpr := call.Args[len(call.Args)-1]
	valueStart := fset.Position(valueExpr.Pos()).Offset
	valueEnd := fset.Position(valueExpr.End()).Offset
	valueSrc := string(src[valueStart:valueEnd])

	// Parse variable names and paths from string literals
	type varBinding struct {
		name string
		path []int
	}
	var bindings []varBinding

	for i := 0; i < len(call.Args)-1; i++ {
		lit, ok := call.Args[i].(*goast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		encoded := lit.Value[1 : len(lit.Value)-1]
		name, path := parseEncodedBinding(encoded)
		if name != "_" {
			bindings = append(bindings, varBinding{name: name, path: path})
		}
	}

	if len(bindings) == 0 {
		return nil
	}

	// Generate replacement statements
	var buf strings.Builder

	// Generate unique tmp variable name
	r.tmpCounter++
	tmpName := "tmp"
	if r.tmpCounter > 1 {
		tmpName = fmt.Sprintf("tmp%d", r.tmpCounter-1)
	}

	// Statement 1: tmp := expr
	buf.WriteString(tmpName)
	buf.WriteString(" := ")
	buf.WriteString(valueSrc)
	buf.WriteByte('\n')

	// Statements 2+: varName := tmp.First.Second...
	for _, b := range bindings {
		buf.WriteString(b.name)
		buf.WriteString(" := ")
		buf.WriteString(tmpName)
		for _, idx := range b.path {
			buf.WriteByte('.')
			buf.WriteString(TupleFieldNames[idx])
		}
		buf.WriteByte('\n')
	}

	// Remove trailing newline
	result := strings.TrimRight(buf.String(), "\n")

	return &markerReplacement{
		start:       start,
		end:         end,
		replacement: []byte(result),
	}
}

// addTuplesImportBytes adds the tuples import using byte-level manipulation.
func addTuplesImportBytes(src []byte) []byte {
	const tuplesImport = `"github.com/MadAppGang/dingo/runtime/tuples"`

	// Check if already imported
	if bytes.Contains(src, []byte(tuplesImport)) {
		return src
	}

	// Find the import block
	importIdx := bytes.Index(src, []byte("import ("))
	if importIdx != -1 {
		// Find the newline after "import ("
		newlineIdx := bytes.IndexByte(src[importIdx:], '\n')
		if newlineIdx != -1 {
			insertPos := importIdx + newlineIdx + 1
			// Insert the new import
			newImport := []byte("\t" + tuplesImport + "\n")
			result := make([]byte, 0, len(src)+len(newImport))
			result = append(result, src[:insertPos]...)
			result = append(result, newImport...)
			result = append(result, src[insertPos:]...)
			return result
		}
	}

	// Fallback: find single import statement
	singleImportIdx := bytes.Index(src, []byte("import "))
	if singleImportIdx != -1 && importIdx == -1 {
		// Find the end of the import line
		lineEnd := bytes.IndexByte(src[singleImportIdx:], '\n')
		if lineEnd != -1 {
			// Convert single import to import block
			// Find the import path
			importStart := singleImportIdx + len("import ")
			importPath := src[importStart : singleImportIdx+lineEnd]

			// Create import block
			var newImport strings.Builder
			newImport.WriteString("import (\n\t")
			newImport.Write(bytes.TrimSpace(importPath))
			newImport.WriteString("\n\t")
			newImport.WriteString(tuplesImport)
			newImport.WriteString("\n)")

			result := make([]byte, 0, len(src)+len(tuplesImport)+20)
			result = append(result, src[:singleImportIdx]...)
			result = append(result, []byte(newImport.String())...)
			result = append(result, src[singleImportIdx+lineEnd:]...)
			return result
		}
	}

	return src
}

// addTuplesImport adds the runtime/tuples import to the file if not already present.
func addTuplesImport(file *goast.File) {
	const tuplesPath = `"github.com/MadAppGang/dingo/runtime/tuples"`

	// Check if already imported
	for _, imp := range file.Imports {
		if imp.Path.Value == tuplesPath {
			return // Already imported
		}
	}

	// Create import spec
	importSpec := &goast.ImportSpec{
		Path: &goast.BasicLit{
			Kind:  token.STRING,
			Value: tuplesPath,
		},
	}

	// Find or create import declaration
	var importDecl *goast.GenDecl
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			importDecl = genDecl
			break
		}
	}

	if importDecl != nil {
		// Add to existing import declaration
		importDecl.Specs = append(importDecl.Specs, importSpec)
	} else {
		// Create new import declaration
		newImport := &goast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []goast.Spec{importSpec},
		}
		// Insert at beginning of declarations
		file.Decls = append([]goast.Decl{newImport}, file.Decls...)
	}

	// Update file's Imports slice
	file.Imports = append(file.Imports, importSpec)
}

// transformMarkers walks the AST and transforms marker function calls.
func (r *TupleTypeResolver) transformMarkers(file *goast.File) {
	for _, decl := range file.Decls {
		r.transformDeclMarkers(decl)
	}
}

// transformDeclMarkers transforms markers within a declaration.
func (r *TupleTypeResolver) transformDeclMarkers(decl goast.Decl) {
	switch d := decl.(type) {
	case *goast.FuncDecl:
		if d.Body != nil {
			d.Body.List = r.transformStmtList(d.Body.List)
		}
	case *goast.GenDecl:
		for _, spec := range d.Specs {
			if vs, ok := spec.(*goast.ValueSpec); ok {
				for i, val := range vs.Values {
					vs.Values[i] = r.transformExpr(val)
				}
			}
		}
	}
}

// transformStmtList transforms statements containing markers.
// For destructure markers, it replaces the single statement with multiple statements.
func (r *TupleTypeResolver) transformStmtList(stmts []goast.Stmt) []goast.Stmt {
	var result []goast.Stmt
	for _, stmt := range stmts {
		// Check if this is a destructure marker statement: _ = __tupleDest*__(...)
		if expanded := r.tryExpandDestructure(stmt); expanded != nil {
			result = append(result, expanded...)
		} else {
			result = append(result, r.transformStmt(stmt))
		}
	}
	return result
}

// tryExpandDestructure checks if stmt is `_ = __tupleDest*__(...)` and expands it.
// Returns nil if not a destructure marker, otherwise returns replacement statements.
func (r *TupleTypeResolver) tryExpandDestructure(stmt goast.Stmt) []goast.Stmt {
	assign, ok := stmt.(*goast.AssignStmt)
	if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil
	}

	// Check LHS is blank identifier
	lhsIdent, ok := assign.Lhs[0].(*goast.Ident)
	if !ok || lhsIdent.Name != "_" {
		return nil
	}

	// Check RHS is __tupleDest*__(...) call
	call, ok := assign.Rhs[0].(*goast.CallExpr)
	if !ok {
		return nil
	}
	fnIdent, ok := call.Fun.(*goast.Ident)
	if !ok || !strings.HasPrefix(fnIdent.Name, "__tupleDest") {
		return nil
	}

	// This is a destructure marker - expand it
	return r.expandDestructureMarker(call)
}

// expandDestructureMarker transforms __tupleDest{N}__("name:path", ..., expr) to statements.
//
// For flat patterns like __tupleDest2__("x:0", "y:1", point):
//   tmp := point
//   x := tmp._0
//   y := tmp._1
//
// For nested patterns like __tupleDest4__("minX:0.0", "minY:0.1", "maxX:1.0", "maxY:1.1", bbox):
//   tmp := bbox
//   minX := tmp._0._0
//   minY := tmp._0._1
//   maxX := tmp._1._0
//   maxY := tmp._1._1
//
// Note: Uses unique tmp variable names (tmp, tmp1, tmp2, ...) to avoid
// "no new variables on left side of :=" errors when multiple destructurings
// appear in the same scope.
func (r *TupleTypeResolver) expandDestructureMarker(call *goast.CallExpr) []goast.Stmt {
	if len(call.Args) < 2 {
		return nil
	}

	// Last arg is the expression being destructured
	valueExpr := call.Args[len(call.Args)-1]

	// Parse variable names and paths from string literals
	type varBinding struct {
		name string
		path []int
	}
	var bindings []varBinding

	for i := 0; i < len(call.Args)-1; i++ {
		lit, ok := call.Args[i].(*goast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		// Remove quotes
		encoded := lit.Value[1 : len(lit.Value)-1]

		// Parse "name:path" format
		name, path := parseEncodedBinding(encoded)
		if name != "_" { // Skip wildcards
			bindings = append(bindings, varBinding{name: name, path: path})
		}
	}

	if len(bindings) == 0 {
		return nil
	}

	var stmts []goast.Stmt

	// Generate unique tmp variable name following CLAUDE.md naming convention:
	// First tmp is "tmp", subsequent are "tmp1", "tmp2", etc.
	r.tmpCounter++
	tmpName := "tmp"
	if r.tmpCounter > 1 {
		tmpName = fmt.Sprintf("tmp%d", r.tmpCounter-1)
	}

	// Statement 1: tmp := expr (or tmp1 := expr, etc.)
	tmpAssign := &goast.AssignStmt{
		Lhs: []goast.Expr{goast.NewIdent(tmpName)},
		Tok: token.DEFINE,
		Rhs: []goast.Expr{valueExpr},
	}
	stmts = append(stmts, tmpAssign)

	// Statements 2+: varName := tmp._0._1... (using the unique tmp name)
	for _, b := range bindings {
		fieldAccess := buildFieldAccess(tmpName, b.path)
		varAssign := &goast.AssignStmt{
			Lhs: []goast.Expr{goast.NewIdent(b.name)},
			Tok: token.DEFINE,
			Rhs: []goast.Expr{fieldAccess},
		}
		stmts = append(stmts, varAssign)
	}

	return stmts
}

// parseEncodedBinding parses "name:path" format.
// Example: "minX:0.0" → ("minX", [0, 0])
// Example: "x:0" → ("x", [0])
// Example: "x" → ("x", []) (legacy format without path)
func parseEncodedBinding(encoded string) (string, []int) {
	colonIdx := strings.Index(encoded, ":")
	if colonIdx == -1 {
		// Legacy format: just name, no path
		return encoded, nil
	}

	name := encoded[:colonIdx]
	pathStr := encoded[colonIdx+1:]

	var path []int
	if pathStr != "" {
		parts := strings.Split(pathStr, ".")
		for _, p := range parts {
			idx, err := strconv.Atoi(p)
			if err == nil {
				path = append(path, idx)
			}
		}
	}

	return name, path
}

// buildFieldAccess creates a selector expression for nested field access.
// Example: buildFieldAccess("tmp", [0, 1]) → tmp.First.Second
func buildFieldAccess(base string, path []int) goast.Expr {
	var result goast.Expr = goast.NewIdent(base)

	for _, idx := range path {
		fieldName := TupleFieldNames[idx]
		result = &goast.SelectorExpr{
			X:   result,
			Sel: goast.NewIdent(fieldName),
		}
	}

	return result
}

// transformStmt transforms a single statement.
func (r *TupleTypeResolver) transformStmt(stmt goast.Stmt) goast.Stmt {
	switch s := stmt.(type) {
	case *goast.ExprStmt:
		s.X = r.transformExpr(s.X)
	case *goast.AssignStmt:
		for i, rhs := range s.Rhs {
			s.Rhs[i] = r.transformExpr(rhs)
		}
	case *goast.ReturnStmt:
		for i, result := range s.Results {
			s.Results[i] = r.transformExpr(result)
		}
	case *goast.IfStmt:
		if s.Init != nil {
			s.Init = r.transformStmt(s.Init)
		}
		s.Cond = r.transformExpr(s.Cond)
		if s.Body != nil {
			s.Body.List = r.transformStmtList(s.Body.List)
		}
		if s.Else != nil {
			s.Else = r.transformStmt(s.Else)
		}
	case *goast.BlockStmt:
		s.List = r.transformStmtList(s.List)
	case *goast.ForStmt:
		if s.Init != nil {
			s.Init = r.transformStmt(s.Init)
		}
		if s.Cond != nil {
			s.Cond = r.transformExpr(s.Cond)
		}
		if s.Post != nil {
			s.Post = r.transformStmt(s.Post)
		}
		if s.Body != nil {
			s.Body.List = r.transformStmtList(s.Body.List)
		}
	case *goast.RangeStmt:
		s.X = r.transformExpr(s.X)
		if s.Body != nil {
			s.Body.List = r.transformStmtList(s.Body.List)
		}
	case *goast.SwitchStmt:
		if s.Init != nil {
			s.Init = r.transformStmt(s.Init)
		}
		if s.Tag != nil {
			s.Tag = r.transformExpr(s.Tag)
		}
		if s.Body != nil {
			s.Body.List = r.transformStmtList(s.Body.List)
		}
	case *goast.CaseClause:
		for i, expr := range s.List {
			s.List[i] = r.transformExpr(expr)
		}
		s.Body = r.transformStmtList(s.Body)
	}
	return stmt
}

// transformExpr transforms an expression, replacing markers with final code.
func (r *TupleTypeResolver) transformExpr(expr goast.Expr) goast.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *goast.CallExpr:
		// Check if this is a tuple marker
		if ident, ok := e.Fun.(*goast.Ident); ok {
			if strings.HasPrefix(ident.Name, "__tuple") && !strings.Contains(ident.Name, "Dest") && !strings.Contains(ident.Name, "Type") {
				return r.resolveLiteralMarker(e)
			}
		}
		// Transform arguments recursively
		for i, arg := range e.Args {
			e.Args[i] = r.transformExpr(arg)
		}
		e.Fun = r.transformExpr(e.Fun)

	case *goast.BinaryExpr:
		e.X = r.transformExpr(e.X)
		e.Y = r.transformExpr(e.Y)

	case *goast.UnaryExpr:
		e.X = r.transformExpr(e.X)

	case *goast.ParenExpr:
		e.X = r.transformExpr(e.X)

	case *goast.IndexExpr:
		e.X = r.transformExpr(e.X)
		e.Index = r.transformExpr(e.Index)

	case *goast.SelectorExpr:
		e.X = r.transformExpr(e.X)

	case *goast.SliceExpr:
		e.X = r.transformExpr(e.X)
		if e.Low != nil {
			e.Low = r.transformExpr(e.Low)
		}
		if e.High != nil {
			e.High = r.transformExpr(e.High)
		}
		if e.Max != nil {
			e.Max = r.transformExpr(e.Max)
		}

	case *goast.CompositeLit:
		for i, elt := range e.Elts {
			e.Elts[i] = r.transformExpr(elt)
		}

	case *goast.KeyValueExpr:
		e.Key = r.transformExpr(e.Key)
		e.Value = r.transformExpr(e.Value)

	case *goast.FuncLit:
		// Function literals (lambdas) contain bodies that may have tuple markers
		if e.Body != nil {
			e.Body.List = r.transformStmtList(e.Body.List)
		}
	}

	return expr
}

// resolveLiteralMarker transforms __tuple{N}__(args) to generic struct literal.
//
// Example:
//   __tuple2__(3.0, 4.0) → tuples.Tuple2[float64, float64]{First: 3.0, Second: 4.0}
func (r *TupleTypeResolver) resolveLiteralMarker(call *goast.CallExpr) goast.Expr {
	r.needsImport = true

	// Get types of arguments
	var elemTypes []types.Type
	for _, arg := range call.Args {
		var typ types.Type

		// Try go/types first
		typeAndVal, ok := r.info.Types[arg]
		if ok && typeAndVal.Type != nil {
			typ = typeAndVal.Type
		} else {
			// Fallback: infer from AST literal type
			typ = r.inferLiteralType(arg)
		}

		elemTypes = append(elemTypes, typ)
	}

	// Transform arguments recursively (handle nested tuples)
	for i, arg := range call.Args {
		call.Args[i] = r.transformExpr(arg)
	}

	// Build the generic type: tuples.Tuple{N}[type1, type2, ...]
	arity := len(call.Args)
	genericTypeName := fmt.Sprintf("tuples.Tuple%d", arity)

	// Build type parameters as index expression
	var typeExpr goast.Expr = goast.NewIdent(genericTypeName)

	// Add type parameters if we have type info
	if len(elemTypes) > 0 {
		typeParams := make([]goast.Expr, len(elemTypes))
		for i, t := range elemTypes {
			typeParams[i] = goast.NewIdent(typeToGoString(t))
		}
		// Create indexed expression for generics: tuples.Tuple2[float64, float64]
		if arity == 1 {
			typeExpr = &goast.IndexExpr{
				X:     typeExpr,
				Index: typeParams[0],
			}
		} else {
			typeExpr = &goast.IndexListExpr{
				X:       typeExpr,
				Indices: typeParams,
			}
		}
	}

	// Create struct literal with named fields: {First: val1, Second: val2}
	elts := make([]goast.Expr, len(call.Args))
	for i, arg := range call.Args {
		fieldName := TupleFieldNames[i]
		elts[i] = &goast.KeyValueExpr{
			Key:   goast.NewIdent(fieldName),
			Value: arg,
		}
	}

	return &goast.CompositeLit{
		Type: typeExpr,
		Elts: elts,
	}
}

// typeToGoString converts a types.Type to a Go type string suitable for generics.
func typeToGoString(t types.Type) string {
	switch typ := t.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.UntypedInt:
			return "int"
		case types.UntypedFloat:
			return "float64"
		case types.UntypedString:
			return "string"
		case types.UntypedBool:
			return "bool"
		case types.UntypedRune:
			return "rune"
		default:
			return typ.Name()
		}
	case *types.Struct:
		// For nested tuples, we need to generate the full generic type
		// e.g., tuples.Tuple2[float64, float64]
		numFields := typ.NumFields()
		if numFields >= 2 && numFields <= 10 {
			var fieldTypes []string
			for i := 0; i < numFields; i++ {
				fieldTypes = append(fieldTypes, typeToGoString(typ.Field(i).Type()))
			}
			return fmt.Sprintf("tuples.Tuple%d[%s]", numFields, strings.Join(fieldTypes, ", "))
		}
		return "any"
	default:
		return types.TypeString(t, nil)
	}
}

// inferLiteralType infers the type of an expression from its AST node.
// Uses go/types Uses map to look up identifier types, falling back to
// AST literal analysis for basic literals.
//
// This function uses heuristics to infer types when go/types can't help:
// - Binary expressions: if either operand has a known type, use that
// - Parenthesized expressions: unwrap and infer recursively
// - Identifiers: check Uses/Defs maps, then fall back to context hints
func (r *TupleTypeResolver) inferLiteralType(expr goast.Expr) types.Type {
	// First, try to get type from go/types info
	if r.info != nil && r.info.Types != nil {
		if tv, ok := r.info.Types[expr]; ok && tv.Type != nil {
			return tv.Type
		}
	}

	switch e := expr.(type) {
	case *goast.BasicLit:
		switch e.Kind {
		case token.INT:
			return types.Typ[types.Int]
		case token.FLOAT:
			return types.Typ[types.Float64]
		case token.STRING:
			return types.Typ[types.String]
		case token.CHAR:
			return types.Typ[types.Rune]
		}
	case *goast.Ident:
		if e.Name == "true" || e.Name == "false" {
			return types.Typ[types.Bool]
		}
		if e.Name == "nil" {
			return types.Typ[types.UntypedNil]
		}
		// Try to look up the identifier's type from go/types Uses map
		if r.info != nil && r.info.Uses != nil {
			if obj := r.info.Uses[e]; obj != nil {
				return obj.Type()
			}
		}
		// Also check Defs map for locally defined variables
		if r.info != nil && r.info.Defs != nil {
			if obj := r.info.Defs[e]; obj != nil {
				return obj.Type()
			}
		}
		// For identifiers without type info, use float64 as default for tuple contexts
		// This is a heuristic: most tuple values in geometry are float64
		// TODO: Implement context-aware type inference from function signatures
		return types.Typ[types.Float64]
	case *goast.BinaryExpr:
		// For binary expressions, try to infer from operands
		// Check both sides - if one is a literal with known type, use that
		leftType := r.inferLiteralType(e.X)
		if !isInterface(leftType) {
			return leftType
		}
		rightType := r.inferLiteralType(e.Y)
		if !isInterface(rightType) {
			return rightType
		}
		// Default to float64 for binary operations (common case)
		return types.Typ[types.Float64]
	case *goast.ParenExpr:
		// Unwrap parenthesized expression
		return r.inferLiteralType(e.X)
	case *goast.UnaryExpr:
		// For unary expressions (like -2.0), infer from operand
		return r.inferLiteralType(e.X)
	case *goast.CallExpr:
		// For nested tuple markers, recurse and construct the struct type
		if ident, ok := e.Fun.(*goast.Ident); ok {
			if strings.HasPrefix(ident.Name, "__tuple") && !strings.Contains(ident.Name, "Dest") && !strings.Contains(ident.Name, "Type") {
				// This is a nested tuple literal - infer element types and construct struct type
				var nestedTypes []types.Type
				for _, arg := range e.Args {
					nestedTypes = append(nestedTypes, r.inferLiteralType(arg))
				}
				// Construct the struct type for this nested tuple
				// This enables proper type matching for returns like BoundingBox
				return r.constructStructType(nestedTypes)
			}
		}
	}
	// Default to float64 for unknown types in tuple contexts
	// This is better than interface{} for most numeric tuple operations
	return types.Typ[types.Float64]
}

// isInterface returns true if the type is an empty interface
func isInterface(t types.Type) bool {
	if iface, ok := t.(*types.Interface); ok {
		return iface.Empty()
	}
	return false
}

// constructStructType creates a *types.Struct from element types.
// This is used by inferLiteralType to properly type nested tuples.
// The struct has fields First, Second, Third, etc. with the corresponding element types.
func (r *TupleTypeResolver) constructStructType(elemTypes []types.Type) types.Type {
	if len(elemTypes) == 0 {
		return types.NewInterfaceType(nil, nil)
	}

	// Create struct fields: First, Second, Third, etc.
	fields := make([]*types.Var, len(elemTypes))
	for i, elemType := range elemTypes {
		fieldName := TupleFieldNames[i]
		fields[i] = types.NewField(token.NoPos, nil, fieldName, elemType, false)
	}

	// Create the struct type
	return types.NewStruct(fields, nil)
}

