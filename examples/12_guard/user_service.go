// Guard Example: Early return pattern for Result and Option types
//
// Syntax:
//
//	guard x := expr else |err| { ... }   // Result - declare new var, bind error
//	guard x := expr else { ... }         // Option - declare new var, no binding
//	guard x = expr else |err| { ... }    // Result - reassign existing var
//	guard (a, b) := expr else { ... }    // Tuple destructuring
package main

import (
	"fmt"
	"github.com/MadAppGang/dingo/pkg/dgo"
)

// --- Mock data types and functions ---

type User struct {
	ID   int
	Name string
}

type Point struct {
	X int
	Y int
}

func findUser(id int) dgo.Result[User, string] {
	if id == 1 {
		return dgo.Ok[User, string](User{1, "Alice"})
	}
	return dgo.Err[User]("user not found")
}

func getTheme(userId int) dgo.Option[string] {
	if userId == 1 {
		return dgo.Some("dark")
	}
	return dgo.None[string]()

}

func getCoords() dgo.Result[Point, string] {
	return dgo.Ok[Point, string](Point{10, 20})
}

// --- Guard with Result type (|err| binding) ---

func loadUser(id int) dgo.Result[string, string] {
	// guard with := declares new variable, |err| binds the error
	tmp := findUser(id)
	if tmp.IsErr() {
		err := *tmp.Err

		return dgo.Err[string](fmt.Sprintf("load failed: %s", err))

	}
	user := *tmp.Ok

	return dgo.Ok[string, string](user.Name)
}

// --- Guard with Option type (no binding) ---

func getUserTheme(id int) string {
	// guard with Option - no |err| because None has nothing to bind
	tmp1 := getTheme(id)
	if tmp1.IsNone() {

		return "default"

	}
	theme := *tmp1.Some

	return theme
}

// --- Guard with = (reassign existing variable) ---

func refreshUser(id int) dgo.Result[User, string] {
	var user User // existing variable

	// guard with = reassigns instead of declaring
	tmp2 := findUser(id)
	if tmp2.IsErr() {
		err := *tmp2.Err

		return dgo.Err[User](err)

	}
	user = *tmp2.Ok

	return dgo.Ok[User, string](user)
}

// --- Guard with struct unwrapping ---

func getPosition() dgo.Result[string, string] {
	tmp3 := getCoords()
	if tmp3.IsErr() {
		err := *tmp3.Err

		return dgo.Err[string](err)

	}
	pt := *tmp3.Ok

	return dgo.Ok[string, string](fmt.Sprintf("(%d, %d)", pt.X, pt.Y))
}

// --- Chained guards (the real power) ---

func getUserWithTheme(id int) dgo.Result[string, string] {
	tmp4 := findUser(id)
	if tmp4.IsErr() {
		err := *tmp4.Err

		return dgo.Err[string](err)

	}
	user := *tmp4.Ok

	tmp5 := getTheme(user.ID)
	if tmp5.IsNone() {

		return dgo.Ok[string, string](fmt.Sprintf("%s (no theme)", user.Name))

	}
	theme := *tmp5.Some

	return dgo.Ok[string, string](fmt.Sprintf("%s (%s)", user.Name, theme))
}

func main() {
	fmt.Println("=== Guard Demo ===\n")

	// Result with error binding
	fmt.Println("1. Result type (|err| binding):")
	if r := loadUser(1); r.IsOk() {
		fmt.Printf("   Found: %s\n", r.Unwrap())
	}
	if r := loadUser(99); r.IsErr() {
		fmt.Printf("   Error: %s\n", r.UnwrapErr())
	}

	// Option without binding
	fmt.Println("\n2. Option type (no binding):")
	fmt.Printf("   User 1 theme: %s\n", getUserTheme(1))
	fmt.Printf("   User 99 theme: %s\n", getUserTheme(99))

	// Reassignment with =
	fmt.Println("\n3. Reassignment (guard x =):")
	if r := refreshUser(1); r.IsOk() {
		fmt.Printf("   Refreshed: %s\n", r.Unwrap().Name)
	}

	// Struct unwrapping
	fmt.Println("\n4. Struct unwrapping:")
	if r := getPosition(); r.IsOk() {
		fmt.Printf("   Position: %s\n", r.Unwrap())
	}

	// Chained guards
	fmt.Println("\n5. Chained guards:")
	if r := getUserWithTheme(1); r.IsOk() {
		fmt.Printf("   %s\n", r.Unwrap())
	}
}
