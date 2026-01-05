package main

// Test: Match expression with valid syntax
// This should transpile successfully (no error)

//line /Users/jack/mag/dingo/tests/lsp/07_match_missing_arm/main.dingo:6:1
type Color interface{ isColor() }

type ColorRed struct{}

func (ColorRed) isColor() {}
func NewColorRed() Color  { return ColorRed{} }

type ColorGreen struct{}

func (ColorGreen) isColor() {}
func NewColorGreen() Color  { return ColorGreen{} }

type ColorBlue struct{}

func (ColorBlue) isColor() {}
func NewColorBlue() Color  { return ColorBlue{} }

//line /Users/jack/mag/dingo/tests/lsp/07_match_missing_arm/main.dingo:11:1

func getName(c Color) string {
	//line /Users/jack/mag/dingo/tests/lsp/07_match_missing_arm/main.dingo:13:12
	val := c
	switch val.(type) {
	case ColorRed:
		return "red"
	case ColorGreen:
		return "green"
	case ColorBlue:
		return "blue"
	}
	panic("unreachable: exhaustive match")
}

func main() {}
