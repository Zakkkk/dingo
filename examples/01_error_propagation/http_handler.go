// Real-world example: HTTP handler that loads and validates user data
// This demonstrates all error propagation patterns in Dingo:
//  1. Basic:   expr?              - propagate error as-is
//  2. Context: expr ? "message"   - wrap with fmt.Errorf
//  3. Lambda:  expr ? |e| f(e)    - custom transform (Rust style)
//  4. Lambda:  expr ? e => f(e)   - custom transform (TypeScript style)
//
// === Design Decision: Go's (T, error) Pattern ===
//
// For error propagation with Go functions, Dingo preserves Go's (T, error)
// pattern and adds the ? operator for concise error handling. This works
// seamlessly with all Go standard library functions.
//
// For explicit Result types, see examples/02_result which uses
// dgo.Result[T, E] via Go 1.18+ generics.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Custom error type for application-level errors
type AppError struct {
	Code    int
	Message string
	Cause   error
}

func (e AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func NewAppError(code int, msg string, cause error) error {
	return AppError{Code: code, Message: msg, Cause: cause}
}

type User struct {
	ID    string
	Name  string
	Email string
}

// GetUserHandler handles GET /users/{id} requests
// Demonstrates all error propagation patterns
func GetUserHandler(w http.ResponseWriter, r *http.Request) error {
	// Pattern 1: Basic ? - propagate error as-is
	// Use when the error is already descriptive enough
	//line /Users/jack/mag/dingo/examples/01_error_propagation/http_handler.dingo:55:2
	tmp, err := extractUserID(r)
	if err != nil {
		return err
	}
	userID := tmp

	// Pattern 2: String context ? "message" - add context with fmt.Errorf
	// Use for simple context wrapping without custom error types
	//line /Users/jack/mag/dingo/examples/01_error_propagation/http_handler.dingo:59:2
	tmp1, err1 := loadUserFromDB(userID)
	if err1 != nil {
		return fmt.Errorf("database lookup failed: %w", err1)
	}
	user := tmp1

	// Pattern 3: Rust-style lambda ? |err| transform(err)
	// Use for custom error types or complex transformations
	//line /Users/jack/mag/dingo/examples/01_error_propagation/http_handler.dingo:63:2
	tmp2, err2 := checkPermissions(r, user)
	if err2 != nil {
		return NewAppError(403, "permission denied", err2)
	}
	_ = tmp2

	// Pattern 4: TypeScript-style lambda ? (e) => transform(e)
	// Same as Pattern 3, just different syntax preference
	//line /Users/jack/mag/dingo/examples/01_error_propagation/http_handler.dingo:67:2
	tmp3, err3 := json.Marshal(user)
	if err3 != nil {
		return NewAppError(500, "serialization error", err3)
	}
	response := tmp3

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
	return nil
}

// AdvancedHandler demonstrates more complex error handling scenarios
func AdvancedHandler(userID string, orderID string) (Order, error) {
	// Pattern 1: Basic - error is already descriptive
	//line /Users/jack/mag/dingo/examples/01_error_propagation/http_handler.dingo:77:2
	tmp4, err4 := getUser(userID)
	if err4 != nil {
		return Order{}, err4
	}
	user := tmp4

	// Pattern 2: String context - simple message wrapping
	//line /Users/jack/mag/dingo/examples/01_error_propagation/http_handler.dingo:80:2
	tmp5, err5 := getOrder(orderID)
	if err5 != nil {
		return Order{}, fmt.Errorf("failed to fetch order: %w", err5)
	}
	order := tmp5

	// Pattern 3: Rust lambda with captured variables
	//line /Users/jack/mag/dingo/examples/01_error_propagation/http_handler.dingo:83:2
	tmp6, err6 := validateOrder(order, user)
	if err6 != nil {
		return Order{}, fmt.Errorf("validation failed for user %s: %w", userID, err6)
	}
	validated := tmp6

	// Pattern 4: TypeScript lambda (single param, no parens)
	//line /Users/jack/mag/dingo/examples/01_error_propagation/http_handler.dingo:86:2
	tmp7, err7 := processOrder(validated)
	if err7 != nil {
		return Order{}, fmt.Errorf("processing error: %w", err7)
	}
	processed := tmp7

	return processed, nil
}

// Helper types and functions

type Order struct {
	ID     string
	UserID string
	Amount float64
}

func extractUserID(r *http.Request) (string, error) {
	id := r.PathValue("id")
	if id == "" {
		return "", errors.New("missing user ID in path")
	}
	return id, nil
}

func loadUserFromDB(id string) (*User, error) {
	if id == "404" {
		return nil, errors.New("user not found")
	}
	return &User{ID: id, Name: "John Doe", Email: "john@example.com"}, nil
}

func checkPermissions(r *http.Request, user *User) (bool, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false, errors.New("missing auth header")
	}
	return true, nil
}

func getUser(id string) (User, error) {
	return User{ID: id, Name: "Test User"}, nil
}

func getOrder(id string) (Order, error) {
	return Order{ID: id, Amount: 99.99}, nil
}

func validateOrder(order Order, user User) (Order, error) {
	order.UserID = user.ID
	return order, nil
}

func processOrder(order Order) (Order, error) {
	return order, nil
}

func main() {
	http.HandleFunc("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := GetUserHandler(w, r); err != nil {
			// Handle AppError specially for proper status codes
			if appErr, ok := err.(AppError); ok {
				http.Error(w, appErr.Error(), appErr.Code)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	http.ListenAndServe(":8080", nil)
}
