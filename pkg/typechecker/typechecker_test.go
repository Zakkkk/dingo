package typechecker

import (
	"testing"
)

func TestSourceChecker_GetExprType(t *testing.T) {
	src := []byte(`
package main

type Config struct {
	Database *Database
	Name     string
	Count    int
}

type Database struct {
	Host     *string
	Port     int
	SSL      *SSLConfig
}

type SSLConfig struct {
	CAPath *string
	Verify bool
}

func example(config *Config) {
	_ = config.Database.Host
	_ = config.Database.Port
	_ = config.Database.SSL.CAPath
	_ = config.Name
	_ = config.Count
}
`)

	sc := NewSourceChecker()
	err := sc.ParseAndCheck("test.go", src)
	if err != nil {
		t.Fatalf("ParseAndCheck failed: %v", err)
	}

	tests := []struct {
		expr     string
		wantType string
	}{
		{"config.Database.Host", "*string"},
		{"config.Database.Port", "int"},
		{"config.Database.SSL.CAPath", "*string"},
		{"config.Database.SSL", "*SSLConfig"},
		{"config.Database", "*Database"},
		{"config.Name", "string"},
		{"config.Count", "int"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := sc.GetExprType(tt.expr)
			if got != tt.wantType {
				t.Errorf("GetExprType(%q) = %q, want %q", tt.expr, got, tt.wantType)
			}
		})
	}
}

func TestSourceChecker_GetAllExprTypes(t *testing.T) {
	src := []byte(`
package main

type User struct {
	Profile *Profile
}

type Profile struct {
	Name string
	Age  int
}

func test(user *User) {
	_ = user.Profile.Name
	_ = user.Profile.Age
}
`)

	sc := NewSourceChecker()
	err := sc.ParseAndCheck("test.go", src)
	if err != nil {
		t.Fatalf("ParseAndCheck failed: %v", err)
	}

	cache := sc.GetAllExprTypes()

	// Should have at least these entries
	expected := map[string]string{
		"user.Profile.Name": "string",
		"user.Profile.Age":  "int",
		"user.Profile":      "*Profile",
	}

	for expr, wantType := range expected {
		if got, ok := cache[expr]; !ok {
			t.Errorf("cache missing %q", expr)
		} else if got != wantType {
			t.Errorf("cache[%q] = %q, want %q", expr, got, wantType)
		}
	}
}

func TestInferSafeNavTypes(t *testing.T) {
	src := []byte(`
package main

type Config struct {
	Database *Database
}

type Database struct {
	Host *string
	Port int
}

func example(config *Config) {
	_ = config.Database.Host
	_ = config.Database.Port
}
`)

	chains := []string{
		"config.Database.Host",
		"config.Database.Port",
	}

	cache := InferSafeNavTypes(src, chains)

	if got := cache["config.Database.Host"]; got != "*string" {
		t.Errorf("config.Database.Host = %q, want *string", got)
	}
	if got := cache["config.Database.Port"]; got != "int" {
		t.Errorf("config.Database.Port = %q, want int", got)
	}
}

func TestChainToExprString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"config?.Database?.Host", "config.Database.Host"},
		{"user?.Profile", "user.Profile"},
		{"a?.b?.c?.d", "a.b.c.d"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ChainToExprString(tt.input)
			if got != tt.want {
				t.Errorf("ChainToExprString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSourceChecker_PartialErrors(t *testing.T) {
	// Source with type errors - should still work for valid parts
	src := []byte(`
package main

type Config struct {
	Host *string
}

func example(config *Config) {
	_ = config.Host
	_ = unknownVar.Something
}
`)

	sc := NewSourceChecker()
	// Should not return error - we ignore type errors during parsing
	err := sc.ParseAndCheck("test.go", src)
	if err != nil {
		t.Fatalf("ParseAndCheck should not fail on type errors: %v", err)
	}

	// Valid expression should still have type
	if got := sc.GetExprType("config.Host"); got != "*string" {
		t.Errorf("config.Host = %q, want *string", got)
	}
}
