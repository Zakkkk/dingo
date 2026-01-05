package main

// Test: Lambda syntax error - missing type after colon
// Expected: Error "expected type after ':'"

func test() {
	fn := func(x int) int { return x + 1 }
	_ = fn
}

func main() {}
