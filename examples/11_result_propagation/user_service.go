// Real-world example: User service with Result types and implicit wrapping
// Shows how dgo.Result[T, E] enables clean, explicit error handling
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

// Repository functions returning Result types
// Implicit wrapping: just return the value or error directly
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
	tmp, err := db.Query("SELECT id, user_id, total FROM orders WHERE user_id = ?", userID)

	if err != nil {
		return dgo.Err[[]Order](ServiceError{Code: "DB_ERROR", Message: err.Error()})
	}
	var rows = tmp
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		// Note: Scan returns only error, not (T, error), so we use explicit error handling
		err1 := rows.Scan(&order.ID, &order.UserID, &order.Total)

		if err1 != nil {
			return dgo.Err[[]Order](ServiceError{Code: "SCAN_ERROR", Message: err.Error()})
		}
		_ = err1 // error-only propagation
		orders = append(orders, order)
	}
	return dgo.Ok[[]Order, ServiceError](orders)
}

// Service function chaining multiple Result-returning functions
// Guard let with pipe binding: explicit |err| for error access
func GetUserOrderTotal(db *sql.DB, userID int) dgo.Result[float64, ServiceError] {
	// Get user - guard let unwraps or returns error
	tmp := FindUser(db, userID)

	if tmp.IsErr() {
		err := tmp.UnwrapErr()
		return dgo.Err[float64](err)
	}
	user := tmp.Unwrap()

	// Get orders - same pattern, clean and readable
	tmp1 := FindOrdersByUser(db, user.ID)

	if tmp1.IsErr() {
		err := tmp1.UnwrapErr()
		return dgo.Err[float64](err)
	}
	orders := tmp1.Unwrap()

	// Calculate total
	var total float64
	for _, order := range orders {
		total += order.Total
	}

	return dgo.Ok[float64, ServiceError](total)
}

// Another example: transferring funds between users
// Guard let makes the happy path clear and linear
func TransferFunds(db *sql.DB, fromID int, toID int, amount float64) dgo.Result[bool, ServiceError] {
	// Find source user
	tmp2 := FindUser(db, fromID)

	if tmp2.IsErr() {
		err := tmp2.UnwrapErr()
		return dgo.Err[bool](err)
	}
	fromUser := tmp2.Unwrap()

	// Find destination user
	tmp3 := FindUser(db, toID)

	if tmp3.IsErr() {
		err := tmp3.UnwrapErr()
		return dgo.Err[bool](err)
	}
	toUser := tmp3.Unwrap()

	fmt.Printf("Transferring $%.2f from %s to %s\n",
		amount, fromUser.Name, toUser.Name)

	return dgo.Ok[bool, ServiceError](true)
}
func main() {
	var db *sql.DB // Would be initialized in real code

	// Using the service with proper error handling
	// Implicit wrapping to Ok

	result := GetUserOrderTotal(db, 123)

	if result.IsOk() {
		total := result.Unwrap()
		fmt.Printf("Total orders: $%.2f\n", total)
	} else {
		err := result.UnwrapErr()
		fmt.Printf("Error [%s]: %s\n", err.Code, err.Message)
	}
}
