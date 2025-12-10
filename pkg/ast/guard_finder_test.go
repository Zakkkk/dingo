package ast

import (
	"testing"
)

func TestFindGuardStatements_SingleBinding(t *testing.T) {
	src := []byte(`package main

func process(id int) Result[User, error] {
    guard user := FindUser(id) else |err| { return Err(err) }
    return Ok(user)
}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(locs))
	}

	loc := locs[0]

	// Check binding
	if loc.IsTuple {
		t.Errorf("Expected single binding, got tuple")
	}
	if len(loc.VarNames) != 1 || loc.VarNames[0] != "user" {
		t.Errorf("Expected VarNames=[user], got %v", loc.VarNames)
	}

	// Check IsDecl
	if !loc.IsDecl {
		t.Errorf("Expected IsDecl=true for :=")
	}

	// Check expression
	expectedExpr := "FindUser(id)"
	if loc.ExprText != expectedExpr {
		t.Errorf("Expected ExprText=%q, got %q", expectedExpr, loc.ExprText)
	}

	// Check pipe binding
	if !loc.HasBinding {
		t.Errorf("Expected HasBinding=true")
	}
	if loc.BindingName != "err" {
		t.Errorf("Expected BindingName=err, got %q", loc.BindingName)
	}

	// Check line/column
	if loc.Line != 4 {
		t.Errorf("Expected Line=4, got %d", loc.Line)
	}
	if loc.Column < 1 {
		t.Errorf("Expected Column >= 1, got %d", loc.Column)
	}

	// Verify extracted else block
	elseBlock := string(src[loc.ElseStart:loc.ElseEnd])
	expectedElse := " return Err(err) "
	if elseBlock != expectedElse {
		t.Errorf("Expected else block=%q, got %q", expectedElse, elseBlock)
	}
}

func TestFindGuardStatements_TupleBinding(t *testing.T) {
	src := []byte(`package main

func parse(data string) Result[Info, error] {
    guard (name, age) := ParseInfo(data) else |e| { return Err(e) }
    return Ok(Info{name, age})
}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(locs))
	}

	loc := locs[0]

	// Check tuple binding
	if !loc.IsTuple {
		t.Errorf("Expected IsTuple=true")
	}
	if len(loc.VarNames) != 2 {
		t.Fatalf("Expected 2 var names, got %d", len(loc.VarNames))
	}
	if loc.VarNames[0] != "name" || loc.VarNames[1] != "age" {
		t.Errorf("Expected VarNames=[name, age], got %v", loc.VarNames)
	}

	// Check IsDecl
	if !loc.IsDecl {
		t.Errorf("Expected IsDecl=true for :=")
	}

	// Check expression
	expectedExpr := "ParseInfo(data)"
	if loc.ExprText != expectedExpr {
		t.Errorf("Expected ExprText=%q, got %q", expectedExpr, loc.ExprText)
	}

	// Check pipe binding
	if !loc.HasBinding {
		t.Errorf("Expected HasBinding=true")
	}
	if loc.BindingName != "e" {
		t.Errorf("Expected BindingName=e, got %q", loc.BindingName)
	}
}

func TestFindGuardStatements_NoBinding(t *testing.T) {
	src := []byte(`package main

func load() Option[Config] {
    guard config := LoadConfig() else { return None() }
    return Some(config)
}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(locs))
	}

	loc := locs[0]

	// Check no pipe binding
	if loc.HasBinding {
		t.Errorf("Expected HasBinding=false, got true with binding=%q", loc.BindingName)
	}

	// Check variable name
	if len(loc.VarNames) != 1 || loc.VarNames[0] != "config" {
		t.Errorf("Expected VarNames=[config], got %v", loc.VarNames)
	}

	// Check IsDecl
	if !loc.IsDecl {
		t.Errorf("Expected IsDecl=true for :=")
	}

	// Check expression
	expectedExpr := "LoadConfig()"
	if loc.ExprText != expectedExpr {
		t.Errorf("Expected ExprText=%q, got %q", expectedExpr, loc.ExprText)
	}

	// Verify else block
	elseBlock := string(src[loc.ElseStart:loc.ElseEnd])
	expectedElse := " return None() "
	if elseBlock != expectedElse {
		t.Errorf("Expected else block=%q, got %q", expectedElse, elseBlock)
	}
}

func TestFindGuardStatements_MultiLine(t *testing.T) {
	src := []byte(`package main

func complex() Result[Data, error] {
    guard data := FetchData(url) else |err| {
        log.Error("fetch failed", err)
        return Err(err)
    }
    return Ok(data)
}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(locs))
	}

	loc := locs[0]

	// Verify multi-line else block extraction
	elseBlock := string(src[loc.ElseStart:loc.ElseEnd])
	if len(elseBlock) == 0 {
		t.Errorf("Expected non-empty else block")
	}
	// Should contain both log and return statements
	if !contains(elseBlock, "log.Error") || !contains(elseBlock, "return Err") {
		t.Errorf("Expected else block to contain both statements, got %q", elseBlock)
	}
}

