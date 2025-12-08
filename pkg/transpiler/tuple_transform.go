package transpiler

import (
	"bytes"
	"fmt"
	goast "go/ast"
	gotoken "go/token"
	"sort"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/codegen"
	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// transformTuplePass1 transforms all tuple syntax to marker functions.
// This is Pass 1 of the two-pass tuple pipeline:
//   - Literals: (a, b) → __tuple2__(a, b)
//   - Destructuring: let (x, y) = point → let __tupleDest2__("x", "y", point)
//   - Type aliases: type Point = (int, int) → type Point = __tupleType2__(int, int)
//
// The markers are valid Go code that will be type-checked by go/types.
// Pass 2 will then resolve the markers to actual struct types.
func transformTuplePass1(src []byte) ([]byte, []ast.SourceMapping, error) {
	// Find all tuple locations
	locations, err := ast.FindTuples(src)
	if err != nil {
		return nil, nil, fmt.Errorf("find tuples: %w", err)
	}

	if len(locations) == 0 {
		return src, nil, nil
	}

	// Filter out nested tuples - only keep top-level tuples
	// Nested tuples are handled by generateLiteralMarker's transformNestedTuples
	locations = filterTopLevelTuples(locations)

	if len(locations) == 0 {
		return src, nil, nil
	}

	// Sort by position descending (transform from end to avoid offset shifts)
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].Start > locations[j].Start
	})

	result := src
	var mappings []ast.SourceMapping
	gen := codegen.NewTupleCodeGen()

	// Transform each tuple from end to beginning
	for _, loc := range locations {
		// Generate marker code for this tuple
		marker, err := gen.GenerateFromLocation(loc, result)
		if err != nil {
			return nil, nil, fmt.Errorf("generate marker at byte %d: %w", loc.Start, err)
		}

		// Handle different tuple kinds for replacement
		var replaceStart, replaceEnd int
		switch loc.Kind {
		case ast.TupleKindDestructure:
			// For destructuring, we need to replace "let (pattern) = expr"
			// The marker is: __tupleDest2__("x", "y", expr)
			// But this isn't valid Go - we need: _ = __tupleDest2__("x", "y", expr)
			// This makes it a valid statement that Pass 2 can replace

			// Use pre-computed positions from TupleLocation (no string scanning per CLAUDE.md)
			replaceStart = loc.KeywordStart // Start of "let"
			replaceEnd = loc.ExprEnd        // End of full statement
			// Make it a valid Go statement: _ = marker
			marker = append([]byte("_ = "), marker...)

		case ast.TupleKindTypeAlias:
			// For type alias: type Point = (int, int)
			// We can't use markers for type declarations - they must be valid Go types
			// Solution: Generate the struct type directly in Pass 1
			// type Point = struct { _0 int; _1 int }

			// Use pre-computed positions from TupleLocation (no string scanning per CLAUDE.md)
			replaceStart = loc.KeywordStart // Start of "type"
			replaceEnd = loc.End            // End of tuple type

			// Generate struct type using pre-parsed type content from finder
			// The prefix "type Name = " is preserved from source
			typePrefix := result[loc.KeywordStart:loc.Start]
			var structDef []byte
			structDef = append(structDef, typePrefix...)
			structDef = append(structDef, []byte("struct { ")...)

			// Use pre-parsed TypeContent if available, otherwise use interface{}
			if len(loc.TypeContent) > 0 {
				for i, typeStr := range loc.TypeContent {
					if i > 0 {
						structDef = append(structDef, []byte("; ")...)
					}
					structDef = append(structDef, []byte(fmt.Sprintf("_%d %s", i, typeStr))...)
				}
			} else {
				for i := 0; i < loc.Elements; i++ {
					if i > 0 {
						structDef = append(structDef, []byte("; ")...)
					}
					structDef = append(structDef, []byte(fmt.Sprintf("_%d interface{}", i))...)
				}
			}
			structDef = append(structDef, []byte(" }")...)
			marker = structDef

		case ast.TupleKindLiteral:
			// For literal, replace just the tuple expression
			replaceStart = loc.Start
			replaceEnd = loc.End

		case ast.TupleKindFuncReturn:
			// For function return types, we can't use markers - they must be valid Go types
			// Generate an anonymous struct type directly
			// Example: func foo() (float64, float64) → func foo() struct { _0 float64; _1 float64 }
			replaceStart = loc.Start
			replaceEnd = loc.End

			// ElementsInfo MUST be populated by the finder (no string manipulation per CLAUDE.md)
			if len(loc.ElementsInfo) == 0 {
				return nil, nil, fmt.Errorf("ElementsInfo not populated for func return tuple at byte %d - use FindTuples() to get properly populated TupleLocation", loc.Start)
			}

			// Generate anonymous struct using pre-parsed element types
			var structDef []byte
			structDef = append(structDef, []byte("struct { ")...)
			for i, elem := range loc.ElementsInfo {
				if i > 0 {
					structDef = append(structDef, []byte("; ")...)
				}
				structDef = append(structDef, []byte(fmt.Sprintf("_%d %s", i, elem.Name))...)
			}
			structDef = append(structDef, []byte(" }")...)
			marker = structDef
		}

		// Splice marker into result
		newResult := make([]byte, 0, len(result)-(replaceEnd-replaceStart)+len(marker))
		newResult = append(newResult, result[:replaceStart]...)
		newResult = append(newResult, marker...)
		newResult = append(newResult, result[replaceEnd:]...)
		result = newResult

		// Add source mapping
		mappings = append(mappings, ast.SourceMapping{
			DingoStart: replaceStart,
			DingoEnd:   replaceEnd,
			GoStart:    replaceStart,
			GoEnd:      replaceStart + len(marker),
			Kind:       "tuple_" + loc.Kind.String(),
		})
	}

	return result, mappings, nil
}

