// Package tuples provides generic tuple types for Dingo's tuple syntax.
//
// These types are used by the Dingo transpiler when generating Go code
// from tuple literals like (a, b) or type aliases like type Point = (int, int).
//
// Field naming convention:
//   - First, Second, Third, Fourth, Fifth, Sixth, Seventh, Eighth, Ninth, Tenth
//
// Usage:
//   import "github.com/MadAppGang/dingo/runtime/tuples"
//
//   type Point2D = tuples.Tuple2[float64, float64]
//   p := Point2D{First: 3.0, Second: 4.0}
package tuples

// Tuple2 is a generic 2-element tuple.
type Tuple2[A, B any] struct {
	First  A
	Second B
}

// Tuple3 is a generic 3-element tuple.
type Tuple3[A, B, C any] struct {
	First  A
	Second B
	Third  C
}

// Tuple4 is a generic 4-element tuple.
type Tuple4[A, B, C, D any] struct {
	First  A
	Second B
	Third  C
	Fourth D
}

// Tuple5 is a generic 5-element tuple.
type Tuple5[A, B, C, D, E any] struct {
	First  A
	Second B
	Third  C
	Fourth D
	Fifth  E
}

// Tuple6 is a generic 6-element tuple.
type Tuple6[A, B, C, D, E, F any] struct {
	First  A
	Second B
	Third  C
	Fourth D
	Fifth  E
	Sixth  F
}

// Tuple7 is a generic 7-element tuple.
type Tuple7[A, B, C, D, E, F, G any] struct {
	First   A
	Second  B
	Third   C
	Fourth  D
	Fifth   E
	Sixth   F
	Seventh G
}

// Tuple8 is a generic 8-element tuple.
type Tuple8[A, B, C, D, E, F, G, H any] struct {
	First   A
	Second  B
	Third   C
	Fourth  D
	Fifth   E
	Sixth   F
	Seventh G
	Eighth  H
}

// Tuple9 is a generic 9-element tuple.
type Tuple9[A, B, C, D, E, F, G, H, I any] struct {
	First   A
	Second  B
	Third   C
	Fourth  D
	Fifth   E
	Sixth   F
	Seventh G
	Eighth  H
	Ninth   I
}

// Tuple10 is a generic 10-element tuple.
type Tuple10[A, B, C, D, E, F, G, H, I, J any] struct {
	First   A
	Second  B
	Third   C
	Fourth  D
	Fifth   E
	Sixth   F
	Seventh G
	Eighth  H
	Ninth   I
	Tenth   J
}
