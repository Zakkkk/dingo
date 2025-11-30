package preprocessor

import (
	"strings"
	"testing"
)

// Test chain detection

func TestDetectChain_FilterMap(t *testing.T) {
	f := NewFunctionalProcessor()
	line := `result := nums.filter(func(x) { return x > 0 }).map(func(y) { return y * 2 })`

	chain := f.detectChain(line)
	if chain == nil {
		t.Fatal("Expected to detect chain, got nil")
	}

	if len(chain.Operations) != 2 {
		t.Fatalf("Expected 2 operations, got %d", len(chain.Operations))
	}

	if chain.Operations[0].Method != "filter" {
		t.Errorf("First operation should be filter, got %s", chain.Operations[0].Method)
	}

	if chain.Operations[1].Method != "map" {
		t.Errorf("Second operation should be map, got %s", chain.Operations[1].Method)
	}
}

func TestDetectChain_MapMapMap(t *testing.T) {
	f := NewFunctionalProcessor()
	line := `x := items.map(func(a) { return a + 1 }).map(func(b) { return b * 2 }).map(func(c) { return c - 1 })`

	chain := f.detectChain(line)
	if chain == nil {
		t.Fatal("Expected to detect chain, got nil")
	}

	if len(chain.Operations) != 3 {
		t.Fatalf("Expected 3 operations, got %d", len(chain.Operations))
	}

	for i, op := range chain.Operations {
		if op.Method != "map" {
			t.Errorf("Operation %d should be map, got %s", i, op.Method)
		}
	}
}

func TestDetectChain_FilterReduce(t *testing.T) {
	f := NewFunctionalProcessor()
	line := `sum := nums.filter(func(x) { return x > 0 }).reduce(0, func(acc, x) { return acc + x })`

	chain := f.detectChain(line)
	if chain == nil {
		t.Fatal("Expected to detect chain, got nil")
	}

	if len(chain.Operations) != 2 {
		t.Fatalf("Expected 2 operations, got %d", len(chain.Operations))
	}

	if chain.Operations[0].Method != "filter" {
		t.Errorf("First operation should be filter, got %s", chain.Operations[0].Method)
	}

	if chain.Operations[1].Method != "reduce" {
		t.Errorf("Second operation should be reduce, got %s", chain.Operations[1].Method)
	}
}

func TestDetectChain_SingleOperation(t *testing.T) {
	f := NewFunctionalProcessor()
	line := `result := nums.map(func(x) { return x * 2 })`

	chain := f.detectChain(line)
	// Single operations should return nil (handled by processLine, not chain detection)
	if chain != nil {
		t.Fatal("Expected nil for single operation, got chain")
	}
}

// Test chain fusion

func TestFuseChain_FilterMap(t *testing.T) {
	f := NewFunctionalProcessor()
	// Use typed lambdas to get proper type inference
	line := `result := nums.filter(func(x int) bool { return x > 0 }).map(func(y int) int { return y * 2 })`

	chain := f.detectChain(line)
	if chain == nil {
		t.Fatal("Failed to detect chain")
	}

	iife, err := f.fuseChain(chain)
	if err != nil {
		t.Fatalf("fuseChain failed: %v", err)
	}

	t.Logf("Generated IIFE:\n%s", iife)

	// Verify generated IIFE structure - typed lambdas produce proper types
	if !strings.Contains(iife, "func() []int") {
		t.Error("IIFE should have func() []int signature")
	}

	if !strings.Contains(iife, "tmp := make([]int, 0, len(nums))") {
		t.Error("IIFE should allocate tmp slice with int type")
	}

	if !strings.Contains(iife, "for _, x := range nums") {
		t.Error("IIFE should have loop over nums")
	}

	// Check fusion: filter condition and map transformation
	if !strings.Contains(iife, "if x > 0") {
		t.Error("IIFE should contain filter predicate")
	}

	// Map transformation should be composed: y parameter substituted with x
	if !strings.Contains(iife, "x * 2") || !strings.Contains(iife, "append") {
		t.Error("IIFE should contain composed map transformation (x * 2) in append")
	}

	// Check no intermediate allocation (single tmp variable)
	if strings.Count(iife, "make([]int") > 1 {
		t.Error("Fused chain should only have one allocation")
	}
}

