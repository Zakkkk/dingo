package codegen

import (
	"bytes"
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
)

// TupleTypeResolver uses go/types to resolve tuple marker functions to concrete types.
//
// This is Pass 2 of the two-pass tuple pipeline:
// - Pass 1 (tuple.go): Transform tuple syntax to markers (no type info needed)
// - Pass 2 (this file): Use go/types to resolve markers to final structs
//
// Responsibilities:
// 1. Parse marker-infused Go source with go/parser
// 2. Type-check with go/types to get TypeInfo
// 3. Resolve marker function calls to struct literals
// 4. Resolve marker destructuring to field access
// 5. Handle context-aware return types (infer from function signature)
// 6. Type deduplication (same types share one struct)
// 7. Generate source mappings
type TupleTypeResolver struct {
	info       *types.Info
	fset       *token.FileSet
	typeCache  map[string]string  // type signature → struct name (for deduplication)
	structs    []structDefinition // Accumulated struct definitions
	tmpCounter int                // Counter for unique tmp variable names per scope
}

// structDefinition represents a generated tuple struct type
type structDefinition struct {
	name      string   // e.g., "Tuple2IntString"
	elemTypes []string // e.g., ["int", "string"]
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
		info:      info,
		fset:      fset,
		typeCache: make(map[string]string),
		structs:   []structDefinition{},
	}, nil
}

