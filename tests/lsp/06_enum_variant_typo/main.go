//go:build ignore
package main

// Test: Enum variant - bare variant name should work
// This file should compile without errors

//line /Users/jack/mag/dingo/tests/lsp/06_enum_variant_typo/main.dingo:6:1
type Status interface{ isStatus() }

type StatusActive struct{}

func (StatusActive) isStatus() {}
func NewStatusActive() Status  { return StatusActive{} }

type StatusInactive struct{}

func (StatusInactive) isStatus() {}
func NewStatusInactive() Status  { return StatusInactive{} }

//line /Users/jack/mag/dingo/tests/lsp/06_enum_variant_typo/main.dingo:10:1

func test() Status {
	return Actve // Bare variant name - transpiler adds NewStatusActive()
}

func main() {}