func TestFuseChain_FilterFilter(t *testing.T) {
	f := NewFunctionalProcessor()
	line := `result := items.filter(func(x) { return x.active }).filter(func(y) { return y.value > 10 })`

	chain := f.detectChain(line)
	if chain == nil {
		t.Fatal("Failed to detect chain")
	}

	iife, err := f.fuseChain(chain)
	if err != nil {
		t.Fatalf("fuseChain failed: %v", err)
	}

	// Check combined predicates with &&
	if !strings.Contains(iife, "if x.active && y.value > 10") && !strings.Contains(iife, "if x.active && x.value > 10") {
		t.Error("IIFE should combine filter predicates with &&")
	}

	// Check appends original value (not transformed)
	if !strings.Contains(iife, "append(tmp, x)") {
		t.Error("IIFE should append original element for filter chain")
	}
}

func TestFuseChain_MapMap(t *testing.T) {
	f := NewFunctionalProcessor()
	line := `result := nums.map(func(x) { return x * 2 }).map(func(y) { return y + 1 })`

	chain := f.detectChain(line)
	if chain == nil {
		t.Fatal("Failed to detect chain")
	}

	iife, err := f.fuseChain(chain)
	if err != nil {
		t.Fatalf("fuseChain failed: %v", err)
	}

	// Check composed transformation
	// Should append (x * 2) + 1, not create intermediate slice
	if !strings.Contains(iife, "append(tmp, (x * 2) + 1)") && !strings.Contains(iife, "append(tmp, (x*2)+1)") {
		t.Error("IIFE should compose map transformations")
	}

	// Check single allocation
	if strings.Count(iife, "make([]") > 1 {
		t.Error("Fused map chain should only have one allocation")
	}
}

func TestFuseChain_FilterMapReduce(t *testing.T) {
	f := NewFunctionalProcessor()
	// Using typed lambdas to generate proper types
	line := `sum := items.filter(func(x Item) bool { return x.active }).map(func(y Item) int { return y.value }).reduce(0, func(acc int, v int) int { return acc + v })`

	chain := f.detectChain(line)
	if chain == nil {
		t.Fatal("Failed to detect chain")
	}

	iife, err := f.fuseChain(chain)
	if err != nil {
		t.Fatalf("fuseChain failed: %v", err)
	}

	t.Logf("Generated IIFE:\n%s", iife)

	// Verify reduce structure - uses return type from lambda
	if !strings.Contains(iife, "func() int") {
		t.Errorf("Reduce IIFE should return int, got: %s", iife)
	}

	if !strings.Contains(iife, "acc := 0") {
		t.Error("IIFE should initialize accumulator")
	}

	// Check filter condition
	if !strings.Contains(iife, "if x.active") {
		t.Error("IIFE should contain filter condition")
	}

	// Check map transformation in reduce body (y parameter substituted with x)
	if !strings.Contains(iife, "x.value") {
		t.Error("IIFE should use composed map transformation (x.value)")
	}

	// Check accumulation
	if !strings.Contains(iife, "acc = acc + ") {
		t.Error("IIFE should contain accumulation logic")
	}

	// Check NO intermediate slice allocation
	if strings.Contains(iife, "make([]") {
		t.Error("Fused reduce chain should NOT allocate intermediate slice")
	}
}

