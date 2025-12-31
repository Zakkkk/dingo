package semantic

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// debug is a helper for debug logging to stderr (disabled by default)
func debug(format string, args ...interface{}) {
	// Uncomment the line below to enable debug logging:
	// fmt.Fprintf(os.Stderr, "[DEBUG builder] "+format+"\n", args...)
	_ = format
	_ = args
}

// Builder constructs a semantic Map from transpilation results
type Builder struct {
	goAST          *ast.File
	goFset         *token.FileSet
	typesInfo      *types.Info
	lineMappings   []sourcemap.LineMapping
	columnMappings []sourcemap.ColumnMapping // For accurate column translation
	dingoSource    []byte
	dingoFset      *token.FileSet
	dingoFile      string
}

// NewBuilder creates a Builder
// IMPORTANT: Follows CLAUDE.md rules - uses token.FileSet for all position tracking
func NewBuilder(
	goAST *ast.File,
	goFset *token.FileSet,
	typesInfo *types.Info,
	lineMappings []sourcemap.LineMapping,
	columnMappings []sourcemap.ColumnMapping,
	dingoSource []byte,
	dingoFset *token.FileSet,
	dingoFile string,
) *Builder {
	return &Builder{
		goAST:          goAST,
		goFset:         goFset,
		typesInfo:      typesInfo,
		lineMappings:   lineMappings,
		columnMappings: columnMappings,
		dingoSource:    dingoSource,
		dingoFset:      dingoFset,
		dingoFile:      dingoFile,
	}
}

// Build creates a semantic Map by:
// 1. Scanning ALL identifiers from Dingo source (primary entity collection)
// 2. Adding specialized entities (operators, lambda params, enum variants, option constructors)
// 3. Building final sorted map with deduplication (specialized wins over generic)
//
// CLAUDE.md COMPLIANT: Uses Dingo tokenizer for position tracking, looks up types by NAME.
// NO Go->Dingo position mapping. All positions come from Dingo source directly.
func (b *Builder) Build() (*Map, error) {
	var entities []SemanticEntity

	// 1. Primary: Scan ALL identifiers from Dingo source
	//    Uses name-based type lookup (first match wins)
	entities = append(entities, b.detectAllIdentifiers()...)

	// 2. Specialized detections (add context-specific entities)
	entities = append(entities, b.detectOperators()...)              // KindOperator, etc.
	entities = append(entities, b.detectLambdaParams()...)           // KindLambda, hardcoded error type
	entities = append(entities, b.detectEnumVariantOccurrences()...) // KindType for enum variants
	entities = append(entities, b.detectOptionConstructors()...)     // Custom Description for None/Some
	entities = append(entities, b.detectGuardKeywords()...)          // Guard statement keyword

	// 3. Build and return the map (deduplication: specialized wins)
	return NewMap(entities), nil
}

// detectAllIdentifiers scans Dingo source for ALL identifiers and looks up their types.
// This is the primary entity collection method, replacing the Go AST walk.
//
// CLAUDE.md COMPLIANT: Uses Dingo tokenizer, looks up types by NAME from Go type checker.
func (b *Builder) detectAllIdentifiers() []SemanticEntity {
	if b.dingoSource == nil || len(b.dingoSource) == 0 {
		return nil
	}

	// Create a FileSet for tokenization if not provided
	fset := b.dingoFset
	if fset == nil {
		fset = token.NewFileSet()
	}

	tok := tokenizer.NewWithFileSet(b.dingoSource, fset, b.dingoFile)
	tokens, err := tok.Tokenize()
	if err != nil {
		return nil
	}

	// Build lookup map: name -> Object (first match wins)
	objByName := b.buildObjectLookup()

	var entities []SemanticEntity
	for _, t := range tokens {
		if t.Kind != tokenizer.IDENT {
			continue
		}

		// Skip keywords and built-ins
		if isKeywordOrBuiltin(t.Lit) {
			continue
		}

		// Skip None/Some - handled by detectOptionConstructors with custom Description
		if t.Lit == "None" || t.Lit == "Some" {
			continue
		}

		// Look up type info by name (first match wins)
		obj, ok := objByName[t.Lit]
		if !ok {
			continue
		}

		entities = append(entities, SemanticEntity{
			Line:   t.Line,
			Col:    t.Column,
			EndCol: t.Column + len(t.Lit),
			Kind:   determineKind(obj),
			Object: obj,
			Type:   obj.Type(),
		})
	}

	return entities
}

