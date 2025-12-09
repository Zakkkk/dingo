package builtin

import (
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/feature"
)

func TestEnumConstructorsPlugin_Transform(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		checks []string // strings that must be present in output
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
			// Check for key elements of the generated interface-based enum
			checks: []string{
				"type Status interface",
				"isStatus()",
				"type StatusPending struct",
				"type StatusActive struct",
				"type StatusDone struct",
				"func NewStatusPending()",
				"func NewStatusActive()",
				"func NewStatusDone()",
				"func main()",
				"NewStatusPending()", // constructor call transformed
			},
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
			checks: []string{
				"type Result",
				"interface",
				"isResult()",
				"type ResultOk",
				"type ResultErr",
				"func NewResultOk",
				"func NewResultErr",
				"func test()",
			},
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
			for _, check := range tt.checks {
				if !strings.Contains(resultStr, check) {
					t.Errorf("Output missing %q\nGot:\n%s", check, resultStr)
				}
			}
		})
	}
}
