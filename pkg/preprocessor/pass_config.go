package preprocessor

// BodyProcessor processes expression content within lambda bodies
// This interface enables decoupled injection - Lambda doesn't import concrete processors
//
// Expression processors (Pass 2) implement this to be injected into lambda bodies
// without creating circular dependencies or tight coupling.
type BodyProcessor interface {
	// ProcessBody transforms expression content (e.g., inside lambda bodies)
	// Returns transformed content and error if transformation failed
	ProcessBody(body []byte) ([]byte, error)

	// Name returns the processor name for debugging and logging
	Name() string
}

// Compile-time assertions: Verify all expression processors implement BodyProcessor
// If any processor is missing ProcessBody/Name methods, this will fail at compile time
var (
	_ BodyProcessor = (*SafeNavASTProcessor)(nil)
	_ BodyProcessor = (*NullCoalesceASTProcessor)(nil)
	_ BodyProcessor = (*TernaryProcessor)(nil)
	_ BodyProcessor = (*ErrorPropASTProcessor)(nil)
	_ BodyProcessor = (*TypeAnnotASTProcessor)(nil)
	_ BodyProcessor = (*FunctionalProcessor)(nil)
)

// PassConfig defines the two-pass preprocessor architecture
//
// Pass 1 (Structural): Transforms that change code structure
//   - Lambda expressions, enums, tuples, pattern matching
//   - These processors generate BLOCKS of code (multiple lines)
//   - Must run FIRST to establish final code structure
//
// Pass 2 (Expression): Transforms within expressions
//   - Error propagation (?), safe navigation (?.),  null coalescing (??), ternary (? :)
//   - These processors transform SINGLE expressions
//   - Run AFTER structural transforms to process all expressions uniformly
//
// Lambda Body Injection:
//   - Lambda (Pass 1) needs expression processors (Pass 2)
//   - Uses BodyProcessor interface for decoupled injection
//   - Expression processors implement both FeatureProcessor and BodyProcessor
type PassConfig struct {
	// Pass 1: Structural transforms (change code shape)
	Structural []FeatureProcessor

	// Pass 2: Expression transforms (within expressions)
	Expression []FeatureProcessor
}

// NewDefaultPassConfig creates the default two-pass configuration
// This defines the canonical ordering of all Dingo preprocessors
func NewDefaultPassConfig() *PassConfig {
	// Pass 1: Structural Transforms (Change code shape)
	structural := []FeatureProcessor{
		// 0. Dingo Pre-Parser (let → var/short decl) - MUST be FIRST
		//    Transforms: let x: Type = val → var x: Type = val
		//    Transforms: let x = val → x := val
		NewDingoPreParser(),

		// 1. Generic syntax (<> → []) - BEFORE type annotations
		//    Transforms: Result<T,E> → Result[T,E]
		NewGenericSyntaxProcessor(),

		// 2. Pattern matching (match) - MUST run BEFORE lambdas (both use =>)
		//    Match arms: Pattern => Expression (structural context)
		//    Lambdas: params => expression (expression context)
		//    MIGRATED TO AST: Uses proper AST-based parsing instead of regex
		NewRustMatchASTProcessor(),

		// 3. Enums (enum Name { ... }) - AST-based
		//    Transforms: enum Result { Ok, Err } → struct types + constructors
		//    MIGRATED TO AST: Uses proper AST-based parsing instead of regex
		NewEnumASTProcessor(),

		// NOTE: Lambda is handled separately with body processor injection
		//       Lambda needs Pass 2 processors injected into body transformation

		// NOTE: Tuples moved AFTER Lambda to avoid matching lambda param lists
		//       See AllProcessorsWithLambda() for final ordering
	}

	// Pass 2: Expression Transforms (Within expressions)
	expression := []FeatureProcessor{
		// 0. Functional utilities (map, filter, reduce, etc.)
		//    Transforms: items.map(x => x * 2) → functional transformations
		//    MIGRATED TO AST: Uses proper AST-based parsing instead of regex
		NewFunctionalASTProcessor(),

		// 1. Type annotations (: → space) - AST-based
		//    Transforms: param: Type → param Type
		NewTypeAnnotASTProcessor(),

		// 2. Safe navigation (?.) - AST-based
		//    Transforms: user?.name → conditional access pattern
		//    MIGRATED TO AST: Uses proper AST-based parsing instead of regex
		NewSafeNavASTProcessor(),

		// 3. Null coalescing (??) - AST-based
		//    Transforms: x ?? default → ternary with nil check
		//    CRITICAL: Must run BEFORE TernaryProcessor and ErrorPropProcessor
		//    MIGRATED TO AST: Uses proper AST-based parsing instead of regex
		NewNullCoalesceASTProcessor(),

		// 4. Ternary operator (? :)
		//    Transforms: condition ? trueVal : falseVal → IIFE pattern
		//    Process ternary BEFORE error prop to cleanly separate ? : from single ?
		NewTernaryProcessor(),

		// 5. Error propagation (expr?) - AST-based, AFTER ternary (handles remaining ?)
		//    Transforms: x? → if err != nil { return ... }
		NewErrorPropASTProcessor(),
	}

	return &PassConfig{
		Structural: structural,
		Expression: expression,
	}
}

