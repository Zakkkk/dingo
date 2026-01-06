// Ternary Operator Example
//
// Dingo supports C-style ternary expressions:
//
//	condition ? valueIfTrue : valueIfFalse
//
// === Design Decision: Context-Aware Ternary Generation ===
//
// Go lacks a ternary operator, so Dingo generates context-specific code:
//
//	Return context:     condition ? a : b
//	                    → if condition { return a }; return b
//
//	Assignment context: x := condition ? a : b
//	                    → var x T; if condition { x = a } else { x = b }
//
//	Nested ternary:     a ? b : c ? d : e
//	                    → if a { x = b } else if c { x = d } else { x = e }
//
// This produces idiomatic Go that gofmt preserves.
package main

import "fmt"

type User struct {
	Name     string
	Age      int
	IsAdmin  bool
	Verified bool
}

// GetUserStatus returns status based on admin flag
func GetUserStatus(user User) string {
	//line examples/09_ternary//permissions.dingo:34:9
	if user.IsAdmin {
		return "Administrator"
	}
	return "Standard User"
}

// GetAgeCategory returns category based on age (nested ternary)
func GetAgeCategory(age int) string {
	//line examples/09_ternary//permissions.dingo:39:9
	if age >= 65 {
		return "Senior"
	}
	if age >= 18 {
		return "Adult"
	}
	return "Minor"
}

// GetDisplayName returns formatted name with optional badge
func GetDisplayName(user User) string {
	//line examples/09_ternary//permissions.dingo:44:9
	if user.Verified {
		return fmt.Sprintf("%s ✓", user.Name)
	}
	return user.Name
}

// GetAccessLevel returns numeric access level (nested ternary with int)
func GetAccessLevel(isAdmin bool, isVerified bool) int {
	//line examples/09_ternary//permissions.dingo:49:9
	if isAdmin {
		return 100
	}
	if isVerified {
		return 50
	}
	return 10
}

func main() {
	fmt.Println("=== Ternary Operator Example ===")

	admin := User{Name: "Alice", Age: 30, IsAdmin: true, Verified: true}
	user := User{Name: "Bob", Age: 25, IsAdmin: false, Verified: true}
	guest := User{Name: "Charlie", Age: 16, IsAdmin: false, Verified: false}

	fmt.Println("--- User Status ---")
	fmt.Printf("%s: %s\n", admin.Name, GetUserStatus(admin))
	fmt.Printf("%s: %s\n", user.Name, GetUserStatus(user))

	fmt.Println("\n--- Age Categories ---")
	fmt.Printf("Age 70: %s\n", GetAgeCategory(70))
	fmt.Printf("Age 30: %s\n", GetAgeCategory(30))
	fmt.Printf("Age 16: %s\n", GetAgeCategory(16))

	fmt.Println("\n--- Display Names ---")
	fmt.Printf("%s\n", GetDisplayName(admin))
	fmt.Printf("%s\n", GetDisplayName(guest))

	fmt.Println("\n--- Access Levels ---")
	fmt.Printf("Admin+Verified: %d\n", GetAccessLevel(true, true))
	fmt.Printf("User+Verified:  %d\n", GetAccessLevel(false, true))
	fmt.Printf("Guest:          %d\n", GetAccessLevel(false, false))
}
