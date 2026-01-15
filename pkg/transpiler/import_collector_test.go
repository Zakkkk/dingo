package transpiler

import (
	"testing"
)

func TestImportCollector_SingleImport(t *testing.T) {
	src := []byte(`package main
import "dgo"
func main() {}`)

	collector := ImportCollector{}
	imports, err := collector.CollectImports(src)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(imports) != 1 || imports[0] != "dgo" {
		t.Errorf("expected [dgo], got %v", imports)
	}
}

func TestImportCollector_MultipleImports(t *testing.T) {
	src := []byte(`package main
import (
	"dgo"
	"github.com/example/util"
	alias "github.com/example/other"
)
func main() {}`)

	collector := ImportCollector{}
	imports, err := collector.CollectImports(src)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"dgo", "github.com/example/util", "github.com/example/other"}
	if len(imports) != len(expected) {
		t.Errorf("expected %d imports, got %d: %v", len(expected), len(imports), imports)
		return
	}

	for i, exp := range expected {
		if imports[i] != exp {
			t.Errorf("import[%d]: expected %s, got %s", i, exp, imports[i])
		}
	}
}

func TestImportCollector_NoImports(t *testing.T) {
	src := []byte(`package main

func main() {}`)

	collector := ImportCollector{}
	imports, err := collector.CollectImports(src)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(imports) != 0 {
		t.Errorf("expected no imports, got %v", imports)
	}
}

func TestImportCollector_DotImport(t *testing.T) {
	src := []byte(`package main
import . "dgo"
func main() {}`)

	collector := ImportCollector{}
	imports, err := collector.CollectImports(src)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(imports) != 1 || imports[0] != "dgo" {
		t.Errorf("expected [dgo], got %v", imports)
	}
}

func TestImportCollector_BlankImport(t *testing.T) {
	src := []byte(`package main
import _ "database/sql"
func main() {}`)

	collector := ImportCollector{}
	imports, err := collector.CollectImports(src)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(imports) != 1 || imports[0] != "database/sql" {
		t.Errorf("expected [database/sql], got %v", imports)
	}
}

func TestImportCollector_DingoSyntax(t *testing.T) {
	// Dingo-specific syntax should not cause errors
	src := []byte(`package main
import (
	"dgo"
	"errors"
)

func GetUser(id string) (User, error) {
	user := loadUser(id)?
	return user, nil
}`)

	collector := ImportCollector{}
	imports, err := collector.CollectImports(src)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"dgo", "errors"}
	if len(imports) != len(expected) {
		t.Errorf("expected %d imports, got %d: %v", len(expected), len(imports), imports)
		return
	}

	for i, exp := range expected {
		if imports[i] != exp {
			t.Errorf("import[%d]: expected %s, got %s", i, exp, imports[i])
		}
	}
}

func TestImportCollector_MultipleSingleImports(t *testing.T) {
	src := []byte(`package main
import "fmt"
import "os"
func main() {}`)

	collector := ImportCollector{}
	imports, err := collector.CollectImports(src)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"fmt", "os"}
	if len(imports) != len(expected) {
		t.Errorf("expected %d imports, got %d: %v", len(expected), len(imports), imports)
		return
	}

	for i, exp := range expected {
		if imports[i] != exp {
			t.Errorf("import[%d]: expected %s, got %s", i, exp, imports[i])
		}
	}
}