// transformTuplePass2 resolves tuple markers to final struct types using go/types.
// This is Pass 2 of the two-pass tuple pipeline.
//
// Markers from Pass 1:
//   - __tuple2__(a, b) → Tuple2IntString{_0: a, _1: b}
//   - __tupleDest2__("x", "y", point) → tmp := point; x := tmp._0; y := tmp._1
//   - __tupleType2__(int, string) → Tuple2IntString
//
// Returns the transformed source with markers replaced by actual Go code.
func transformTuplePass2(fset *gotoken.FileSet, file *goast.File, checker *typechecker.Checker, src []byte) ([]byte, error) {
	// Quick check: do we have any tuple markers?
	// If not, skip the transformation entirely to avoid overhead
	if !bytes.Contains(src, []byte("__tuple")) {
		return src, nil
	}

	// Create a type resolver from the marker-infused source
	resolver, err := codegen.NewTupleTypeResolver(src)
	if err != nil {
		return nil, fmt.Errorf("create tuple resolver: %w", err)
	}

	// Resolve markers to final Go code
	result, err := resolver.Resolve(src)
	if err != nil {
		return nil, fmt.Errorf("resolve tuple markers: %w", err)
	}

	return result.Output, nil
}

// filterTopLevelTuples removes nested tuple literals from the list.
// Only top-level tuples should be processed - nested ones will be handled
// recursively by generateLiteralMarker's transformNestedTuples function.
//
// A tuple is "nested" if its byte range [Start, End) is fully contained
// within another tuple's byte range.
//
// Example: For input "return ((0.0, 0.0), (0.0, 0.0))", FindTuples returns:
//   - Outer tuple at 7-31
//   - Inner1 at 8-18
//   - Inner2 at 20-30
//
// After filtering, only the outer tuple (7-31) is returned.
// Inner tuples will be transformed when processing the outer tuple's content.
func filterTopLevelTuples(locations []ast.TupleLocation) []ast.TupleLocation {
	if len(locations) <= 1 {
		return locations
	}

	// Mark which tuples are nested inside others
	isNested := make([]bool, len(locations))

	for i := range locations {
		for j := range locations {
			if i == j {
				continue
			}
			// Check if locations[i] is fully contained within locations[j]
			// A tuple is nested if: j.Start <= i.Start && i.End <= j.End
			if locations[j].Start <= locations[i].Start && locations[i].End <= locations[j].End {
				// locations[i] is nested inside locations[j]
				isNested[i] = true
				break
			}
		}
	}

	// Return only non-nested tuples
	var result []ast.TupleLocation
	for i, loc := range locations {
		if !isNested[i] {
			result = append(result, loc)
		}
	}

	return result
}
