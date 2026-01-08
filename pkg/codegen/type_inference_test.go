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

func TestExtractFunctionSignatures(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantFunc string // Expected function name to be found
		wantRet  string // Expected return type
	}{
		{
			name:     "simple function",
			src:      "package main\nfunc foo() error { return nil }",
			wantFunc: "foo",
			wantRet:  "error",
		},
		{
			name: "function with match expression",
			src: `package main
func process() Result[int, error] {
	match result {
		Ok(v) => return v,
		Err(e) => return e,
	}
}`,
			wantFunc: "process",
			wantRet:  "Result[int, error]",
		},
		{
			name:     "function with error propagation",
			src:      "package main\nfunc load() (*Config, error) { cfg := fetch()? }",
			wantFunc: "load",
			wantRet:  "*Config",
		},
		{
			name:     "method with receiver",
			src:      "package main\nfunc (f *Fetcher) GetKey(kid string) Result[any, error] { return Ok[any, error](nil) }",
			wantFunc: "GetKey",
			wantRet:  "Result[any, error]",
		},
		{
			name: "multiple functions with match",
			src: `package main
func first() int { return 1 }
func second() Result[string, error] {
	match x {
		Some(v) => v,
		None => "",
	}
}
func third() bool { return true }`,
			wantFunc: "second",
			wantRet:  "Result[string, error]",
		},
		{
			name: "nested braces in match",
			src: `package main
func fetch() Result[User, error] {
	match response {
		Ok(data) => {
			user := parse(data)
			return Ok[User, error](user)
		},
		Err(e) => {
			log.Printf("error: %v", e)
			return Err[User, error](e)
		},
	}
}`,
			wantFunc: "fetch",
			wantRet:  "Result[User, error]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use findMethodReturnType which internally uses extractFunctionSignatures
			ret := findMethodReturnType([]byte(tt.src), tt.wantFunc)
			if ret != tt.wantRet {
				t.Errorf("findMethodReturnType(%q) = %q, want %q", tt.wantFunc, ret, tt.wantRet)
			}
		})
	}
}

func TestFindMethodReturnTypeWithDingoSyntax(t *testing.T) {
	// Test case based on real passgate file with match expressions
	src := `package util

func NewJWKSFetcher(jwksURL string) Result[JWKSFetcher, error] {
	f.Refresh()?
	return Ok[JWKSFetcher, error](f)
}

func (f *jwksFetcher) backgroundRefresh(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			match f.fetchJWKSWithRetry() {
				Ok(_) => log.Printf("success"),
				Err(e) => log.Printf("error: %v", e),
			}
		}
	}
}

func (f *jwksFetcher) fetchJWKS() Result[Unit, error] {
	match keyResult {
		Ok(key) => {
			newKeys[jwk.Kid] = key
		},
		Err(e) => {
			log.Printf("warn: %v", e)
		},
	}
	return Ok[Unit, error](Unit{})
}
`

	tests := []struct {
		methodName string
		wantRet    string
	}{
		{"NewJWKSFetcher", "Result[JWKSFetcher, error]"},
		{"backgroundRefresh", ""}, // void return
		{"fetchJWKS", "Result[Unit, error]"},
	}

	for _, tt := range tests {
		t.Run(tt.methodName, func(t *testing.T) {
			ret := findMethodReturnType([]byte(src), tt.methodName)
			if ret != tt.wantRet {
				t.Errorf("findMethodReturnType(%q) = %q, want %q", tt.methodName, ret, tt.wantRet)
			}
		})
	}
}

func TestFindMethodReturnCountWithDingoSyntax(t *testing.T) {
	src := `package main

func single() error {
	match x {
		Ok(_) => nil,
		Err(e) => e,
	}
}

func double() (int, error) {
	return 0, nil
}

func triple() (string, int, error) {
	return "", 0, nil
}
`

	tests := []struct {
		methodName string
		wantCount  int
	}{
		{"single", 1},
		{"double", 2},
		{"triple", 3},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.methodName, func(t *testing.T) {
			count := findMethodReturnCount([]byte(src), tt.methodName)
			if count != tt.wantCount {
				t.Errorf("findMethodReturnCount(%q) = %d, want %d", tt.methodName, count, tt.wantCount)
			}
		})
	}
}