// buildObjectLookup creates a name->Object map from Go type info.
// First match wins - this is acceptable for hover info.
func (b *Builder) buildObjectLookup() map[string]types.Object {
	objByName := make(map[string]types.Object)
	if b.typesInfo == nil {
		return objByName
	}

	for ident, obj := range b.typesInfo.Uses {
		if ident != nil && obj != nil {
			if _, exists := objByName[ident.Name]; !exists {
				objByName[ident.Name] = obj
			}
		}
	}
	for ident, obj := range b.typesInfo.Defs {
		if ident != nil && obj != nil {
			if _, exists := objByName[ident.Name]; !exists {
				objByName[ident.Name] = obj
			}
		}
	}
	return objByName
}

// determineKind determines the SemanticKind from a types.Object
func determineKind(obj types.Object) SemanticKind {
	switch obj.(type) {
	case *types.TypeName:
		return KindType
	case *types.Func:
		return KindIdent // Functions are identifiers until called
	case *types.Var:
		return KindIdent
	case *types.Const:
		return KindIdent
	default:
		return KindIdent
	}
}

// detectOperators finds Dingo operators and creates entities for them
func (b *Builder) detectOperators() []SemanticEntity {
	// Create a FileSet for operator detection if not provided
	// This is safe because we only need it for position tracking during tokenization
	fset := b.dingoFset
	if fset == nil {
		fset = token.NewFileSet()
	}

	// Use the operators.go DetectOperators function
	operators := DetectOperators(b.dingoSource, fset, b.dingoFile)

	var entities []SemanticEntity
	for _, op := range operators {
		// Try to infer the type for this operator by looking nearby in the semantic map
		// For now, we create the entity without a type - this will be enriched later
		// when we have more context about the expression it applies to

		var context *DingoContext
		switch op.Kind {
		case ContextErrorProp:
			// Error propagation: always provide context (hover should work even without type info)
			context = &DingoContext{
				Kind:        ContextErrorProp,
				Description: "Error propagation operator",
			}
			// Try to extract Result type for richer hover info
			if op.ExprType != nil {
				unwrapped, original, ok := b.extractResultType(op.ExprType)
				if ok {
					context.OriginalType = original
					context.UnwrappedType = unwrapped
				}
			}

		case ContextSafeNav:
			context = &DingoContext{
				Kind:        ContextSafeNav,
				Description: "Safe navigation operator",
			}

		case ContextNullCoal:
			context = &DingoContext{
				Kind:        ContextNullCoal,
				Description: "Null coalescing operator",
			}

		case ContextTernary:
			context = &DingoContext{
				Kind:        ContextTernary,
				Description: "Ternary conditional operator",
			}
		}

		entities = append(entities, SemanticEntity{
			Line:    op.Line,
			Col:     op.Col,
			EndCol:  op.EndCol,
			Kind:    KindOperator,
			Type:    op.ExprType,
			Context: context,
		})
	}

	return entities
}

// detectLambdaParams finds lambda parameters in Dingo source and creates entities
// Note: Lambda parameter types are not available as they get renamed during transformation.
func (b *Builder) detectLambdaParams() []SemanticEntity {
	fset := b.dingoFset
	if fset == nil {
		fset = token.NewFileSet()
	}

	params := DetectLambdaParams(b.dingoSource, fset, b.dingoFile)

	var entities []SemanticEntity
	for _, p := range params {
		entities = append(entities, SemanticEntity{
			Line:   p.Line,
			Col:    p.Col,
			EndCol: p.EndCol,
			Kind:   KindLambda,
			Context: &DingoContext{
				Kind: ContextLambda,
				Name: p.Name,
			},
		})
	}

	return entities
}

