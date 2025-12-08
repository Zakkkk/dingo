// Real-world example: Geometry calculations with tuples
// Tuples are perfect for grouping related values without creating a struct
package main

import (
	"github.com/MadAppGang/dingo/runtime/tuples"
	"fmt"
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
	tmp := p1
	x1 := tmp.First
	y1 := tmp.Second
	tmp1 := p2
	x2 := tmp1.First
	y2 := tmp1.Second

	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// Midpoint returns the point between two points
func Midpoint(p1 Point2D, p2 Point2D) Point2D {
	tmp2 := p1
	x1 := tmp2.First
	y1 := tmp2.Second
	tmp3 := p2
	x2 := tmp3.First
	y2 := tmp3.Second
	return tuples.Tuple2[float64, float64]{First: (x1 + x2) / 2, Second: (y1 + y2) / 2}
}

// CalculateBoundingBox finds the bounding box for a set of points
func CalculateBoundingBox(points []Point2D) BoundingBox {
	if len(points) == 0 {
		return tuples.Tuple2[tuples.Tuple2[float64, float64], tuples.Tuple2[float64, float64]]{First: tuples.Tuple2[float64, float64]{First: 0.0, Second: 0.0}, Second: tuples.Tuple2[float64, float64]{First: 0.0, Second: 0.0}}
	}

	tmp4 := points[0]
	minX := tmp4.First
	minY := tmp4.Second
	tmp5 := points[0]
	maxX := tmp5.First
	maxY := tmp5.Second

	for _, point := range points {
		tmp6 := point
		x := tmp6.First
		y := tmp6.Second
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
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
		tmp7 := p
		x := tmp7.First
		y := tmp7.Second
		fmt.Printf("P%d: (%.1f, %.1f)\n", i, x, y)
	}

	// Calculate distances
	d := Distance(points[0], points[1])
	fmt.Printf("\nDistance P0 to P1: %.2f\n", d)

	// Find midpoint
	mid := Midpoint(points[0], points[2])
	tmp8 := mid
	mx := tmp8.First
	my := tmp8.Second
	fmt.Printf("Midpoint P0-P2: (%.1f, %.1f)\n", mx, my)

	// Calculate bounding box
	bbox := CalculateBoundingBox(points)
	tmp9 := bbox
	minX := tmp9.First.First
	minY := tmp9.First.Second
	maxX := tmp9.Second.First
	maxY := tmp9.Second.Second
	fmt.Printf("\nBounding Box: (%.1f, %.1f) to (%.1f, %.1f)\n", minX, minY, maxX, maxY)

	// Transform all points (scale by 2)
	scaled := TransformPoints(points, func(p Point2D) Point2D {
		tmp10 := p
		x := tmp10.First
		y := tmp10.Second
		return tuples.Tuple2[float64, float64]{First: x * 2, Second: y * 2}
	})

	fmt.Println("\n=== Scaled Points (2x) ===")
	for i, p := range scaled {
		tmp11 := p
		x := tmp11.First
		y := tmp11.Second
		fmt.Printf("P%d: (%.1f, %.1f)\n", i, x, y)
	}
}
