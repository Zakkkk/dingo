// Real-world example: Database repository with Result types
// Result<T, E> makes success/failure states explicit and type-safe
package main

import (
	"database/sql"
	"fmt"
	"github.com/MadAppGang/dingo/pkg/dgo"
)

type User struct {
	ID    int
	Name  string
	Email string
}

type DBError struct {
	Code    string
	Message string
}

func (e DBError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// FindUserByID returns a Result that explicitly models success or failure
// Caller must handle both cases - no silent nil pointer bugs
func FindUserByID(db *sql.DB, id int) dgo.Result[User, DBError] {
	row := db.QueryRow("SELECT id, name, email FROM users WHERE id = ?", id)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email)
	if err == sql.ErrNoRows {
		return dgo.Err[User](DBError{Code: "NOT_FOUND", Message: "user not found"})
	}
	if err != nil {
		return dgo.Err[User](DBError{Code: "SCAN_ERROR", Message: err.Error()})
	}

	return dgo.Ok[User, DBError](user)
}

// TransferFunds shows Result chaining - each step must succeed
func TransferFunds(db *sql.DB, fromID int, toID int, amount float64) dgo.Result[bool, DBError] {
	// Find source user - check for errors
	fromResult := FindUserByID(db, fromID)
	if fromResult.IsErr() {
		return dgo.Err[bool](fromResult.MustErr()) // Implicit wrapping → ResultBoolDBErrorErr(...)
	}

	// Find destination user
	toResult := FindUserByID(db, toID)
	if toResult.IsErr() {
		return dgo.Err[bool](toResult.MustErr())
	}

	// In real code: begin transaction, update balances, commit
	fmt.Printf("Transferring $%.2f from %s to %s\n",
		amount, fromResult.MustOk().Name, toResult.MustOk().Name)

	return dgo.Ok[bool, DBError](true)
}

func main() {
	// Example usage showing explicit error handling
	var db *sql.DB // Would be initialized in real code

	result := FindUserByID(db, 123)

	// Pattern: Must explicitly check and handle both cases
	// Alternative approaches:
	//   result.MustOk()    - Go style, panics if Err (recommended)
	//   result.Unwrap()    - Rust style alias for MustOk() (deprecated)
	//   result.UnwrapOr(defaultUser) - Returns default if Err
	//   result.Expect("msg")         - Panics with custom message
	if result.IsOk() {
		user := result.MustOk()
		fmt.Printf("Found user: %s <%s>\n", user.Name, user.Email)
	} else {
		err := result.MustErr()
		fmt.Printf("Error: %s\n", err.Message)
	}
}
