// Dingo Feature Showcase - Key features working together
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

// Simple structs
type User struct {
	ID       int
	Name     string
	Settings *Settings
}

type Settings struct {
	Theme    string
	Language *string
}

// === 4. RESULT type ===
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

// Demo function with multiple features
func demo() string {
	lang := "en"
	user := User{
		ID:       42,
		Name:     "Alice",
		Settings: &Settings{Theme: "dark", Language: &lang},
	}

	// 8. SAFE NAVIGATION
	var userLang *string
	if user.Settings != nil {
		userLang = user.Settings.Language
	}

	// 10. NULL COALESCE
	displayLang := func() string {
		if userLang != nil {
			return *userLang
		}
		return "default"
	}()

	// 2. MATCH on enum
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

	// 12. LET binding
	result := fmt.Sprintf("User: %s, Lang: %s, Status: %s",
		user.Name, displayLang, statusMsg)

	return result
}

func main() {
	// Test basic demo
	fmt.Println(demo())

	// Test Result
	r := fetchUser(42)
	if r.IsOk() {
		fmt.Println("Fetched:", r.MustOk().Name)
	}
}
