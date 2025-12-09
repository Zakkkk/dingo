// Functional Programming with Dingo
// Demonstrates all dgo functional utilities with lambda syntax
//
// KNOWN BUGS (see ai-docs/bugs/):
// 1. block_lambda_parse_error.md - Block lambdas (params) => { ... } fail to parse
// 2. lambda_return_type_inference.md - Void lambdas get wrong return type
// 3. match_option_codegen.md - Match on Option generates invalid code
// 4. lambda_generic_return_type.md - Lambda return types inferred as `any` not concrete
package main

import (
	"fmt"
	"strings"

	"github.com/MadAppGang/dingo/pkg/dgo"
)

// ============================================================================
// Data Types for Examples
// ============================================================================

type Product struct {
	ID       int
	Name     string
	Price    float64
	Category string
	InStock  bool
}

type Order struct {
	ID       int
	Customer string
	Items    []string
	Total    float64
}

// ============================================================================
// Core Functions: Map, Filter, Reduce
// ============================================================================

func demonstrateCoreFunctions() {
	fmt.Println("=== Core Functions ===")

	numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Map: Transform each element
	// Rust-style lambda: |params| expr
	doubled = dgo.Map(numbers, func(x __TYPE_INFERENCE_NEEDED) { return x * 2 })
	fmt.Println("Doubled:", doubled)

	// Map with type conversion
	// TypeScript-style lambda: (params) => expr
	asStrings = dgo.Map(numbers, func(n __TYPE_INFERENCE_NEEDED) { return fmt.Sprintf("#%d", n) })
	fmt.Println("As strings:", asStrings)

	// Filter: Keep elements matching predicate
	evens = dgo.Filter(numbers, func(x __TYPE_INFERENCE_NEEDED) { return x%2 == 0 })
	fmt.Println("Evens:", evens)

	// Filter with complex condition
	inRange = dgo.Filter(numbers, func(x __TYPE_INFERENCE_NEEDED) { return x >= 3 && x <= 7 })
	fmt.Println("In range [3,7]:", inRange)

	// Reduce: Fold into single value
	sum = dgo.Reduce(numbers, 0, func(acc __TYPE_INFERENCE_NEEDED, x __TYPE_INFERENCE_NEEDED) { return acc + x })
	fmt.Println("Sum:", sum)

	// Reduce: Product
	product = dgo.Reduce(numbers, 1, func(acc __TYPE_INFERENCE_NEEDED, x __TYPE_INFERENCE_NEEDED) { return acc * x })
	fmt.Println("Product:", product)

	// ForEach Using regular Go func (lambda void return bug workaround)
	fmt.Print("ForEach output: ")
	for _, s := range []string{"a", "b", "c"} {
		fmt.Print(s, " ")
	}
	fmt.Println()
}

// ============================================================================
// Index-Aware Variants
// ============================================================================

func demonstrateIndexAware() {
	fmt.Println("\n=== Index-Aware Functions ===")

	words := []string{"apple", "banana", "cherry", "date"}

	// MapWithIndex: Transform with access to index
	indexed = dgo.MapWithIndex(words, func(i __TYPE_INFERENCE_NEEDED, w __TYPE_INFERENCE_NEEDED) { return fmt.Sprintf("%d. %s", i+1, w) })
	fmt.Println("Indexed list:")
	for _, s := range indexed {
		fmt.Println("  ", s)
	}

	// FilterWithIndex: Filter by index
	everyOther = dgo.FilterWithIndex(words, func(i __TYPE_INFERENCE_NEEDED, w __TYPE_INFERENCE_NEEDED) { return i%2 == 0 })
	fmt.Println("Every other:", everyOther)

	// ForEachWithIndex: Using regular Go for loop (lambda void return bug workaround)
	fmt.Println("With indices:")
	for i, w := range words {
		fmt.Printf("  [%d] %s\n", i, w)
	}
}

// ============================================================================
// Search and Predicate Functions
// ============================================================================

