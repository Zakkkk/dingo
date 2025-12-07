package transpiler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// TestAnalyzeReturnType_Result tests detection of Result[T,E] return types.
func TestAnalyzeReturnType_Result(t *testing.T) {
	src := `package main

type User struct { ID int }
type DBError struct { Code string }

func FindUser(id int) Result[User, DBError] {
	return User{ID: id}
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	// Find the function
	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	if funcDecl == nil {
		t.Fatal("function not found")
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	if info == nil {
		t.Fatal("expected return type info, got nil")
	}

	if info.Kind != "result" {
		t.Errorf("expected kind 'result', got %q", info.Kind)
	}

	if info.TAstExpr == nil {
		t.Error("expected TAstExpr to be set")
	} else {
		if ident, ok := info.TAstExpr.(*ast.Ident); ok {
			if ident.Name != "User" {
				t.Errorf("expected T type 'User', got %q", ident.Name)
			}
		}
	}

	if info.EAstExpr == nil {
		t.Error("expected EAstExpr to be set")
	} else {
		if ident, ok := info.EAstExpr.(*ast.Ident); ok {
			if ident.Name != "DBError" {
				t.Errorf("expected E type 'DBError', got %q", ident.Name)
			}
		}
	}
}

// TestAnalyzeReturnType_Option tests detection of Option[T] return types.
func TestAnalyzeReturnType_Option(t *testing.T) {
	src := `package main

type User struct { ID int }

func FindUser(id int) Option[User] {
	return nil
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	if funcDecl == nil {
		t.Fatal("function not found")
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	if info == nil {
		t.Fatal("expected return type info, got nil")
	}

	if info.Kind != "option" {
		t.Errorf("expected kind 'option', got %q", info.Kind)
	}

	if info.TAstExpr == nil {
		t.Error("expected TAstExpr to be set")
	} else {
		if ident, ok := info.TAstExpr.(*ast.Ident); ok {
			if ident.Name != "User" {
				t.Errorf("expected T type 'User', got %q", ident.Name)
			}
		}
	}

	if info.EAstExpr != nil {
		t.Error("expected EAstExpr to be nil for Option")
	}
}

// TestAnalyzeReturnType_NotResultOrOption tests functions that don't return Result/Option.
func TestAnalyzeReturnType_NotResultOrOption(t *testing.T) {
	src := `package main

func GetInt() int {
	return 42
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "GetInt" {
			funcDecl = fn
			break
		}
	}

	if funcDecl == nil {
		t.Fatal("function not found")
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	if info != nil {
		t.Errorf("expected nil for non-Result/Option return, got %+v", info)
	}
}

// TestDetermineWrapper_ResultOk tests wrapping success values with Ok.
func TestDetermineWrapper_ResultOk(t *testing.T) {
	src := `package main

type User struct { ID int }
type DBError struct { Code string }

func FindUser(id int) Result[User, DBError] {
	return User{ID: id}
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	// Find the return statement
	var returnStmt *ast.ReturnStmt
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			returnStmt = ret
			return false
		}
		return true
	})

	if returnStmt == nil || len(returnStmt.Results) == 0 {
		t.Fatal("return statement not found")
	}

	wrapper := analyzer.DetermineWrapper(returnStmt.Results[0], info)

	if wrapper != WrapperOk {
		t.Errorf("expected WrapperOk, got %v", wrapper)
	}
}

// TestDetermineWrapper_ResultErr tests wrapping error values with Err.
func TestDetermineWrapper_ResultErr(t *testing.T) {
	src := `package main

type User struct { ID int }
type DBError struct { Code string }

func FindUser(id int) Result[User, DBError] {
	return DBError{Code: "NOT_FOUND"}
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	var returnStmt *ast.ReturnStmt
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			returnStmt = ret
			return false
		}
		return true
	})

	if returnStmt == nil || len(returnStmt.Results) == 0 {
		t.Fatal("return statement not found")
	}

	wrapper := analyzer.DetermineWrapper(returnStmt.Results[0], info)

	if wrapper != WrapperErr {
		t.Errorf("expected WrapperErr, got %v", wrapper)
	}
}

// TestDetermineWrapper_OptionSome tests wrapping non-nil values with Some.
func TestDetermineWrapper_OptionSome(t *testing.T) {
	src := `package main

type User struct { ID int }

func FindUser(id int) Option[User] {
	return User{ID: id}
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	var returnStmt *ast.ReturnStmt
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			returnStmt = ret
			return false
		}
		return true
	})

	if returnStmt == nil || len(returnStmt.Results) == 0 {
		t.Fatal("return statement not found")
	}

	wrapper := analyzer.DetermineWrapper(returnStmt.Results[0], info)

	if wrapper != WrapperSome {
		t.Errorf("expected WrapperSome, got %v", wrapper)
	}
}