func TestExtractFunctionSignaturesSkipsFunctionTypes(t *testing.T) {
	// Test that function type definitions and function literals are NOT extracted
	// This caused parse errors like "func(*jwksFetcher)" without a name
	src := `package util

// Function type definition - should NOT be extracted
type JWKSFetcherOption func(*jwksFetcher)

// Regular named function - SHOULD be extracted
func NewJWKSFetcher(jwksURL string, options ...JWKSFetcherOption) Result[JWKSFetcher, error] {
	f := &jwksFetcher{url: jwksURL}
	return Ok[JWKSFetcher, error](f)
}

// Method returning function literal - should NOT extract the literal
func (f *jwksFetcher) GetOption() JWKSFetcherOption {
	return func(f *jwksFetcher) {
		f.timeout = 30
	}
}

// Method with Result return - SHOULD be extracted
func (f *jwksFetcher) Refresh() Result[Unit, error] {
	return Ok[Unit, error](Unit{})
}

// Anonymous function in variable assignment - should NOT be extracted
var handler = func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello"))
}

// Function literal as return value - should NOT extract the literal
func makeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}
}
`

	// Test that we correctly extract only named functions/methods
	tests := []struct {
		name      string
		wantFound bool
		wantRet   string
	}{
		{"NewJWKSFetcher", true, "Result[JWKSFetcher, error]"},
		{"GetOption", true, "JWKSFetcherOption"},
		{"Refresh", true, "Result[Unit, error]"},
		{"makeHandler", true, "http.HandlerFunc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret := findMethodReturnType([]byte(src), tt.name)
			found := ret != ""
			if found != tt.wantFound {
				t.Errorf("findMethodReturnType(%q) found=%v, want found=%v", tt.name, found, tt.wantFound)
			}
			if tt.wantFound && ret != tt.wantRet {
				t.Errorf("findMethodReturnType(%q) = %q, want %q", tt.name, ret, tt.wantRet)
			}
		})
	}

	// Verify the extracted signatures parse correctly (no parse errors)
	signatures := extractFunctionSignatures([]byte(src))
	if signatures == "" {
		t.Fatal("extractFunctionSignatures returned empty string")
	}

	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "", signatures, 0)
	if err != nil {
		t.Errorf("extractFunctionSignatures produced invalid Go: %v\nSignatures:\n%s", err, signatures)
	}
}

func TestExtractFunctionSignaturesPassgateCase(t *testing.T) {
	// Real-world test case from passgate that was causing parse errors
	src := `package util

type JWKSFetcherOption func(*jwksFetcher)

func WithRefreshInterval(d time.Duration) JWKSFetcherOption {
	return func(f *jwksFetcher) {
		f.refreshInterval = d
	}
}

func (f *jwksFetcher) Refresh() Result[Unit, error] {
	data := f.fetch()?
	f.parseKeys(data)?
	return Ok[Unit, error](Unit{})
}

func (f *jwksFetcher) fetch() Result[[]byte, error] {
	return Ok[[]byte, error](nil)
}
`

	// The key test: Refresh should be detected as returning Result[Unit, error]
	ret := findMethodReturnType([]byte(src), "Refresh")
	if ret != "Result[Unit, error]" {
		t.Errorf("findMethodReturnType(Refresh) = %q, want %q", ret, "Result[Unit, error]")
	}

	// Also verify parse doesn't fail
	signatures := extractFunctionSignatures([]byte(src))
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "", signatures, 0)
	if err != nil {
		t.Errorf("parse error: %v\nSignatures:\n%s", err, signatures)
	}
}
