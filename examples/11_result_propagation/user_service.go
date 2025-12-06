// Real-world example: User service with Result types and implicit wrapping
// Shows how Result<T, E> enables clean, explicit error handling
package main

import (
	"database/sql"
	"fmt"
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
func FindUser(db *sql.DB, id int) Result[User, ServiceError] {
	row := db.QueryRow("SELECT id, name, email FROM users WHERE id = ?", id)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email)
	if err == sql.ErrNoRows {
		return ServiceError{Code: "NOT_FOUND", Message: "user not found"}
	}
	if err != nil {
		return ServiceError{Code: "DB_ERROR", Message: err.Error()}
	}

	return user
}

func FindOrdersByUser(db *sql.DB, userID int) Result[[]Order, ServiceError] {
	rows := db.Query("SELECT id, user_id, total FROM orders WHERE user_id = ?", userID) /*DINGO_ERR_PROP*/
	ServiceError{Code: "DB_ERROR", Message: err.Error()}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		// Note: Scan returns only error, not (T, error), so we use explicit error handling
		rows.Scan(&order.ID, &order.UserID, &order.Total) /*DINGO_ERR_PROP*/
		ServiceError{Code: "SCAN_ERROR", Message: err.Error()}
		orders = append(orders, order)
	}
	return orders
}

// Service function chaining multiple Result-returning functions
// Guard let with pipe binding: explicit func(err) { return for error access }
func GetUserOrderTotal(db *sql.DB, userID int) Result[float64, ServiceError] {
	// Get user - guard let unwraps or returns error
	user, err := FindUser(db, userID)
	if err != nil {
		return Err(err)
	}

	// Get orders - same pattern, clean and readable
	orders, err := FindOrdersByUser(db, user.ID)
	if err != nil {
		return Err(err)
	}

	// Calculate total
	var total float64
	for _, order := range orders {
		total += order.Total
	}

	return total // Implicit wrapping to Ok
}

// Another example: transferring funds between users
// Guard let makes the happy path clear and linear
func TransferFunds(db *sql.DB, fromID int, toID int, amount float64) Result[bool, ServiceError] {
	// Find source user
	fromUser, err := FindUser(db, fromID)
	if err != nil {
		return Err(err)
	}

	// Find destination user
	toUser, err := FindUser(db, toID)
	if err != nil {
		return Err(err)
	}

	fmt.Printf("Transferring $%.2f from %s to %s\n",
		amount, fromUser.Name, toUser.Name)

	return true
}

func main() {
	var db *sql.DB // Would be initialized in real code

	// Using the service with proper error handling
	result := GetUserOrderTotal(db, 123)

	if result.IsOk() {
		total := result.Unwrap()
		fmt.Printf("Total orders: $%.2f\n", total)
	} else {
		err := result.UnwrapErr()
		fmt.Printf("Error [%s]: %s\n", err.Code, err.Message)
	}
}
