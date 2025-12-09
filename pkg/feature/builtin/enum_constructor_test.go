package builtin

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/feature"
)

// TestEnumConstructorsPlugin_Transform tests enum constructor generation
// Note: We don't generate Is*() methods - users should use Go's native type switch:
//   switch s.(type) { case StatusPending: ... }
// This is more Go-idiomatic than helper methods.
func TestEnumConstructorsPlugin_Transform(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "unqualified constructor",
			input: `package main

enum Status {
	Pending
	Active
	Done
}

func main() {
	status := Pending()
	println(status)
}`,
			expected: `package main

type Status interface { isStatus() }

type StatusPending struct {}
func (StatusPending) isStatus() {}
func NewStatusPending() Status { return StatusPending{} }

type StatusActive struct {}
func (StatusActive) isStatus() {}
func NewStatusActive() Status { return StatusActive{} }

type StatusDone struct {}
func (StatusDone) isStatus() {}
func NewStatusDone() Status { return StatusDone{} }



func main() {
	status := NewStatusPending()
	println(status)
}`,
		},
		{
			name: "constructor with arguments",
			input: `package main

enum Result<T, E> {
	Ok(T)
	Err(E)
}

func test() Result<int, error> {
	return Ok(42)
}`,
			expected: `package main

type Result[T, E any] interface { isResult() }

type ResultOk[T, E any] struct {Value T}
func (ResultOk[T, E any]) isResult() {}
func NewResultOk[T, E any](value T) Result[T, E any] { return ResultOk[T, E any]{Value: value} }

type ResultErr[T, E any] struct {Value E}
func (ResultErr[T, E any]) isResult() {}
func NewResultErr[T, E any](value E) Result[T, E any] { return ResultErr[T, E any]{Value: value} }



func test() Result<int, error> {
	return NewResultOk(42)
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First apply enum transform (populates registry)
			enumPlugin := &EnumPlugin{}
			ctx := &feature.Context{
				Registry: feature.NewSharedRegistry(),
			}

			transformed1, err := enumPlugin.Transform([]byte(tt.input), ctx)
			if err != nil {
				t.Fatalf("EnumPlugin.Transform() error = %v", err)
			}

			// Then apply enum constructor transform
			constructorPlugin := &EnumConstructorsPlugin{}
			result, err := constructorPlugin.Transform(transformed1, ctx)
			if err != nil {
				t.Fatalf("EnumConstructorsPlugin.Transform() error = %v", err)
			}

			resultStr := string(result)
			if resultStr != tt.expected {
				t.Errorf("EnumConstructorsPlugin.Transform() =\n%s\n\nwant:\n%s", resultStr, tt.expected)
			}
		})
	}
}
