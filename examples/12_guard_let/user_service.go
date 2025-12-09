// Guard Let Example: Early return pattern for Result and Option types
//
// Guard let shines when chaining multiple fallible operations.
// It keeps the "happy path" linear instead of nested if-else pyramids.
package main

import (
	"fmt"
	"github.com/MadAppGang/dingo/pkg/dgo"
)

// === Simulated external functions that return Result/Option ===
// (In real code, these would be DB queries, API calls, file reads, etc.)

type User struct {
	ID    int
	Name  string
	Email string
}

type Settings struct {
	Theme    string
	Language string
}

// Simulates DB lookup - returns Result
func fetchUser(id int) dgo.Result[User, string] {
	if id == 1 {
		u := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
		return dgo.Ok[User, string](u)
	}
	if id == 2 {
		u := User{ID: 2, Name: "Bob", Email: ""}
		return dgo.Ok[User, string](u)
	}
	return dgo.Err[User]("user not found")
}

// Simulates settings lookup - returns Result
func loadSettings(userId int) dgo.Result[Settings, string] {
	if userId == 1 {
		s := Settings{Theme: "dark", Language: "en"}
		return dgo.Ok[Settings, string](s)
	}
	return dgo.Err[Settings]("settings not found")
}

// Simulates optional preference lookup - returns Option
func getPreference(settings Settings, key string) dgo.Option[string] {
	if key == "theme" && settings.Theme != "" {
		return dgo.Some(settings.Theme)
	}
	if key == "language" && settings.Language != "" {
		return dgo.Some(settings.Language)
	}
	return dgo.None[string]()

}

// === Guard let in a producer function ===
// When a producer itself calls other fallible operations, guard let shines

// Simulates a complex user lookup that depends on another service
func fetchUserWithPermissions(id int) dgo.Result[User, string] {
	// First, fetch the base user
	tmp := fetchUser(id)
	if tmp.IsErr() {
		err := *tmp.Err

		return dgo.Err[User](fmt.Sprintf("user lookup failed: %s", err))

	}
	user := *tmp.Ok

	// Then verify the user has required permissions (another fallible call)
	tmp1 := loadSettings(user.ID)
	if tmp1.IsErr() {
		err := *tmp1.Err

		return dgo.Err[User](fmt.Sprintf("permissions check failed: %s", err))

	}
	settings := *tmp1.Ok

	// Only return user if they have settings (business rule)
	if settings.Theme == "" {
		return dgo.Err[User]("user has no theme configured")
	}

	return dgo.Ok[User, string](user)
}

// === The power of guard let: chaining multiple fallible operations ===

// WITHOUT guard let (nested pyramid of doom):
func getUserThemeNested(userId int) dgo.Result[string, string] {
	userResult := fetchUser(userId)
	if userResult.IsErr() {
		return dgo.Err[string](userResult.UnwrapErr())
	}
	user := userResult.Unwrap()

	settingsResult := loadSettings(user.ID)
	if settingsResult.IsErr() {
		return dgo.Err[string](settingsResult.UnwrapErr())
	}
	settings := settingsResult.Unwrap()

	themeOpt := getPreference(settings, "theme")
	if themeOpt.IsNone() {
		return dgo.Ok[string, string]("default")
	}

	return dgo.Ok[string, string](themeOpt.Unwrap())
}

// WITH guard let (linear happy path):
func getUserTheme(userId int) dgo.Result[string, string] {
	// Each guard let either unwraps or returns early
	tmp2 := fetchUser(userId)
	if tmp2.IsErr() {
		err := *tmp2.Err

		return dgo.Err[string](err)

	}
	user := *tmp2.Ok

	tmp3 := loadSettings(user.ID)
	if tmp3.IsErr() {
		err := *tmp3.Err

		return dgo.Err[string](err)

	}
	settings := *tmp3.Ok

	// Option type: no binding, just early return with default
	tmp4 := getPreference(settings, "theme")
	if tmp4.IsNone() {

		return dgo.Ok[string, string]("default")

	}
	theme := *tmp4.Some

	return dgo.Ok[string, string](theme)
}

// Another example: validating and processing user data
func processUserProfile(userId int) dgo.Result[string, string] {
	tmp5 := fetchUser(userId)
	if tmp5.IsErr() {
		err := *tmp5.Err

		return dgo.Err[string](fmt.Sprintf("fetch failed: %s", err))

	}
	user := *tmp5.Ok

	// Early return if email is missing
	if user.Email == "" {
		return dgo.Err[string]("user has no email")
	}

	tmp6 := loadSettings(user.ID)
	if tmp6.IsErr() {
		err := *tmp6.Err

		// Can do logging or cleanup in else block
		fmt.Printf("Warning: no settings for %s, using defaults\n", user.Name)
		return dgo.Err[string](err)

	}
	settings := *tmp6.Ok

	profile := fmt.Sprintf("%s (%s) - theme: %s", user.Name, user.Email, settings.Theme)
	return dgo.Ok[string, string](profile)
}

func main() {
	fmt.Println("=== Guard Let Demo ===\n")

	// Test with valid user
	fmt.Println("User 1 (Alice with settings):")
	r1 := getUserTheme(1)
	if r1.IsOk() {
		fmt.Printf("  Theme: %s\n", r1.Unwrap())
	}

	// Test with user missing settings
	fmt.Println("\nUser 2 (Bob, no settings):")
	r2 := getUserTheme(2)
	if r2.IsErr() {
		fmt.Printf("  Error: %s\n", r2.UnwrapErr())
	}

	// Test with non-existent user
	fmt.Println("\nUser 99 (not found):")
	r3 := getUserTheme(99)
	if r3.IsErr() {
		fmt.Printf("  Error: %s\n", r3.UnwrapErr())
	}

	// Test profile processing
	fmt.Println("\n--- Profile Processing ---")
	p1 := processUserProfile(1)
	if p1.IsOk() {
		fmt.Printf("Profile: %s\n", p1.Unwrap())
	}

	p2 := processUserProfile(2)
	if p2.IsErr() {
		fmt.Printf("Error: %s\n", p2.UnwrapErr())
	}

	// Test fetchUserWithPermissions (guard let in producer)
	fmt.Println("\n--- Guard Let in Producer ---")
	u1 := fetchUserWithPermissions(1)
	if u1.IsOk() {
		fmt.Printf("User with permissions: %s\n", u1.Unwrap().Name)
	}

	u2 := fetchUserWithPermissions(2)
	if u2.IsErr() {
		fmt.Printf("Error: %s\n", u2.UnwrapErr())
	}
}
