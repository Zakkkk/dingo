// Real-world example: HTTP handler that loads and validates user data
// This demonstrates how ? operator reduces error handling boilerplate
package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

type User struct {
	ID    string
	Name  string
	Email string
}

// GetUserHandler handles GET /users/{id} requests
// Without Dingo: 25+ lines with if err != nil checks
// With Dingo: 10 lines with clear data flow
func GetUserHandler(w http.ResponseWriter, r *http.Request) error {
	// Extract and validate user ID from path
	tmp3, err3 := extractUserID(r)
	if err3 != nil {
		return err3
	}
	userID := tmp3

	// Load user from database
	tmp2, err2 := loadUserFromDB(userID)
	if err2 != nil {
		return err2
	}
	user := tmp2

	// Check user permissions
	tmp1, err1 := checkPermissions(r, user)
	if err1 != nil {
		return err1
	}
	_ = tmp1

	// Encode response
	tmp, err := json.Marshal(user)
	if err != nil {
		return err
	}
	response := tmp

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
	return nil
}

func extractUserID(r *http.Request) (string, error) {
	id := r.PathValue("id")
	if id == "" {
		return "", errors.New("missing user ID in path")
	}
	return id, nil
}

func loadUserFromDB(id string) (*User, error) {
	// Simulated database lookup
	if id == "404" {
		return nil, errors.New("user not found")
	}
	return &User{ID: id, Name: "John Doe", Email: "john@example.com"}, nil
}

func checkPermissions(r *http.Request, user *User) (bool, error) {
	// Simulated permission check
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false, errors.New("unauthorized: missing auth header")
	}
	return true, nil
}

func main() {
	http.HandleFunc("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := GetUserHandler(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	http.ListenAndServe(":8080", nil)
}
