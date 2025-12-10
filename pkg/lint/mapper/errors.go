package mapper

import "errors"

var (
	// ErrNoMapping indicates a Go position has no mapping to Dingo source.
	// This typically occurs for generated code (boilerplate, wrapper functions, etc.)
	// that doesn't correspond to any original Dingo source.
	// Such diagnostics should be filtered out by the linter.
	ErrNoMapping = errors.New("mapper: position has no mapping (generated code)")
)
