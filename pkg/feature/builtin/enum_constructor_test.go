package builtin

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/feature"
)

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

type Status interface { isStatus(); IsPending() bool; IsActive() bool; IsDone() bool }

type StatusPending struct{}
func (StatusPending) isStatus() {}
func NewStatusPending() Status { return StatusPending{} }

type StatusActive struct{}
func (StatusActive) isStatus() {}
func NewStatusActive() Status { return StatusActive{} }

type StatusDone struct{}
func (StatusDone) isStatus() {}
func NewStatusDone() Status { return StatusDone{} }

func IsPending(s Status) bool { _, ok := s.(StatusPending); return ok }
func IsActive(s Status) bool { _, ok := s.(StatusActive); return ok }
func IsDone(s Status) bool { _, ok := s.(StatusDone); return ok }

func (s StatusPending) IsPending() bool { return true }
func (s StatusPending) IsActive() bool { return false }
func (s StatusPending) IsDone() bool { return false }

func (s StatusActive) IsPending() bool { return false }
func (s StatusActive) IsActive() bool { return true }
func (s StatusActive) IsDone() bool { return false }

func (s StatusDone) IsPending() bool { return false }
func (s StatusDone) IsActive() bool { return false }
func (s StatusDone) IsDone() bool { return true }

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

type Result[T, E any] interface { isResult(); IsOk() bool; IsErr() bool }

type ResultOk[T, E any] struct{value0 T}
func (ResultOk[T, E]) isResult() {}
func NewResultOk[T, E any](value0 T) Result[T, E] { return ResultOk[T, E]{value0} }

type ResultErr[T, E any] struct{value0 E}
func (ResultErr[T, E]) isResult() {}
func NewResultErr[T, E any](value0 E) Result[T, E] { return ResultErr[T, E]{value0} }

func IsOk[T, E any](r Result[T, E]) bool { _, ok := r.(ResultOk[T, E]); return ok }
func IsErr[T, E any](r Result[T, E]) bool { _, ok := r.(ResultErr[T, E]); return ok }

func (r ResultOk[T, E]) IsOk() bool { return true }
func (r ResultOk[T, E]) IsErr() bool { return false }

func (r ResultErr[T, E]) IsOk() bool { return false }
func (r ResultErr[T, E]) IsErr() bool { return true }

func test() Result[int, error] {
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