// detectEnumVariantOccurrences creates entities for ALL occurrences of each enum variant
// in the Dingo source. This is needed because:
// 1. Go collapses multiple Dingo match arms (with/without guards) into single case
// 2. Go position mapping is imprecise due to enum expansion
// 3. "Find closest" heuristic fails when computed line is closer to wrong occurrence
//
// This method ensures every variant usage in Dingo source has hover support.
// CLAUDE.md COMPLIANT: Uses Dingo tokenizer for position tracking.
func (b *Builder) detectEnumVariantOccurrences() []SemanticEntity {
	if b.typesInfo == nil || b.dingoSource == nil || len(b.dingoSource) == 0 {
		return nil
	}

	fset := b.dingoFset
	if fset == nil {
		fset = token.NewFileSet()
	}

	// Step 1: Find all enum variant types from Go AST
	// Collect TypeName -> variantInfo mapping
	type variantInfo struct {
		typeName   *types.TypeName
		dingoName  string // Short name like "UserCreated"
		entityType types.Type
	}
	variants := make(map[string]variantInfo) // dingoName -> info

	for ident, obj := range b.typesInfo.Defs {
		if ident == nil || obj == nil {
			continue
		}
		typeName, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		dingoName := extractDingoVariantName(typeName.Type())
		if dingoName == "" {
			continue
		}
		// Store variant info
		variants[dingoName] = variantInfo{
			typeName:   typeName,
			dingoName:  dingoName,
			entityType: typeName.Type(),
		}
	}

	if len(variants) == 0 {
		return nil
	}

	// Step 2: Scan Dingo source for all occurrences of variant names using Dingo tokenizer
	tok := tokenizer.NewWithFileSet(b.dingoSource, fset, b.dingoFile)
	tokens, err := tok.Tokenize()
	if err != nil {
		return nil
	}

	var entities []SemanticEntity
	seenPositions := make(map[string]bool)

	for _, t := range tokens {
		if t.Kind != tokenizer.IDENT {
			continue
		}

		info, ok := variants[t.Lit]
		if !ok {
			continue
		}

		key := fmt.Sprintf("%d:%d", t.Line, t.Column)
		if seenPositions[key] {
			continue
		}
		seenPositions[key] = true

		// Create entity for this occurrence
		entities = append(entities, SemanticEntity{
			Line:    t.Line,
			Col:     t.Column,
			EndCol:  t.Column + len(t.Lit),
			Kind:    KindType,
			Object:  info.typeName,
			Type:    info.entityType,
			Context: nil, // Hover formatting will detect this is a variant
		})
	}

	return entities
}

// detectOptionConstructors finds bare None/Some identifiers in Dingo source
// and creates semantic entities for them. These become dgo.None[T]()/dgo.Some(value)
// in Go but struct literal areas may not have line mappings.
func (b *Builder) detectOptionConstructors() []SemanticEntity {
	if b.dingoSource == nil || len(b.dingoSource) == 0 {
		return nil
	}

	fset := b.dingoFset
	if fset == nil {
		fset = token.NewFileSet()
	}

	// Scan Dingo source for None and Some identifiers
	optionIdents := DetectOptionConstructorIdentifiers(b.dingoSource, fset, b.dingoFile)

	var entities []SemanticEntity
	seenPositions := make(map[string]bool)

	for _, oi := range optionIdents {
		key := fmt.Sprintf("%d:%d", oi.Line, oi.Col)
		if seenPositions[key] {
			continue
		}
		seenPositions[key] = true

		// Create entity with context indicating it's an Option constructor
		entity := SemanticEntity{
			Line:   oi.Line,
			Col:    oi.Col,
			EndCol: oi.EndCol,
			Kind:   KindIdent,
			Context: &DingoContext{
				Kind:        ContextNone, // Generic context
				Description: oi.Description,
			},
		}

		entities = append(entities, entity)
	}

	return entities
}