func demonstrateSearch() {
	fmt.Println("\n=== Search & Predicate Functions ===")

	products := []Product{
		{1, "Laptop", 999.99, "Electronics", true},
		{2, "Mouse", 29.99, "Electronics", true},
		{3, "Keyboard", 79.99, "Electronics", false},
		{4, "Desk", 299.99, "Furniture", true},
		{5, "Chair", 199.99, "Furniture", true},
	}

	// Find: First matching element (returns Option)
	// Using method-based API (match Option bug workaround)
	found = dgo.Find(products, func(p __TYPE_INFERENCE_NEEDED) { return p.Price > 500 })
	if found.IsSome() {
		p := found.Unwrap()
		fmt.Printf("Found expensive item: %s ($%.2f)\n", p.Name, p.Price)
	} else {
		fmt.Println("No expensive items found")
	}

	// FindIndex: Index of first match
	idx = dgo.FindIndex(products, func(p __TYPE_INFERENCE_NEEDED) { return p.Category == "Furniture" })
	if idx.IsSome() {
		fmt.Printf("First furniture at index: %d\n", idx.Unwrap())
	} else {
		fmt.Println("No furniture found")
	}

	// Any: Check if any element matches
	hasOutOfStock = dgo.Any(products, func(p __TYPE_INFERENCE_NEEDED) { return !p.InStock })
	fmt.Printf("Has out of stock items: %v\n", hasOutOfStock)

	// All: Check if all elements match
	allInStock = dgo.All(products, func(p __TYPE_INFERENCE_NEEDED) { return p.InStock })
	fmt.Printf("All items in stock: %v\n", allInStock)

	// NoneMatch: Check if no elements match
	noExpensive = dgo.NoneMatch(products, func(p __TYPE_INFERENCE_NEEDED) { return p.Price > 2000 })
	fmt.Printf("No items over $2000: %v\n", noExpensive)

	// Contains: Check for value (comparable types)
	numbers := []int{10, 20, 30, 40, 50}
	has30 := dgo.Contains(numbers, 30)
	fmt.Printf("Contains 30: %v\n", has30)

	// Count: Count matching elements
	electronicsCount = dgo.Count(products, func(p __TYPE_INFERENCE_NEEDED) { return p.Category == "Electronics" })
	fmt.Printf("Electronics count: %d\n", electronicsCount)
}

// ============================================================================
// Advanced Functions
// ============================================================================

func demonstrateAdvanced() {
	fmt.Println("\n=== Advanced Functions ===")

	orders := []Order{
		{1, "Alice", []string{"Laptop", "Mouse"}, 1029.98},
		{2, "Bob", []string{"Keyboard"}, 79.99},
		{3, "Alice", []string{"Desk", "Chair"}, 499.98},
	}

	// FlatMap: Map then flatten
	// Using explicit func due to type inference bug (lambda_generic_return_type.md)
	allItems = dgo.FlatMap(orders, func(o Order) []string { return o.Items })
	fmt.Println("All ordered items:", allItems)

	// Flatten: Flatten nested slices
	nested := [][]int{{1, 2}, {3, 4, 5}, {6}}
	flat := dgo.Flatten(nested)
	fmt.Println("Flattened:", flat)

	// Partition: Split into two groups
	numbers := []int{1, 2, 3, 4, 5, 6, 7, 8}
	evens, odds = dgo.Partition(numbers, func(x __TYPE_INFERENCE_NEEDED) { return x%2 == 0 })
	fmt.Printf("Evens: %v, Odds: %v\n", evens, odds)

	// GroupBy: Group by key
	products := []Product{
		{1, "Laptop", 999.99, "Electronics", true},
		{2, "Mouse", 29.99, "Electronics", true},
		{3, "Desk", 299.99, "Furniture", true},
		{4, "Chair", 199.99, "Furniture", true},
	}
	byCategory = dgo.GroupBy(products, func(p __TYPE_INFERENCE_NEEDED) { return p.Category })
	fmt.Println("Products by category:")
	for cat, prods := range byCategory {
		names = dgo.Map(prods, func(p __TYPE_INFERENCE_NEEDED) { return p.Name })
		fmt.Printf("  %s: %v\n", cat, names)
	}

	// Unique: Remove duplicates
	withDups := []string{"apple", "banana", "apple", "cherry", "banana"}
	unique := dgo.Unique(withDups)
	fmt.Println("Unique fruits:", unique)

	// Reverse: Reverse order
	reversed := dgo.Reverse([]int{1, 2, 3, 4, 5})
	fmt.Println("Reversed:", reversed)
}

// ============================================================================
// Slice Manipulation
// ============================================================================

func demonstrateSliceManipulation() {
	fmt.Println("\n=== Slice Manipulation ===")

	numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Take: First n elements
	first3 := dgo.Take(numbers, 3)
	fmt.Println("First 3:", first3)

	// Drop: Skip first n elements
	after3 := dgo.Drop(numbers, 3)
	fmt.Println("After first 3:", after3)

	// TakeWhile: Take while condition holds
	smallNums = dgo.TakeWhile(numbers, func(x __TYPE_INFERENCE_NEEDED) { return x < 5 })
	fmt.Println("Take while < 5:", smallNums)

	// DropWhile: Drop while condition holds
	fromFive = dgo.DropWhile(numbers, func(x __TYPE_INFERENCE_NEEDED) { return x < 5 })
	fmt.Println("Drop while < 5:", fromFive)

	// Chunk: Split into chunks
	chunks := dgo.Chunk(numbers, 3)
	fmt.Println("Chunks of 3:", chunks)

	// ZipSlices: Combine two slices
	names := []string{"Alice", "Bob", "Charlie"}
	scores := []int{95, 87, 92}
	pairs := dgo.ZipSlices(names, scores)
	fmt.Println("Zipped name-score pairs:")
	for _, p := range pairs {
		fmt.Printf("  %s: %d\n", p.First, p.Second)
	}
}