func TestFuseChain_FilterAll(t *testing.T) {
	f := NewFunctionalProcessor()
	line := `check := items.filter(func(x) { return x.value > 0 }).all(func(y) { return y.active })`

	chain := f.detectChain(line)
	if chain == nil {
		t.Fatal("Failed to detect chain")
	}

	iife, err := f.fuseChain(chain)
	if err != nil {
		t.Fatalf("fuseChain failed: %v", err)
	}

	// Verify all() structure
	if !strings.Contains(iife, "func() bool") {
		t.Error("all() IIFE should return bool")
	}

	// Check combined predicate
	if !strings.Contains(iife, "if !(x.value > 0 && y.active)") && !strings.Contains(iife, "if !(x.value > 0 && x.active)") {
		t.Error("IIFE should combine filter and all predicates")
	}

	// Check early exit
	if !strings.Contains(iife, "return false") {
		t.Error("all() should have early exit")
	}

	// Check NO intermediate slice
	if strings.Contains(iife, "make([]") {
		t.Error("Fused all chain should NOT allocate intermediate slice")
	}
}

func TestFuseChain_FilterAny(t *testing.T) {
	f := NewFunctionalProcessor()
	line := `found := items.filter(func(x) { return x.category == "test" }).any(func(y) { return y.active })`

	chain := f.detectChain(line)
	if chain == nil {
		t.Fatal("Failed to detect chain")
	}

	iife, err := f.fuseChain(chain)
	if err != nil {
		t.Fatalf("fuseChain failed: %v", err)
	}

	// Verify any() structure
	if !strings.Contains(iife, "func() bool") {
		t.Error("any() IIFE should return bool")
	}

	// Check combined predicate
	if !strings.Contains(iife, `x.category == "test" && y.active`) && !strings.Contains(iife, `x.category == "test" && x.active`) {
		t.Error("IIFE should combine filter and any predicates")
	}

	// Check early exit
	if !strings.Contains(iife, "return true") {
		t.Error("any() should have early exit")
	}

	// Check NO intermediate slice
	if strings.Contains(iife, "make([]") {
		t.Error("Fused any chain should NOT allocate intermediate slice")
	}
}

// Test integration with processLine

func TestProcessLine_ChainVsSingle(t *testing.T) {
	f := NewFunctionalProcessor()

	// Single operation - should NOT use chain fusion
	singleLine := `x := nums.map(func(a) { return a * 2 })`
	singleResult, err := f.processLine(singleLine, 1)
	if err != nil {
		t.Fatalf("processLine failed for single operation: %v", err)
	}

	// Chain - should use chain fusion
	chainLine := `y := nums.filter(func(b) { return b > 0 }).map(func(c) { return c * 2 })`
	chainResult, err := f.processLine(chainLine, 2)
	if err != nil {
		t.Fatalf("processLine failed for chain: %v", err)
	}

	// Both should generate IIFEs
	if !strings.Contains(singleResult, "func()") {
		t.Error("Single operation should generate IIFE")
	}

	if !strings.Contains(chainResult, "func()") {
		t.Error("Chain should generate IIFE")
	}

	// Chain should be fused (no intermediate allocation after filter)
	// Single operations for comparison can have two makes if we do both separately
	// But fused chain should only have ONE make
	chainMakeCount := strings.Count(chainResult, "make([]")
	if chainMakeCount > 1 {
		t.Errorf("Fused chain should have at most 1 allocation, got %d", chainMakeCount)
	}
}

func TestProcessLine_Metadata_Chain(t *testing.T) {
	f := NewFunctionalProcessor()
	line := `z := items.filter(func(x) { return x > 0 }).map(func(y) { return y * 2 })`

	_, err := f.processLine(line, 5)
	if err != nil {
		t.Fatalf("processLine failed: %v", err)
	}

	if len(f.metadata) != 1 {
		t.Fatalf("Expected 1 metadata entry, got %d", len(f.metadata))
	}

	meta := f.metadata[0]
	if meta.Type != "functional_chain" {
		t.Errorf("Metadata type should be 'functional_chain', got %s", meta.Type)
	}

	if meta.OriginalLine != 5 {
		t.Errorf("Metadata line should be 5, got %d", meta.OriginalLine)
	}

	if !strings.HasPrefix(meta.GeneratedMarker, "// dingo:f:") {
		t.Errorf("Metadata marker should start with '// dingo:f:', got %s", meta.GeneratedMarker)
	}
}
