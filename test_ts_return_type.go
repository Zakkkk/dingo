package main

import "fmt"

func main() {
	// TypeScript lambda with return type annotation
	add = func(x int, y int) int { return x + y }

	// Without return type (existing behavior)
	multiply = func(x int, y int) { return x * y }

	fmt.Println(add(2, 3))
	fmt.Println(multiply(4, 5))
}
