// Simple Sum Types Example
// Demonstrates interface-based sum types with Go's idiomatic type switch
//
// === Design Decision: Enum to Interface Pattern ===
//
// Dingo enums compile to Go interface + struct patterns.
// Each variant becomes a struct implementing the interface.
// Variants with data have their data as struct fields.
// This enables Go's type switch for exhaustive pattern matching.
package main

import "fmt"

// Status enum - simple variants without data
type Status interface{ isStatus() }

type StatusPending struct{}

func (StatusPending) isStatus() {}
func NewStatusPending() Status  { return StatusPending{} }

type StatusActive struct{}

func (StatusActive) isStatus() {}
func NewStatusActive() Status  { return StatusActive{} }

type StatusComplete struct{}

func (StatusComplete) isStatus() {}
func NewStatusComplete() Status  { return StatusComplete{} }

// Shape enum - variants with data (struct fields)
type Shape interface{ isShape() }

type ShapePoint struct{}

func (ShapePoint) isShape() {}
func NewShapePoint() Shape  { return ShapePoint{} }

type ShapeCircle struct{ radius float64 }

func (ShapeCircle) isShape()              {}
func NewShapeCircle(radius float64) Shape { return ShapeCircle{radius: radius} }

type ShapeRectangle struct {
	width  float64
	height float64
}

func (ShapeRectangle) isShape() {}
func NewShapeRectangle(width float64, height float64) Shape {
	return ShapeRectangle{width: width, height: height}
}

// describeStatus shows pattern matching on simple enum
func describeStatus(s Status) string {
	val1 := s
	switch val1.(type) {
	case StatusPending:
		return "Waiting to start"
	case StatusActive:
		return "Currently running"
	case StatusComplete:
		return "All done!"
	}
	panic("unreachable: exhaustive match")
}

// describeShape shows pattern matching with data extraction
func describeShape(s Shape) string {
	val := s
	switch val.(type) {
	case ShapePoint:
		return "A point (no dimensions)"
	case ShapeCircle:
		return "A circle"
	case ShapeRectangle:
		return "A rectangle"
	}
	panic("unreachable: exhaustive match")
}

func main() {
	// Create enum variants using NewVariant() constructors
	pending := NewStatusPending()
	active := NewStatusActive()
	complete := NewStatusComplete()

	fmt.Println("=== Status Enum ===")
	fmt.Println("Pending:", describeStatus(pending))
	fmt.Println("Active:", describeStatus(active))
	fmt.Println("Complete:", describeStatus(complete))

	// Create shape variants
	point := NewShapePoint()
	circle := NewShapeCircle(5.0)
	rect := NewShapeRectangle(10.0, 20.0)

	fmt.Println("\n=== Shape Enum ===")
	fmt.Println("Point:", describeShape(point))
	fmt.Println("Circle:", describeShape(circle))
	fmt.Println("Rectangle:", describeShape(rect))

	// Idiomatic Go: Type assertion for single-case checks
	// This is the Go way - no Is*() methods needed
	fmt.Println("\n=== Type Assertions (Idiomatic Go) ===")

	if _, ok := pending.(StatusPending); ok {
		fmt.Println("pending is StatusPending ✓")
	}

	if _, ok := active.(StatusActive); ok {
		fmt.Println("active is StatusActive ✓")
	}

	// Type switch for multiple cases
	fmt.Println("\n=== Type Switch (Idiomatic Go) ===")
	shapes := []Shape{point, circle, rect}
	for _, shape := range shapes {
		switch v := shape.(type) {
		case ShapePoint:
			fmt.Println("Found a point")
		case ShapeCircle:
			fmt.Printf("Found a circle with radius %.1f\n", v.radius)
		case ShapeRectangle:
			fmt.Printf("Found a %.1f x %.1f rectangle\n", v.width, v.height)
		}
	}
}
