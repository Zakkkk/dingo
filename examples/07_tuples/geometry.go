// Real-world example: Geometry calculations with tuples
// Tuples are perfect for grouping related values without creating a struct
package main

import (
	"fmt"
	"math"
)

type Tuple2Float64Float64 struct {
	_0 float64
	_1 float64
}
type Tuple2AnyAny struct {
	_0 struct {
		_0 float64
		_1 float64
	}
	_1 struct {
		_0 float64
		_1 float64
	}
}

// Point2D as a tuple - no need for a full struct definition
type Point2D = struct {
	_0 float64
	_1 float64
}

// Bounding box as nested tuples: ((minX, minY), (maxX, maxY))
type BoundingBox = struct {
	_0 Point2D
	_1 Point2D
}

// ParseCoordinates returns multiple related values
// Without tuples, you'd need a struct or multiple returns
func ParseCoordinates(input string) (struct {
	_0 float64
	_1 float64
}, error) {
	var x, y float64
	_, err := fmt.Sscanf(input, "(%f,%f)", &x, &y)
	if err != nil {
		return Tuple2Float64Float64{_0: 0.0, _1: 0.0}, err
	}
	return Tuple2Float64Float64{_0: x, _1: y}, nil
}

// Distance calculates distance between two points
func Distance(p1 Point2D, p2 Point2D) float64 {
	tmp :=
		// Tuple destructuring
		p1
	x1 := tmp._0
	y1 := tmp._1
	tmp1 := p2
	x2 := tmp1._0
	y2 := tmp1._1

	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// Midpoint returns the point between two points
func Midpoint(p1 Point2D, p2 Point2D) Point2D {
	tmp2 := p1
	x1 := tmp2._0
	y1 := tmp2._1
	tmp3 := p2
	x2 := tmp3._0
	y2 := tmp3._1
	return Tuple2Float64Float64{_0: (x1 + x2) / 2, _1: (y1 + y2) / 2}
}

// CalculateBoundingBox finds the bounding box for a set of points
func CalculateBoundingBox(points []Point2D) BoundingBox {
	if len(points) == 0 {
		return Tuple2AnyAny{_0: Tuple2Float64Float64{_0: 0.0, _1: 0.0}, _1: Tuple2Float64Float64{_0: 0.0, _1: 0.0}}
	}
	tmp4 := points[0]
	minX := tmp4._0
	minY := tmp4._1
	tmp5 := points[0]
	maxX := tmp5._0
	maxY := tmp5._1

	for _, point := range points {
		tmp6 := point
		x := tmp6._0
		y := tmp6._1
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

	return Tuple2AnyAny{_0: Tuple2Float64Float64{_0: minX, _1: minY}, _1: Tuple2Float64Float64{_0: maxX, _1: maxY}}
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
	points := []Point2D{Tuple2Float64Float64{_0: 0.0, _1: 0.0}, Tuple2Float64Float64{_0: 3.0, _1: 4.0}, Tuple2Float64Float64{_0: 6.0, _1: 0.0}, Tuple2Float64Float64{_0: 3.0, _1: -2.0}}

	fmt.Println("=== Points ===")
	for i, p := range points {
		tmp7 := p
		x := tmp7._0
		y := tmp7._1
		fmt.Printf("P%d: (%.1f, %.1f)\n", i, x, y)
	}

	// Calculate distances
	d := Distance(points[0], points[1])
	fmt.Printf("\nDistance P0 to P1: %.2f\n", d)

	// Find midpoint
	mid := Midpoint(points[0], points[2])
	tmp8 := mid
	mx := tmp8._0
	my := tmp8._1
	fmt.Printf("Midpoint P0-P2: (%.1f, %.1f)\n", mx, my)

	// Calculate bounding box
	bbox := CalculateBoundingBox(points)
	tmp9 := bbox
	minX := tmp9._0._0
	minY := tmp9._0._1
	maxX := tmp9._1._0
	maxY := tmp9._1._1
	fmt.Printf("\nBounding Box: (%.1f, %.1f) to (%.1f, %.1f)\n", minX, minY, maxX, maxY)

	// Transform all points (scale by 2)
	scaled := TransformPoints(points, func(p Point2D) Point2D {
		tmp10 := p
		x := tmp10._0
		y := tmp10._1
		return Tuple2Float64Float64{_0: x * 2, _1: y * 2}
	})

	fmt.Println("\n=== Scaled Points (2x) ===")
	for i, p := range scaled {
		tmp11 := p
		x := tmp11._0
		y := tmp11._1
		fmt.Printf("P%d: (%.1f, %.1f)\n", i, x, y)
	}
}
