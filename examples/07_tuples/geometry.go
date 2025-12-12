// Real-world example: Geometry calculations with tuples
// Tuples are perfect for grouping related values without creating a struct
//
// === Design Decision: Tuples via Runtime Package ===
//
// Dingo tuples use Go generics via the runtime/tuples package:
//
//	(A, B)       → tuples.T2[A, B]
//	(A, B, C)    → tuples.T3[A, B, C]
//	(1, "hello") → tuples.New2(1, "hello")
//
// Tuple types support up to 10 elements with positional access via .V0, .V1, etc.
package main

import (
	"fmt"
	"github.com/MadAppGang/dingo/runtime/tuples"
	"math"
)

// Point2D as a tuple - no need for a full struct definition
type Point2D = tuples.Tuple2[float64, float64]

// Bounding box as nested tuples: ((minX, minY), (maxX, maxY))
type BoundingBox = tuples.Tuple2[Point2D, Point2D]

// ParseCoordinates returns multiple related values
// Without tuples, you'd need a struct or multiple returns
func ParseCoordinates(input string) (Point2D, error) {
	var x, y float64
	_, err := fmt.Sscanf(input, "(%f,%f)", &x, &y)
	if err != nil {
		return tuples.Tuple2[float64, float64]{First: 0.0, Second: 0.0}, err
	}
	return tuples.Tuple2[float64, float64]{First: x, Second: y}, nil
}

// Distance calculates distance between two points
func Distance(p1 Point2D, p2 Point2D) float64 {
	// Tuple destructuring
	tpl := p1
	x1 := tpl.First
	y1 := tpl.Second
	tpl1 := p2
	x2 := tpl1.First
	y2 := tpl1.Second

	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// Midpoint returns the point between two points
func Midpoint(p1 Point2D, p2 Point2D) Point2D {
	tpl2 := p1
	x1 := tpl2.First
	y1 := tpl2.Second
	tpl3 := p2
	x2 := tpl3.First
	y2 := tpl3.Second
	return tuples.Tuple2[float64, float64]{First: (x1 + x2) / 2, Second: (y1 + y2) / 2}
}

// CalculateBoundingBox finds the bounding box for a set of points
func CalculateBoundingBox(points []Point2D) BoundingBox {
	if len(points) == 0 {
		return tuples.Tuple2[tuples.Tuple2[float64, float64], tuples.Tuple2[float64, float64]]{First: tuples.Tuple2[float64, float64]{First: 0.0, Second: 0.0}, Second: tuples.Tuple2[float64, float64]{First: 0.0, Second: 0.0}}
	}

	tpl4 := points[0]
	minX := tpl4.First
	minY := tpl4.Second
	tpl5 := points[0]
	maxX := tpl5.First
	maxY := tpl5.Second

	for _, point := range points {
		tpl6 := point
		x := tpl6.First
		y := tpl6.Second
		minX = min(minX, x)
		maxX = max(maxX, x)
		minY = min(minY, y)
		maxY = max(maxY, y)
	}

	return tuples.Tuple2[tuples.Tuple2[float64, float64], tuples.Tuple2[float64, float64]]{First: tuples.Tuple2[float64, float64]{First: minX, Second: minY}, Second: tuples.Tuple2[float64, float64]{First: maxX, Second: maxY}}
}

// TransformPoints applies a transformation to all points
func TransformPoints(points []Point2D, transform func(Point2D) Point2D) []Point2D {
	result := make([]Point2D, len(points))
	for i, p := range points {
		result[i] = transform(p)
	}
	return result
}

func main() {
	// Create points using tuple syntax
	points := []Point2D{
		tuples.Tuple2[float64, float64]{First: 0.0, Second: 0.0},
		tuples.Tuple2[float64, float64]{First: 3.0, Second: 4.0},
		tuples.Tuple2[float64, float64]{First: 6.0, Second: 0.0},
		tuples.Tuple2[float64, float64]{First: 3.0, Second: -2.0},
	}

	fmt.Println("=== Points ===")
	for i, p := range points {
		tpl7 := p
		x := tpl7.First
		y := tpl7.Second
		fmt.Printf("P%d: (%.1f, %.1f)\n", i, x, y)
	}

	// Calculate distances
	d := Distance(points[0], points[1])
	fmt.Printf("\nDistance P0 to P1: %.2f\n", d)

	// Find midpoint
	mid := Midpoint(points[0], points[2])
	tpl8 := mid
	mx := tpl8.First
	my := tpl8.Second
	fmt.Printf("Midpoint P0-P2: (%.1f, %.1f)\n", mx, my)

	// Calculate bounding box
	bbox := CalculateBoundingBox(points)
	// Note: nested tuple destructuring ((a, b), (c, d)) not yet supported
	// Using two-step destructuring instead
	tpl9 := bbox
	minPt := tpl9.First
	maxPt := tpl9.Second
	tpl10 := minPt
	minX := tpl10.First
	minY := tpl10.Second
	tpl11 := maxPt
	maxX := tpl11.First
	maxY := tpl11.Second
	fmt.Printf("\nBounding Box: (%.1f, %.1f) to (%.1f, %.1f)\n", minX, minY, maxX, maxY)

	// Transform all points (scale by 2)
	scaled := TransformPoints(points, func(p Point2D) Point2D {
		tpl12 := p
		x := tpl12.First
		y := tpl12.Second
		return tuples.Tuple2[float64, float64]{First: x * 2, Second: y * 2}
	})

	fmt.Println("\n=== Scaled Points (2x) ===")
	for i, p := range scaled {
		tpl13 := p
		x := tpl13.First
		y := tpl13.Second
		fmt.Printf("P%d: (%.1f, %.1f)\n", i, x, y)
	}
}
