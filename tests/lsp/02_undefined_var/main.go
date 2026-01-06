//go:build ignore
package main

// Test: Go type error - undefined variable
// Expected: Error at line 6, pointing to 'unknownVar'

func test() int {
	return unknownVar
}

func main() {}