// detectGuardKeywords finds guard keywords and creates semantic entities for them
func (b *Builder) detectGuardKeywords() []SemanticEntity {
	if b.dingoSource == nil || len(b.dingoSource) == 0 {
		return nil
	}

	fset := b.dingoFset
	if fset == nil {
		fset = token.NewFileSet()
	}

	guards := DetectGuardKeywords(b.dingoSource, fset, b.dingoFile)

	var entities []SemanticEntity
	for _, g := range guards {
		entities = append(entities, SemanticEntity{
			Line:   g.Line,
			Col:    g.Col,
			EndCol: g.EndCol,
			Kind:   KindOperator, // Use operator kind for special syntax
			Context: &DingoContext{
				Kind:        ContextGuard,
				Description: "Guard statement",
			},
		})
	}

	return entities
}

// extractDingoVariantName extracts the Dingo variant name from a Go enum variant type.
// Go enum variants are named "<Enum><Variant>" but Dingo uses just "<Variant>".
// Returns empty string if not an enum variant.
func extractDingoVariantName(t types.Type) string {
	named, ok := t.(*types.Named)
	if !ok {
		return ""
	}

	// Must be a struct
	if _, ok := named.Underlying().(*types.Struct); !ok {
		return ""
	}

	// Check for marker method is<Enum>()
	var enumName string
	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if strings.HasPrefix(method.Name(), "is") && len(method.Name()) > 2 {
			sig := method.Type().(*types.Signature)
			if sig.Params().Len() == 0 && sig.Results().Len() == 0 {
				enumName = method.Name()[2:] // Strip "is" prefix
				break
			}
		}
	}

	if enumName == "" {
		return ""
	}

	// Strip enum prefix to get variant name
	typeName := named.Obj().Name()
	if !strings.HasPrefix(typeName, enumName) {
		return ""
	}
	variantName := typeName[len(enumName):]
	if variantName == "" {
		return ""
	}

	return variantName
}

// extractResultType extracts T from Result[T, E] or Option[T]
// Returns (innerType, originalType, true) if successful
func (b *Builder) extractResultType(t types.Type) (inner, original types.Type, ok bool) {
	// Handle named types
	named, ok := t.(*types.Named)
	if !ok {
		return nil, nil, false
	}

	// Get the type name
	typeName := named.Obj().Name()

	// Check for Result[T, E] or Option[T]
	if typeName != "Result" && typeName != "Option" {
		return nil, nil, false
	}

	// Check package path (should be dgo package)
	pkg := named.Obj().Pkg()
	if pkg == nil {
		return nil, nil, false
	}

	// Package name should contain "dgo" (could be "github.com/user/dgo")
	if !strings.Contains(pkg.Path(), "dgo") && pkg.Name() != "dgo" {
		return nil, nil, false
	}

	// Extract type arguments
	// Result[T, E] has 2 type args, Option[T] has 1
	if named.TypeArgs() == nil || named.TypeArgs().Len() == 0 {
		return nil, nil, false
	}

	// Get the first type argument (T)
	innerType := named.TypeArgs().At(0)

	return innerType, t, true
}

// inferContext determines Dingo-specific context for an entity
// NOTE: This no longer adds error propagation context just because a type is Option/Result.
// Error propagation context is only added for:
// 1. The ? operator (detected separately in detectOperators)
// 2. Variables that are the RESULT of unwrapping via ? (TODO: track during transformation)
//
// A field of type Option[string] should show Option[string], not "string (from Option)"
func (b *Builder) inferContext(node interface{}, typ types.Type) *DingoContext {
	// Currently no automatic context inference
	// Error propagation context is handled by detectOperators for ? operators
	// Regular Option/Result fields should just show their actual type
	return nil
}
