package main

// Test: Safe navigation on non-pointer type
// Expected: Error - cannot use ?. on non-pointer

type User struct {
	Name string
}

func test() string {
	var u User
	//line /Users/jack/mag/dingo/tests/lsp/08_safe_nav_error/main.dingo:12:12
	tmp := u
	if tmp == nil {
		return nil
	}
	return tmp.Name
}

func main() {}