// TestDetermineWrapper_OptionNone tests wrapping nil with None.
func TestDetermineWrapper_OptionNone(t *testing.T) {
	src := `package main

type User struct { ID int }

func FindUser(id int) Option[User] {
	return nil
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	var returnStmt *ast.ReturnStmt
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			returnStmt = ret
			return false
		}
		return true
	})

	if returnStmt == nil || len(returnStmt.Results) == 0 {
		t.Fatal("return statement not found")
	}

	wrapper := analyzer.DetermineWrapper(returnStmt.Results[0], info)

	if wrapper != WrapperNone {
		t.Errorf("expected WrapperNone, got %v", wrapper)
	}
}

// TestIsAlreadyWrapped_NonGeneric tests detection of already-wrapped returns.
func TestIsAlreadyWrapped_NonGeneric(t *testing.T) {
	src := `package main
import "github.com/MadAppGang/dingo/pkg/dgo"

type User struct { ID int }
type DBError struct { Code string }

func FindUser(id int) Result[User, DBError] {
	return dgo.Ok(User{ID: id})
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	var returnStmt *ast.ReturnStmt
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			returnStmt = ret
			return false
		}
		return true
	})

	if returnStmt == nil || len(returnStmt.Results) == 0 {
		t.Fatal("return statement not found")
	}

	wrapper := analyzer.DetermineWrapper(returnStmt.Results[0], info)

	if wrapper != WrapperSkip {
		t.Errorf("expected WrapperSkip for already wrapped, got %v", wrapper)
	}
}

// TestIsAlreadyWrapped_Generic tests detection of generic wrapped returns.
func TestIsAlreadyWrapped_Generic(t *testing.T) {
	src := `package main
import "github.com/MadAppGang/dingo/pkg/dgo"

type User struct { ID int }
type DBError struct { Code string }

func FindUser(id int) Result[User, DBError] {
	return dgo.Ok[User, DBError](User{ID: id})
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	var returnStmt *ast.ReturnStmt
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			returnStmt = ret
			return false
		}
		return true
	})

	if returnStmt == nil || len(returnStmt.Results) == 0 {
		t.Fatal("return statement not found")
	}

	wrapper := analyzer.DetermineWrapper(returnStmt.Results[0], info)

	if wrapper != WrapperSkip {
		t.Errorf("expected WrapperSkip for already wrapped, got %v", wrapper)
	}
}

// TestDetermineWrapper_ErrorInterface tests detection via error interface.
func TestDetermineWrapper_ErrorInterface(t *testing.T) {
	src := `package main
import "errors"

type User struct { ID int }

func FindUser(id int) Result[User, error] {
	return errors.New("not found")
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	checker, err := typechecker.New(fset, file, "main")
	if err != nil {
		t.Fatalf("type check error: %v", err)
	}

	analyzer := NewReturnAnalyzer(checker)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	var returnStmt *ast.ReturnStmt
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			returnStmt = ret
			return false
		}
		return true
	})

	if returnStmt == nil || len(returnStmt.Results) == 0 {
		t.Fatal("return statement not found")
	}

	wrapper := analyzer.DetermineWrapper(returnStmt.Results[0], info)

	if wrapper != WrapperErr {
		t.Errorf("expected WrapperErr for error type, got %v", wrapper)
	}
}

// TestDetermineWrapper_WithoutTypeChecker tests fallback behavior without type checker.
func TestDetermineWrapper_WithoutTypeChecker(t *testing.T) {
	src := `package main

type User struct { ID int }
type DBError struct { Code string }

func FindUser(id int) Result[User, DBError] {
	return DBError{Code: "NOT_FOUND"}
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Create analyzer WITHOUT type checker
	analyzer := NewReturnAnalyzer(nil)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "FindUser" {
			funcDecl = fn
			break
		}
	}

	info := analyzer.AnalyzeReturnType(funcDecl)

	// Without type checker, TType and EType should be nil
	if info.TType != nil {
		t.Error("expected TType to be nil without type checker")
	}
	if info.EType != nil {
		t.Error("expected EType to be nil without type checker")
	}

	// But it should still detect composite literal matching E type name
	var returnStmt *ast.ReturnStmt
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			returnStmt = ret
			return false
		}
		return true
	})

	wrapper := analyzer.DetermineWrapper(returnStmt.Results[0], info)

	// Fallback heuristic should detect DBError composite literal as error
	if wrapper != WrapperErr {
		t.Errorf("expected WrapperErr even without type checker (heuristic), got %v", wrapper)
	}
}
