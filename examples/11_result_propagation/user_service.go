// Real-world example: User service with Result types and implicit wrapping
// Shows how Result[T, E] enables clean, explicit error handling
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
		return ResultSliceOrderServiceErrorErr(ServiceError{Code: "DB_ERROR", Message: err.Error()})
	}
	var rows = tmp
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		// Note: Scan returns only error, not (T, error), so we use explicit error handling
		err1 := rows.Scan(&order.ID, &order.UserID, &order.Total)

		if err1 != nil {
			return ResultSliceOrderServiceErrorErr(ServiceError{Code: "SCAN_ERROR", Message: err.Error()})
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
		err := *tmp.err
		return dgo.Err[float64](err)
	}
	user := *tmp.ok

	// Get orders - same pattern, clean and readable
	// ERROR: guard let could not determine type for: FindOrdersByUser(db, user.ID)
	// Original: guard let orders = FindOrdersByUser(db, user.ID) else { return Err(err) }
	// Calculate total
	var total float64
	for _, order := range orders {
		total += order.Total
	}

	return dgo.Ok[float64, // Implicit wrapping to Ok
	ServiceError](total)
}

// Another example: transferring funds between users
// Guard let makes the happy path clear and linear
func TransferFunds(db *sql.DB, fromID int, toID int, amount float64) dgo.Result[bool, ServiceError] {
	// Find source user
	tmp1 := FindUser(db, fromID)

	if tmp1.IsErr() {
		err := *tmp1.err
		return dgo.Err[bool](err)
	}
	fromUser := *tmp1.ok

	// Find destination user
	tmp2 := FindUser(db, toID)

	if tmp2.IsErr() {
		err := *tmp2.err
		return dgo.Err[bool](err)
	}
	toUser := *tmp2.ok

	fmt.Printf("Transferring $%.2f from %s to %s\n",
		amount, fromUser.Name, toUser.Name)

	return dgo.Ok[bool, ServiceError](true)
}
func main() {
	var db *sql.DB // Would be initialized in real code

	// Using the service with proper error handling
	result := GetUserOrderTotal(db, 123)

	if result.IsOk() {
		total := result.MustOk()
		fmt.Printf("Total orders: $%.2f\n", total)
	} else {
		err := result.MustErr()
		fmt.Printf("Error [%s]: %s\n", err.Code, err.Message)
	}
}
