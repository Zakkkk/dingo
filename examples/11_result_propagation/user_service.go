// Result Propagation Example: Three patterns for error handling with ?
//
// Pattern 1: expr?              - propagate error as-is
// Pattern 2: expr ? "message"   - wrap with string context
// Pattern 3: expr ? |e| f(e)    - transform with explicit lambda binding
//
// === Design Decision: Generic Types via dgo Package ===
//
// Dingo uses Go 1.18+ generics for Result types via the dgo runtime:
//
//	Result[T, E] → dgo.Result[T, E]  (single generic struct)
//	Ok(value)    → dgo.Ok[T, E](value)
//	Err(err)     → dgo.Err[T, E](err)
//
// Why generics instead of code generation?
// 1. No code bloat - one generic type serves all uses
// 2. Better IDE support - gopls understands dgo.Result[T, E] directly
// 3. Cleaner output - generated .go files are minimal
package main

import (
	"database/sql"
	"fmt"
	"github.com/MadAppGang/dingo/pkg/dgo"
)

// Domain types
type User struct {
	ID    int
	Name  string
	Email string
}

type Order struct {
	ID     int
	UserID int
	Total  float64
}

// Custom error type with error codes
type ServiceError struct {
	Code    string
	Message string
}

func (e ServiceError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// === Pattern 1: Basic ? - propagate error as-is ===
// Use when error is already descriptive enough
// Works with functions returning (T, error)

func GetUserBasic(db *sql.DB, id int) (User, error) {
	// Simple propagation - error passes through unchanged
	row := db.QueryRow("SELECT id, name, email FROM users WHERE id = ?", id)

	var user User
	// Basic ? just propagates the error as-is
	// Note: Scan returns only error, so use explicit error handling
	err := row.Scan(&user.ID, &user.Name, &user.Email)
	if err != nil {
		return User{}, err
	}

	return user, nil
}

// === Pattern 2: String context ? "message" ===
// Use for simple context wrapping without custom error types
// Generates: fmt.Errorf("message: %w", err)

func GetUserWithContext(db *sql.DB, id int) (User, error) {
	row := db.QueryRow("SELECT id, name, email FROM users WHERE id = ?", id)

	var user User
	// String context adds a descriptive wrapper
	// Note: Scan returns only error, so use explicit error handling
	err := row.Scan(&user.ID, &user.Name, &user.Email)
	if err != nil {
		return User{}, fmt.Errorf("failed to scan user row: %w", err)
	}

	return user, nil
}

// === Pattern 3: Lambda ? |err| transform(err) ===
// Use for custom error types or complex transformations
// Explicit |err| binding - no magic implicit variables

func FindUser(db *sql.DB, id int) dgo.Result[User, ServiceError] {
	row := db.QueryRow("SELECT id, name, email FROM users WHERE id = ?", id)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email)
	if err == sql.ErrNoRows {
		return dgo.Err[User](ServiceError{Code: "NOT_FOUND", Message: "user not found"})
	}
	if err != nil {
		return dgo.Err[User](ServiceError{Code: "DB_ERROR", Message: err.Error()})
	}

	return dgo.Ok[User, ServiceError](user)
}

func FindOrdersByUser(db *sql.DB, userID int) dgo.Result[[]Order, ServiceError] {
	// Pattern 3: Explicit |err| lambda binding
	// Note: db.Query returns (*sql.Rows, error) - supports ?
	tmp, err := db.Query("SELECT id, user_id, total FROM orders WHERE user_id = ?", userID)
	if err != nil {
		return dgo.Err[[]Order](ServiceError{Code: "DB_ERROR", Message: err.Error()})
	}
	rows := tmp
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		// Note: Scan returns only error, so use explicit error handling
		err := rows.Scan(&order.ID, &order.UserID, &order.Total)
		if err != nil {
			return dgo.Err[[]Order](ServiceError{Code: "SCAN_ERROR", Message: err.Error()})
		}
		orders = append(orders, order)
	}
	return dgo.Ok[[]Order, ServiceError](orders)
}

// === Combining patterns with guard ===
// guard uses explicit |err| binding for consistency

func GetUserOrderTotal(db *sql.DB, userID int) dgo.Result[float64, ServiceError] {
	// guard unwraps Result or returns error via explicit |err|
	// Note: Err[float64] explicit type parameter needed for type inference
	tmp := FindUser(db, userID)
	if tmp.IsErr() {
		err := *tmp.Err
		return dgo.Err[float64](err)
	}
	user := *tmp.Ok

	tmp1 := FindOrdersByUser(db, user.ID)
	if tmp1.IsErr() {
		err := *tmp1.Err
		return dgo.Err[float64](err)
	}
	orders := *tmp1.Ok

	var total float64
	for _, order := range orders {
		total += order.Total
	}

	return dgo.Ok[float64, ServiceError](total) // Implicit wrapping to Ok
}

// === All three patterns in one function ===
// Demonstrates mixing patterns based on context

func ProcessUserOrder(db *sql.DB, userID int, orderID int) (string, error) {
	// Pattern 1: Basic ? - error is descriptive enough
	tmp1, err1 := GetUserBasic(db, userID)
	if err1 != nil {
		return "", err1
	}
	user := tmp1

	// Pattern 2: String context - add simple context
	tmp2, err2 := getOrderByID(db, orderID)
	if err2 != nil {
		return "", fmt.Errorf("order lookup failed: %w", err2)
	}
	order := tmp2

	// Pattern 3: Lambda - custom error transformation
	tmp3, err3 := validateOrder(order, user)
	if err3 != nil {
		return "", fmt.Errorf("validation failed for user %d: %w", userID, err3)
	}
	validated := tmp3

	return fmt.Sprintf("Processed order %d for %s", validated.ID, user.Name), nil
}

// Helper functions
func getOrderByID(db *sql.DB, id int) (Order, error) {
	return Order{ID: id, Total: 99.99}, nil
}

func validateOrder(order Order, user User) (Order, error) {
	order.UserID = user.ID
	return order, nil
}

func main() {
	var db *sql.DB // Would be initialized in real code

	// Using Result type with guard
	result := GetUserOrderTotal(db, 123)

	if result.IsOk() {
		total := result.MustOk()
		fmt.Printf("Total orders: $%.2f\n", total)
	} else {
		err := result.MustErr()
		fmt.Printf("Error [%s]: %s\n", err.Code, err.Message)
	}
}
