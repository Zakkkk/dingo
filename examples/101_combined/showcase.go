// Dingo Feature Showcase - ALL 12 features
// This file demonstrates every Dingo feature.
//
// === Design Decisions Summary ===
//
// Generic Types: Result[T,E] and Option[T] use dgo package generics
// Enums: Compile to Go interface + struct patterns
// Lambdas: |x| expr and (x) => expr compile to func literals
//
// To build: dingo build showcase.dingo
// To test:  go build showcase.go
package main

import (
	"fmt"
	"github.com/MadAppGang/dingo/pkg/dgo"
)

// === 1. ENUM ===
type Status interface{ isStatus() }

type StatusPending struct{}

func (StatusPending) isStatus() {}
func NewStatusPending() Status  { return StatusPending{} }

type StatusActive struct{ message string }

func (StatusActive) isStatus()              {}
func NewStatusActive(message string) Status { return StatusActive{message: message} }

type StatusDone struct{ code int }

func (StatusDone) isStatus()        {}
func NewStatusDone(code int) Status { return StatusDone{code: code} }

// === Types ===
type User struct {
	ID       int
	Name     string
	Settings *Settings
}

type Settings struct {
	Theme    string
	Language *string
}

// === 4. RESULT ===
func fetchUser(id int) dgo.Result[User, string] {
	if id <= 0 {
		return dgo.Err[User]("invalid id")
	}
	lang := "en"
	return dgo.Ok[User, string](User{
		ID:       id,
		Name:     "Alice",
		Settings: &Settings{Theme: "dark", Language: &lang},
	})
}

// === 5. OPTION ===
func findUser(name string) dgo.Option[User] {
	if name == "" {
		return dgo.None[User]()
	}
	return dgo.Some(User{ID: 1, Name: name})
}

// === 3. ERROR PROPAGATION ===
func loadConfig() (string, error) {
	return "config-data", nil
}

func processConfig() (string, error) {
	// Pattern 1: Basic ?
	tmp, err := loadConfig()
	if err != nil {
		return "", err
	}
	cfg := tmp

	// Pattern 2: String context
	tmp1, err1 := loadConfig()
	if err1 != nil {
		return "", fmt.Errorf("config load failed: %w", err1)
	}
	cfg2 := tmp1

	// Pattern 3: Lambda transform
	tmp2, err2 := loadConfig()
	if err2 != nil {
		return "", fmt.Errorf("wrapped: %w", err2)
	}
	cfg3 := tmp2

	return fmt.Sprintf("%s|%s|%s", cfg, cfg2, cfg3), nil
}

// === 6. LAMBDAS ===
// Standalone lambdas need type annotations (no context to infer from)
// Lambdas passed to generic functions get types inferred automatically
func useLambdas() {
	// Rust-style typed lambda (implicit return)
	fn1 := func(x int) int { return x * 2 }

	// TypeScript-style typed lambda (implicit return)
	fn2 := func(x int) int { return x * 3 }

	fmt.Println(fn1(5), fn2(5))
}

// === 7. TUPLES ===
func getPoint() (int, int) {
	return 10, 20
}

// === 11. GUARD ===
func guardLetDemo() string {
	// Guard with Option
	tmp := findUser("Bob")
	if tmp.IsNone() {

		return "user not found"

	}
	user := *tmp.Some

	// Guard with Result
	tmp1 := fetchUser(42)
	if tmp1.IsErr() {
		err := *tmp1.Err

		return fmt.Sprintf("fetch failed: %s", err)

	}
	fetched := *tmp1.Ok

	return fmt.Sprintf("Found: %s, Fetched: %s", user.Name, fetched.Name)
}

// === MAIN DEMO ===
func demo() string {
	lang := "en"
	user := User{
		ID:       42,
		Name:     "Alice",
		Settings: &Settings{Theme: "dark", Language: &lang},
	}

	// === 8. SAFE NAVIGATION ===
	userLang := func() *string {
		tmp := user.Settings
		if tmp == nil {
			return nil
		}
		return tmp.Language
	}()

	// === 10. NULL COALESCE ===
	displayLang := func() string {
		if userLang != nil {
			return *userLang
		}
		return "default"
	}()

	// === 9. TERNARY ===
	// BUG: Generates assignment without declaration - See BUGS.md Bug 2
	var greeting any
	if user.ID > 0 {
		greeting = "Welcome"
	} else {
		greeting = "Hello"
	}

	// === 2. MATCH ===
	status := NewStatusActive("working")
	var statusMsg string
	val := status
	switch v1 := val.(type) {
	case StatusPending:
		statusMsg = "pending"
	case StatusActive:
		message := v1.message
		statusMsg = message
	case StatusDone:
		code := v1.code
		statusMsg = fmt.Sprintf("done:%d", code)
	}

	// === 7. TUPLES (destructuring) ===
	// BUG: Mishandles Go multiple returns as tuple struct - See BUGS.md Bug 3
	x, y := getPoint()

	// Build result string
	result := fmt.Sprintf("%s %s! Lang=%s Status=%s Point=(%d,%d)",
		greeting, user.Name, displayLang, statusMsg, x, y)

	return result
}

func main() {
	fmt.Println("=== Demo ===")
	fmt.Println(demo())

	fmt.Println("\n=== Result ===")
	r := fetchUser(42)
	if r.IsOk() {
		fmt.Println("Fetched:", r.MustOk().Name)
	}

	fmt.Println("\n=== Guard Let ===")
	fmt.Println(guardLetDemo())

	fmt.Println("\n=== Error Prop ===")
	if cfg, err := processConfig(); err == nil {
		fmt.Println("Config:", cfg)
	}

	fmt.Println("\n=== Lambdas ===")
	useLambdas()
}