// GetBodyProcessors returns expression processors that implement BodyProcessor
// This enables lambda body injection without circular dependencies
//
// Usage:
//   config := NewDefaultPassConfig()
//   bodyProcessors := config.GetBodyProcessors()
//   lambda := NewLambdaASTProcessorWithBodyProcessors(bodyProcessors)
func (pc *PassConfig) GetBodyProcessors() []BodyProcessor {
	var bodyProcessors []BodyProcessor

	// Cast expression processors to BodyProcessor interface
	// Only processors that implement both interfaces will be included
	for _, proc := range pc.Expression {
		if bp, ok := proc.(BodyProcessor); ok {
			bodyProcessors = append(bodyProcessors, bp)
		}
	}

	return bodyProcessors
}

// AllProcessors returns all processors in correct execution order
// Pass 1 (Structural) → Pass 2 (Expression)
//
// NOTE: This does NOT include Lambda processor
// Lambda must be created separately with body processor injection
func (pc *PassConfig) AllProcessors() []FeatureProcessor {
	// Combine Pass 1 + Pass 2 in order
	all := make([]FeatureProcessor, 0, len(pc.Structural)+len(pc.Expression))
	all = append(all, pc.Structural...)
	all = append(all, pc.Expression...)
	return all
}

// AllProcessorsWithLambda returns all processors including Lambda and Tuple
// Lambda is injected between Pass 1 structural transforms and Pass 2 expression transforms
// Tuple is injected AFTER Lambda to avoid matching lambda parameter lists
//
// Final order: DingoPreParser → Generic → Match → Enum → **Lambda** → **Tuple** → Functional → TypeAnnot → SafeNav → NullCoalesce → Ternary → ErrorProp
//
// NOTE: Lambda body processing is currently handled inline within LambdaASTProcessor
// Future enhancement: Inject body processors via constructor for decoupled architecture
func (pc *PassConfig) AllProcessorsWithLambda() []FeatureProcessor {
	// Create lambda and tuple processors
	lambda := NewLambdaASTProcessor()
	tuple := NewTupleProcessor()

	// Build final processor list
	// Pass 1: Structural (without Lambda and Tuple)
	all := make([]FeatureProcessor, 0, len(pc.Structural)+2+len(pc.Expression))
	all = append(all, pc.Structural...)

	// Lambda: Between Pass 1 and Pass 2
	all = append(all, lambda)

	// Tuple: AFTER Lambda (to avoid matching lambda param lists as tuples)
	all = append(all, tuple)

	// Pass 2: Expression
	all = append(all, pc.Expression...)

	return all
}
