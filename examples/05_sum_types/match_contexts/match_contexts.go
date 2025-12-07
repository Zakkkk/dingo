package main

import "fmt"

// Enum definition
type Status interface{ isStatus() }

type StatusPending struct{}

func (StatusPending) isStatus() {}
func NewStatusPending() Status  { return StatusPending{} }

type StatusActive struct{}

func (StatusActive) isStatus() {}
func NewStatusActive() Status  { return StatusActive{} }

type StatusDone struct{}

func (StatusDone) isStatus() {}
func NewStatusDone() Status  { return StatusDone{} }

// Test 1: Assignment Context
func testAssignment(status Status) {
	var x string
	val := status
	switch val.(type) {
	case StatusPending:
		x = "waiting"
	case StatusActive:
		x = "running"
	case StatusDone:
		x = "finished"
	}
	fmt.Println("Assignment result:", x)
}

// Test 2: Return Context
func testReturn(status Status) string {
	val1 := status
	switch val1.(type) {
	case StatusPending:
		return "waiting"
	case StatusActive:
		return "running"
	case StatusDone:
		return "finished"
	}
	panic("unreachable: exhaustive match")
}

// Test 3: Argument Context
func testArgument(status Status) {
	var result string
	val2 := status
	switch val2.(type) {
	case StatusPending:
		result = "waiting"
	case StatusActive:
		result = "running"
	case StatusDone:
		result = "finished"
	}
	fmt.Println("Argument result:", result)
}

// Test 4: Multiple contexts in one function
func testMultiple(status Status) string {
	// Assignment context
	var msg string
	val3 := status
	switch val3.(type) {
	case StatusPending:
		msg = "Job is pending"
	case StatusActive:
		msg = "Job is active"
	case StatusDone:
		msg = "Job is done"
	}

	// Argument context
	var result string
	val4 := status
	switch val4.(type) {
	case StatusPending:
		result = "PENDING"
	case StatusActive:
		result = "ACTIVE"
	case StatusDone:
		result = "DONE"
	}
	fmt.Println("Status:", result)

	// Use the msg variable to avoid unused warning
	fmt.Println("Message:", msg)

	// Return context
	val5 := status
	switch val5.(type) {
	case StatusPending:
		return "waiting for resources"
	case StatusActive:
		return "processing"
	case StatusDone:
		return "completed successfully"
	}
	panic("unreachable: exhaustive match")
}

func main() {
	pending := NewStatusPending()
	active := NewStatusActive()
	done := NewStatusDone()

	fmt.Println("=== Test 1: Assignment Context ===")
	testAssignment(pending)
	testAssignment(active)
	testAssignment(done)

	fmt.Println("\n=== Test 2: Return Context ===")
	fmt.Println("Return result:", testReturn(pending))
	fmt.Println("Return result:", testReturn(active))
	fmt.Println("Return result:", testReturn(done))

	fmt.Println("\n=== Test 3: Argument Context ===")
	testArgument(pending)
	testArgument(active)
	testArgument(done)

	fmt.Println("\n=== Test 4: Multiple Contexts ===")
	result := testMultiple(active)
	fmt.Println("Final result:", result)
}
