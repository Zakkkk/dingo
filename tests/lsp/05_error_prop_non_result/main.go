package main

// Test: Error propagation on non-Result type
// Expected: Error - cannot use ? on int

func getInt() int {
	return 42
}

func test() int {
	tmp, err := getInt()
	if err != nil {
		return err
	}
	x := tmp
	return x
}

func main() {}