func TestFindGuardStatements_Multiple(t *testing.T) {
	src := []byte(`package main

func chain() Result[Result, error] {
    guard a := First() else |e1| { return Err(e1) }
    guard (b, c) := Second(a) else |e2| { return Err(e2) }
    guard d := Third(b, c) else { return None() }
    return Ok(Result{a, b, c, d})
}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 3 {
		t.Fatalf("Expected 3 guards, got %d", len(locs))
	}

	// First guard
	if len(locs[0].VarNames) != 1 || locs[0].VarNames[0] != "a" {
		t.Errorf("First guard: expected VarNames=[a], got %v", locs[0].VarNames)
	}
	if locs[0].BindingName != "e1" {
		t.Errorf("First guard: expected BindingName=e1, got %q", locs[0].BindingName)
	}

	// Second guard (tuple)
	if !locs[1].IsTuple {
		t.Errorf("Second guard: expected IsTuple=true")
	}
	if len(locs[1].VarNames) != 2 || locs[1].VarNames[0] != "b" || locs[1].VarNames[1] != "c" {
		t.Errorf("Second guard: expected VarNames=[b, c], got %v", locs[1].VarNames)
	}
	if locs[1].BindingName != "e2" {
		t.Errorf("Second guard: expected BindingName=e2, got %q", locs[1].BindingName)
	}

	// Third guard (no binding)
	if locs[2].HasBinding {
		t.Errorf("Third guard: expected HasBinding=false")
	}
	if len(locs[2].VarNames) != 1 || locs[2].VarNames[0] != "d" {
		t.Errorf("Third guard: expected VarNames=[d], got %v", locs[2].VarNames)
	}
}

func TestFindGuardStatements_ComplexExpression(t *testing.T) {
	src := []byte(`package main

func nested() Result[Value, error] {
    guard val := process(fetch(id).Map(transform)) else |err| { return Err(err) }
    return Ok(val)
}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(locs))
	}

	loc := locs[0]

	// Check expression includes nested calls
	expectedExpr := "process(fetch(id).Map(transform))"
	if loc.ExprText != expectedExpr {
		t.Errorf("Expected ExprText=%q, got %q", expectedExpr, loc.ExprText)
	}
}

func TestFindGuardStatements_ThreeElementTuple(t *testing.T) {
	src := []byte(`package main

func parse3() Result[Triple, error] {
    guard (x, y, z) := Parse3D(data) else |e| { return Err(e) }
    return Ok(Triple{x, y, z})
}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(locs))
	}

	loc := locs[0]

	// Check 3-element tuple
	if !loc.IsTuple {
		t.Errorf("Expected IsTuple=true")
	}
	if len(loc.VarNames) != 3 {
		t.Fatalf("Expected 3 var names, got %d", len(loc.VarNames))
	}
	if loc.VarNames[0] != "x" || loc.VarNames[1] != "y" || loc.VarNames[2] != "z" {
		t.Errorf("Expected VarNames=[x, y, z], got %v", loc.VarNames)
	}
}

func TestFindGuardStatements_NestedBraces(t *testing.T) {
	src := []byte(`package main

func withNested() Result[Data, error] {
    guard data := fetch() else |err| {
        if err.Critical {
            panic(err)
        }
        return Err(err)
    }
    return Ok(data)
}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(locs))
	}

	loc := locs[0]

	// Verify else block includes nested braces
	elseBlock := string(src[loc.ElseStart:loc.ElseEnd])
	if !contains(elseBlock, "if err.Critical") || !contains(elseBlock, "panic") {
		t.Errorf("Expected else block to contain nested if statement, got %q", elseBlock)
	}
}

func TestFindGuardStatements_EmptySource(t *testing.T) {
	src := []byte(`package main

func empty() {}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 0 {
		t.Errorf("Expected 0 guards in empty source, got %d", len(locs))
	}
}

func TestFindGuardStatements_Assignment(t *testing.T) {
	src := []byte(`package main

func reassign(x int) Result[User, error] {
    var user User
    guard user = FindUser(x) else |err| { return Err(err) }
    return Ok(user)
}
`)

	locs, err := FindGuardStatements(src)
	if err != nil {
		t.Fatalf("FindGuardStatements failed: %v", err)
	}

	if len(locs) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(locs))
	}

	loc := locs[0]

	// Check IsDecl is false for =
	if loc.IsDecl {
		t.Errorf("Expected IsDecl=false for =")
	}

	// Check variable name
	if len(loc.VarNames) != 1 || loc.VarNames[0] != "user" {
		t.Errorf("Expected VarNames=[user], got %v", loc.VarNames)
	}
}

func TestFindGuardStatements_LegacySyntaxError(t *testing.T) {
	src := []byte(`package main

func legacy() Result[User, error] {
    guard let user = FindUser(1) else |err| { return Err(err) }
    return Ok(user)
}
`)

	_, err := FindGuardStatements(src)
	if err == nil {
		t.Fatalf("Expected error for legacy 'guard let' syntax, got nil")
	}

	// Check error message (now includes line:col: prefix)
	expectedMsg := "4:5: guard let syntax removed: use 'guard x :=' instead of 'guard let x ='"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