// ============================================================================
// Chaining Operations (Nested Calls)
// ============================================================================

func demonstrateChaining() {
	fmt.Println("\n=== Chaining Operations ===")

	products := []Product{
		{1, "Laptop", 999.99, "Electronics", true},
		{2, "Mouse", 29.99, "Electronics", true},
		{3, "Keyboard", 79.99, "Electronics", false},
		{4, "Desk", 299.99, "Furniture", true},
		{5, "Chair", 199.99, "Furniture", false},
		{6, "Monitor", 449.99, "Electronics", true},
	}

	// Chain: Filter -> Map -> Reduce
	// Get total value of in-stock electronics
	// Using explicit func for Reduce due to type inference bug
	inStockElectronics = dgo.Filter(products, func(p __TYPE_INFERENCE_NEEDED) { return p.InStock && p.Category == "Electronics" })
	prices = dgo.Map(inStockElectronics, func(p Product) float64 { return p.Price })
	totalValue = dgo.Reduce(prices, 0.0, func(acc, price float64) float64 { return acc + price })
	fmt.Printf("Total in-stock electronics value: $%.2f\n", totalValue)

	// Chain: Filter -> Map -> Take
	// Get first 2 expensive item names
	expensiveNames := dgo.Take(
		dgo.Map(
			dgo.Filter(products, func(p __TYPE_INFERENCE_NEEDED) { return p.Price > 100 }),
			func(p __TYPE_INFERENCE_NEEDED) { return strings.ToUpper(p.Name) },
		),
		2,
	)
	fmt.Println("First 2 expensive items:", expensiveNames)

	// Complex pipeline: Transform products for display
	display := dgo.Map(
		dgo.Filter(products, func(p __TYPE_INFERENCE_NEEDED) { return p.InStock }),
		func(p __TYPE_INFERENCE_NEEDED) { return fmt.Sprintf("%s ($%.2f)", p.Name, p.Price) },
	)
	fmt.Println("In-stock products:")
	for _, s := range display {
		fmt.Println("  -", s)
	}
}

// ============================================================================
// Real-World Example: Order Processing
// ============================================================================

func demonstrateRealWorld() {
	fmt.Println("\n=== Real-World: Order Processing ===")

	orders := []Order{
		{1, "Alice", []string{"Laptop", "Mouse"}, 1029.98},
		{2, "Bob", []string{"Keyboard", "Mousepad"}, 89.98},
		{3, "Alice", []string{"Monitor"}, 449.99},
		{4, "Charlie", []string{"Desk", "Chair", "Lamp"}, 549.97},
		{5, "Bob", []string{"Webcam"}, 79.99},
	}

	// Group orders by customer
	byCustomer = dgo.GroupBy(orders, func(o __TYPE_INFERENCE_NEEDED) { return o.Customer })

	// Calculate total spent per customer
	fmt.Println("Customer totals:")
	for customer, customerOrders := range byCustomer {
		total = dgo.Reduce(customerOrders, 0.0, func(acc __TYPE_INFERENCE_NEEDED, o __TYPE_INFERENCE_NEEDED) { return acc + o.Total })
		orderCount := len(customerOrders)
		fmt.Printf("  %s: $%.2f (%d orders)\n", customer, total, orderCount)
	}

	// Find high-value orders
	highValue = dgo.Filter(orders, func(o __TYPE_INFERENCE_NEEDED) { return o.Total > 100 })
	fmt.Printf("\nHigh-value orders (>$100): %d\n", len(highValue))

	// Get all unique items ordered
	// Using explicit func due to type inference bug
	allItems = dgo.Unique(dgo.FlatMap(orders, func(o Order) []string { return o.Items }))
	fmt.Println("All unique items ordered:", allItems)

	// Count orders by customer
	fmt.Println("\nOrder counts:")
	for customer, customerOrders := range byCustomer {
		fmt.Printf("  %s: %d orders\n", customer, len(customerOrders))
	}
}

// ============================================================================
// Main
// ============================================================================

func main() {
	demonstrateCoreFunctions()
	demonstrateIndexAware()
	demonstrateSearch()
	demonstrateAdvanced()
	demonstrateSliceManipulation()
	demonstrateChaining()
	demonstrateRealWorld()

	fmt.Println("\n=== All functional examples complete! ===")
}
