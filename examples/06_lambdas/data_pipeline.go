// Generated Go code from data_pipeline.dingo
// Lambdas become standard Go anonymous functions
package main

import (
	"fmt"
	"strings"
)

type User struct {
	ID      int
	Name    string
	Email   string
	Age     int
	Active  bool
	Premium bool
}

// ProcessUsers demonstrates a data pipeline with lambdas
func ProcessUsers(users []User) {
	// Filter active premium users over 18
	eligible := Filter(users, func(u User) bool {
		return u.Active && u.Premium && u.Age >= 18
	})

	// Transform to display format
	names := Map(eligible, func(u User) string {
		return fmt.Sprintf("%s <%s>", u.Name, u.Email)
	})

	// Multi-line lambda for complex logic
	summary := Reduce(eligible, "", func(acc string, u User) string {
		if acc == "" {
			return u.Name
		}
		return acc + ", " + u.Name
	})

	fmt.Println("Eligible users:")
	for _, name := range names {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Printf("Summary: %s\n", summary)
}

// Higher-order functions that accept lambdas
func Filter[T any](items []T, predicate func(T) bool) []T {
	var result []T
	for _, item := range items {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

func Map[T, R any](items []T, transform func(T) R) []R {
	result := make([]R, len(items))
	for i, item := range items {
		result[i] = transform(item)
	}
	return result
}

func Reduce[T, R any](items []T, initial R, reducer func(R, T) R) R {
	result := initial
	for _, item := range items {
		result = reducer(result, item)
	}
	return result
}

// SortUsers sorts with custom comparator
func SortUsers(users []User, compare func(User, User) bool) []User {
	// Copy to avoid mutating original
	sorted := make([]User, len(users))
	copy(sorted, users)

	// Simple bubble sort for demo
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if compare(sorted[j+1], sorted[j]) {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	return sorted
}

func main() {
	users := []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com", Age: 30, Active: true, Premium: true},
		{ID: 2, Name: "Bob", Email: "bob@example.com", Age: 17, Active: true, Premium: true},
		{ID: 3, Name: "Charlie", Email: "charlie@example.com", Age: 25, Active: false, Premium: true},
		{ID: 4, Name: "Diana", Email: "diana@example.com", Age: 28, Active: true, Premium: false},
		{ID: 5, Name: "Eve", Email: "eve@example.com", Age: 35, Active: true, Premium: true},
	}

	ProcessUsers(users)

	// Sort by age (ascending) using lambda
	byAge := SortUsers(users, func(a, b User) bool {
		return a.Age < b.Age
	})

	fmt.Println("\nUsers by age:")
	for _, u := range byAge {
		fmt.Printf("  %s (%d)\n", u.Name, u.Age)
	}

	// Sort by name using lambda
	byName := SortUsers(users, func(a, b User) bool {
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	fmt.Println("\nUsers by name:")
	for _, u := range byName {
		fmt.Printf("  %s\n", u.Name)
	}
}