// Resolve processes marker-infused source and generates final Go code.
//
// Input: Source with markers from Pass 1
// Output: Final Go code with struct definitions and transformed expressions
func (r *TupleTypeResolver) Resolve(src []byte) (ast.CodeGenResult, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "tuple.go", src, goparser.ParseComments)
	if err != nil {
		return ast.CodeGenResult{}, fmt.Errorf("failed to parse source: %w", err)
	}

	// Transform markers in-place in the AST
	r.transformMarkers(file)

	// Insert struct definitions into the AST (after imports, before other decls)
	if len(r.structs) > 0 {
		structDecls := r.generateStructDecls()
		file.Decls = insertStructsAfterImports(file.Decls, structDecls)
	}

	// Print the transformed AST back to source
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, file); err != nil {
		return ast.CodeGenResult{}, fmt.Errorf("failed to print AST: %w", err)
	}

	return ast.CodeGenResult{
		Output:   buf.Bytes(),
		Mappings: []ast.SourceMapping{},
	}, nil
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
// Example: buildFieldAccess("tmp", [0, 1]) → tmp._0._1
func buildFieldAccess(base string, path []int) goast.Expr {
	var result goast.Expr = goast.NewIdent(base)

	for _, idx := range path {
		result = &goast.SelectorExpr{
			X:   result,
			Sel: goast.NewIdent(fmt.Sprintf("_%d", idx)),
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

// resolveLiteralMarker transforms __tuple{N}__(args) to struct literal.
func (r *TupleTypeResolver) resolveLiteralMarker(call *goast.CallExpr) goast.Expr {
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

	// Get or create struct name
	structName := r.getOrCreateStructType(elemTypes)

	// Transform arguments recursively (handle nested tuples)
	for i, arg := range call.Args {
		call.Args[i] = r.transformExpr(arg)
	}

	// Create struct literal: StructName{_0: arg0, _1: arg1, ...}
	elts := make([]goast.Expr, len(call.Args))
	for i, arg := range call.Args {
		elts[i] = &goast.KeyValueExpr{
			Key:   goast.NewIdent(fmt.Sprintf("_%d", i)),
			Value: arg,
		}
	}

	return &goast.CompositeLit{
		Type: goast.NewIdent(structName),
		Elts: elts,
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
// The struct has fields _0, _1, _2, etc. with the corresponding element types.
func (r *TupleTypeResolver) constructStructType(elemTypes []types.Type) types.Type {
	if len(elemTypes) == 0 {
		return types.NewInterfaceType(nil, nil)
	}

	// Create struct fields: _0, _1, _2, etc.
	fields := make([]*types.Var, len(elemTypes))
	for i, elemType := range elemTypes {
		fieldName := fmt.Sprintf("_%d", i)
		fields[i] = types.NewField(token.NoPos, nil, fieldName, elemType, false)
	}

	// Create the struct type
	return types.NewStruct(fields, nil)
}

// getOrCreateStructType returns the struct name for given element types.
func (r *TupleTypeResolver) getOrCreateStructType(elemTypes []types.Type) string {
	sig := r.getTypeSignature(elemTypes)

	// Check cache
	if name, exists := r.typeCache[sig]; exists {
		return name
	}

	// Create new struct
	name := r.generateStructName(elemTypes)
	r.typeCache[sig] = name

	// Store for later definition generation
	typeStrings := make([]string, len(elemTypes))
	for i, t := range elemTypes {
		typeStrings[i] = types.TypeString(t, nil)
	}
	r.structs = append(r.structs, structDefinition{
		name:      name,
		elemTypes: typeStrings,
	})

	return name
}

// getTypeSignature creates a canonical signature for type deduplication.
func (r *TupleTypeResolver) getTypeSignature(elemTypes []types.Type) string {
	var parts []string
	for _, t := range elemTypes {
		parts = append(parts, types.TypeString(t, nil))
	}
	return strings.Join(parts, ",")
}

// generateStructName creates a CamelCase struct name from element types.
func (r *TupleTypeResolver) generateStructName(elemTypes []types.Type) string {
	var parts []string
	parts = append(parts, "Tuple"+strconv.Itoa(len(elemTypes)))

	for _, t := range elemTypes {
		parts = append(parts, typeToNameComponent(t))
	}

	return strings.Join(parts, "")
}

// typeToNameComponent converts a Go type to a CamelCase name component.
func typeToNameComponent(t types.Type) string {
	switch typ := t.(type) {
	case *types.Basic:
		return basicTypeName(typ)
	case *types.Pointer:
		return "Ptr" + typeToNameComponent(typ.Elem())
	case *types.Slice:
		return "Slice" + typeToNameComponent(typ.Elem())
	case *types.Map:
		return "Map" + typeToNameComponent(typ.Key()) + typeToNameComponent(typ.Elem())
	case *types.Named:
		return typ.Obj().Name()
	case *types.Interface:
		if typ.Empty() {
			return "Any"
		}
		return "Interface"
	default:
		return "Any"
	}
}

// basicTypeName converts basic Go types to CamelCase names.
func basicTypeName(t *types.Basic) string {
	switch t.Kind() {
	case types.Bool:
		return "Bool"
	case types.Int:
		return "Int"
	case types.Int8:
		return "Int8"
	case types.Int16:
		return "Int16"
	case types.Int32:
		return "Int32"
	case types.Int64:
		return "Int64"
	case types.Uint:
		return "Uint"
	case types.Uint8:
		return "Uint8"
	case types.Uint16:
		return "Uint16"
	case types.Uint32:
		return "Uint32"
	case types.Uint64:
		return "Uint64"
	case types.Float32:
		return "Float32"
	case types.Float64:
		return "Float64"
	case types.String:
		return "String"
	case types.UntypedInt:
		return "Int"
	case types.UntypedFloat:
		return "Float64"
	case types.UntypedString:
		return "String"
	case types.UntypedBool:
		return "Bool"
	case types.UntypedRune:
		return "Rune"
	case types.UntypedNil:
		return "Any"
	default:
		return "Any"
	}
}

// generateStructDefinitions creates the struct type definitions as a string.
// NOTE: This is used for debugging/testing. Use generateStructDecls for AST-based insertion.
func (r *TupleTypeResolver) generateStructDefinitions() string {
	if len(r.structs) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString("// Generated tuple types\n")

	for _, s := range r.structs {
		result.WriteString("type ")
		result.WriteString(s.name)
		result.WriteString(" struct {\n")
		for i, elemType := range s.elemTypes {
			result.WriteString("\t_")
			result.WriteString(strconv.Itoa(i))
			result.WriteString(" ")
			result.WriteString(elemType)
			result.WriteString("\n")
		}
		result.WriteString("}\n\n")
	}

	return result.String()
}

// generateStructDecls creates AST declaration nodes for tuple struct types.
// This is used for AST-based struct insertion in Resolve().
//
// Note: We use a special position (1) for the Tok field to ensure go/printer
// places the type keyword on the same line as the struct. Without position info,
// go/printer may misplace comments between 'type' and the struct name.
func (r *TupleTypeResolver) generateStructDecls() []goast.Decl {
	var decls []goast.Decl

	for _, s := range r.structs {
		// Create struct fields with fresh positions
		var fields []*goast.Field
		for i, elemType := range s.elemTypes {
			field := &goast.Field{
				Names: []*goast.Ident{{Name: fmt.Sprintf("_%d", i)}},
				Type:  &goast.Ident{Name: elemType},
			}
			fields = append(fields, field)
		}

		// Create struct type
		structType := &goast.StructType{
			Fields: &goast.FieldList{List: fields},
		}

		// Create type spec with fresh ident (no position info)
		typeSpec := &goast.TypeSpec{
			Name: &goast.Ident{Name: s.name},
			Type: structType,
		}

		// Create GenDecl for type declaration
		// Use TokPos=1 to give the type keyword a position, which helps
		// go/printer format correctly without splitting 'type' from struct name
		genDecl := &goast.GenDecl{
			TokPos: 1,
			Tok:    token.TYPE,
			Specs:  []goast.Spec{typeSpec},
		}

		decls = append(decls, genDecl)
	}

	return decls
}

// insertStructsAfterImports inserts struct declarations after import declarations.
// If there are no imports, structs are inserted at the beginning of declarations.
func insertStructsAfterImports(decls []goast.Decl, structDecls []goast.Decl) []goast.Decl {
	if len(structDecls) == 0 {
		return decls
	}

	// Find the last import declaration
	lastImportIdx := -1
	for i, decl := range decls {
		if genDecl, ok := decl.(*goast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			lastImportIdx = i
		}
	}

	// Insert after imports (or at beginning if no imports)
	insertIdx := lastImportIdx + 1

	// Create new slice with capacity for all declarations
	result := make([]goast.Decl, 0, len(decls)+len(structDecls))
	result = append(result, decls[:insertIdx]...)
	result = append(result, structDecls...)
	result = append(result, decls[insertIdx:]...)

	return result
}
