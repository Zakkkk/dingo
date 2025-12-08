package ast

// SourceMapping represents a mapping from Dingo source positions to Go output positions.
// Used by the LSP for translating positions between the two representations.
type SourceMapping struct {
	DingoStart int    // Start byte offset in Dingo source (inclusive)
	DingoEnd   int    // End byte offset in Dingo source (exclusive)
	GoStart    int    // Start byte offset in Go output (inclusive)
	GoEnd      int    // End byte offset in Go output (exclusive)
	Kind       string // Type of mapping (e.g., "tuple_literal", "tuple_destructure")
}

// NewSourceMapping creates a new SourceMapping with the given positions.
func NewSourceMapping(dingoStart, dingoEnd, goStart, goEnd int, kind string) SourceMapping {
	return SourceMapping{
		DingoStart: dingoStart,
		DingoEnd:   dingoEnd,
		GoStart:    goStart,
		GoEnd:      goEnd,
		Kind:       kind,
	}
}

// CodeGenResult represents the output of a code generation operation.
// It contains the generated Go code and any source mappings.
type CodeGenResult struct {
	Output          []byte          // Generated Go code
	Mappings        []SourceMapping // Source mappings for LSP
	StatementOutput []byte          // Statement-level output (for hoisting)
	HoistedCode     []byte          // Code to hoist before the expression
	Error           *CodeGenError   // Error if code generation failed
}

// CodeGenError represents an error during code generation.
type CodeGenError struct {
	Message  string
	Pos      int    // Position in source where error occurred
	Position int    // Line/column position (for LSP)
	Hint     string // Optional hint for fixing the error
}

// NewCodeGenResult creates a new CodeGenResult with the given output.
// Mappings default to empty if not provided.
func NewCodeGenResult(output []byte, mappings ...[]SourceMapping) CodeGenResult {
	var m []SourceMapping
	if len(mappings) > 0 {
		m = mappings[0]
	}
	return CodeGenResult{
		Output:   output,
		Mappings: m,
	}
}

// MappingBuilder helps build source mappings incrementally.
type MappingBuilder struct {
	mappings []SourceMapping
}

// NewMappingBuilder creates a new MappingBuilder.
func NewMappingBuilder() *MappingBuilder {
	return &MappingBuilder{
		mappings: make([]SourceMapping, 0),
	}
}

// Add adds a new source mapping with a single output position.
// This is the simplified form used by match.go and other existing code.
// The goPos is used for both GoStart and GoEnd.
func (b *MappingBuilder) Add(dingoStart, dingoEnd, goPos int, kind string) {
	b.mappings = append(b.mappings, SourceMapping{
		DingoStart: dingoStart,
		DingoEnd:   dingoEnd,
		GoStart:    goPos,
		GoEnd:      goPos,
		Kind:       kind,
	})
}

// AddRange adds a new source mapping with explicit start and end positions.
func (b *MappingBuilder) AddRange(dingoStart, dingoEnd, goStart, goEnd int, kind string) {
	b.mappings = append(b.mappings, SourceMapping{
		DingoStart: dingoStart,
		DingoEnd:   dingoEnd,
		GoStart:    goStart,
		GoEnd:      goEnd,
		Kind:       kind,
	})
}

// Build returns the accumulated source mappings.
func (b *MappingBuilder) Build() []SourceMapping {
	return b.mappings
}
