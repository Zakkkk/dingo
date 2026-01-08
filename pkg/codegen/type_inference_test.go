package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestInferReturnTypes(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		exprPos int
		want    []string
	}{
		{
			name:    "single pointer return",
			src:     "func foo() (*User, error) { x := bar()? }",
			exprPos: 35,
			want:    []string{"nil"},
		},
		{
			name:    "int and error",
			src:     "func count() (int, error) { n := fetch()? }",
			exprPos: 35,
			want:    []string{"0"},
		},
		{
			name:    "string and error",
			src:     "func name() (string, error) { s := get()? }",
			exprPos: 37,
			want:    []string{`""`},
		},
		{
			name:    "multiple returns",
			src:     "func multi() (int, string, *Config, error) { x := foo()? }",
			exprPos: 50,
			want:    []string{"0", `""`, "nil"},
		},
		{
			name:    "slice return",
			src:     "func list() ([]Item, error) { items := fetch()? }",
			exprPos: 42,
			want:    []string{"nil"},
		},
		{
			name:    "struct return",
			src:     "func config() (Config, error) { cfg := load()? }",
			exprPos: 40,
			want:    []string{"Config{}"},
		},
		{
			name:    "named returns",
			src:     "func process() (count int, msg string, err error) { x := do()? }",
			exprPos: 58,
			want:    []string{"0", `""`},
		},
		{
			name:    "package qualified type",
			src:     "func handler() (*http.Request, error) { req := parse()? }",
			exprPos: 50,
			want:    []string{"nil"},
		},
		{
			name:    "map return",
			src:     "func lookup() (map[string]int, error) { m := fetch()? }",
			exprPos: 47,
			want:    []string{"nil"},
		},
		{
			name:    "interface return",
			src:     "func get() (interface{}, error) { v := fetch()? }",
			exprPos: 42,
			want:    []string{"nil"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferReturnTypes([]byte(tt.src), tt.exprPos)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("at %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestZeroValueFor(t *testing.T) {
	tests := []struct {
		typeName string
		want     string
	}{
		{"int", "0"},
		{"int8", "0"},
		{"int16", "0"},
		{"int32", "0"},
		{"int64", "0"},
		{"uint", "0"},
		{"uint8", "0"},
		{"uint16", "0"},
		{"uint32", "0"},
		{"uint64", "0"},
		{"float32", "0"},
		{"float64", "0"},
		{"byte", "0"},
		{"rune", "0"},
		{"bool", "false"},
		{"string", `""`},
		{"error", "nil"},
		{"*User", "nil"},
		{"*string", "nil"},
		{"[]string", "nil"},
		{"[]int", "nil"},
		{"map[string]int", "nil"},
		{"map[int]bool", "nil"},
		{"interface{}", "nil"},
		{"any", "nil"},
		{"chan int", "nil"},
		{"chan string", "nil"},
		{"func()", "nil"},
		{"Config", "Config{}"},
		{"User", "User{}"},
		{"http.Request", "nil"},
		{"os.File", "nil"},
		{"pkg.Type", "nil"},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			got := zeroValueFor(tt.typeName)
			if got != tt.want {
				t.Errorf("zeroValueFor(%q) = %q, want %q", tt.typeName, got, tt.want)
			}
		})
	}
}

func TestFindEnclosingFunction(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		exprPos  int
		wantFunc bool
		skip     bool
	}{
		{
			name:     "simple function",
			src:      "package main\nfunc foo() error { x := bar()? }",
			exprPos:  30,
			wantFunc: true,
		},
		{
			name:     "outside function",
			src:      "package main\nvar x = 10",
			exprPos:  20,
			wantFunc: false,
		},
		{
			name:     "method",
			src:      "package main\nfunc (r *Repo) Save() error { x := db()? }",
			exprPos:  45,
			wantFunc: true,
		},
		{
			name: "nested function",
			src: `package main
func outer() error {
	inner := func() error {
		x := foo()?
		return nil
	}
	return inner()
}`,
			exprPos:  70,
			wantFunc: true, // Should find outer(), not inner
			skip:     true, // Known limitation: nested functions not yet supported
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("known limitation")
			}
			fn := findEnclosingFunction([]byte(tt.src), tt.exprPos)
			if (fn != nil) != tt.wantFunc {
				t.Errorf("findEnclosingFunction() found function = %v, want %v", fn != nil, tt.wantFunc)
			}
		})
	}
}

func TestParseReturnTypes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "single return",
			src:  "func foo() error { }",
			want: []string{"error"},
		},
		{
			name: "multiple returns",
			src:  "func foo() (int, error) { }",
			want: []string{"int", "error"},
		},
		{
			name: "named returns",
			src:  "func foo() (count int, err error) { }",
			want: []string{"int", "error"},
		},
		{
			name: "pointer return",
			src:  "func foo() (*User, error) { }",
			want: []string{"*User", "error"},
		},
		{
			name: "slice return",
			src:  "func foo() ([]string, error) { }",
			want: []string{"[]string", "error"},
		},
		{
			name: "map return",
			src:  "func foo() (map[string]int, error) { }",
			want: []string{"map[string]int", "error"},
		},
		{
			name: "no returns",
			src:  "func foo() { }",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "", "package p\n"+tt.src, 0)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			if len(f.Decls) == 0 {
				t.Fatal("no declarations found")
			}
			fn, ok := f.Decls[0].(*ast.FuncDecl)
			if !ok {
				t.Fatal("not a function declaration")
			}

			got := parseReturnTypes(fn)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("at %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
