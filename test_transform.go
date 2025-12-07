package main

import (
	"fmt"
	"github.com/MadAppGang/dingo/pkg/ast"
)

func main() {
	src := []byte(`package main

func main() {
	add := (x: int, y: int): int => x + y
}`)

	transformed, _, err := ast.TransformSource(src)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("TRANSFORMED:")
	fmt.Println(string(transformed))
}
