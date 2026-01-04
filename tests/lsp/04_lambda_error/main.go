package main

// Test: Lambda syntax error - missing arrow
// Expected: Error pointing to malformed lambda

func test() {
	fn := func(x int) int { return x + 1 }
	_ = fn
}

func main() {}
