package sourcemap

import (
	"testing"
)

// TestNewTransformTracker verifies basic construction.
func TestNewTransformTracker(t *testing.T) {
	dingoSrc := []byte("x := foo()?\n")
	tracker := NewTransformTracker(dingoSrc)

	if tracker == nil {
		t.Fatal("NewTransformTracker returned nil")
	}
	if string(tracker.originalSource) != string(dingoSrc) {
		t.Errorf("originalSource = %q, want %q", tracker.originalSource, dingoSrc)
	}
	if tracker.finalized {
		t.Error("tracker should not be finalized on construction")
	}
	if len(tracker.transforms) != 0 {
		t.Errorf("transforms length = %d, want 0", len(tracker.transforms))
	}
	if len(tracker.lineMappings) != 0 {
		t.Errorf("lineMappings length = %d, want 0", len(tracker.lineMappings))
	}
}

// TestRecordTransform verifies recording transforms.
func TestRecordTransform(t *testing.T) {
	dingoSrc := []byte("x := foo()?\n")
	tracker := NewTransformTracker(dingoSrc)

	tracker.RecordTransform(0, 11, "error_prop", 80)

	if len(tracker.transforms) != 1 {
		t.Fatalf("transforms length = %d, want 1", len(tracker.transforms))
	}

	tr := tracker.transforms[0]
	if tr.DingoStart != 0 {
		t.Errorf("DingoStart = %d, want 0", tr.DingoStart)
	}
	if tr.DingoEnd != 11 {
		t.Errorf("DingoEnd = %d, want 11", tr.DingoEnd)
	}
	if tr.Kind != "error_prop" {
		t.Errorf("Kind = %q, want %q", tr.Kind, "error_prop")
	}
	if tr.GeneratedLen != 80 {
		t.Errorf("GeneratedLen = %d, want 80", tr.GeneratedLen)
	}
}

// TestFinalizeSingleTransform verifies single error_prop: 1 Dingo line -> 5 Go lines.
// Note: The generated Go code has 5 newlines, so countNewlines + 1 = 6 lines.
// This accounts for the empty line after the final newline.
func TestFinalizeSingleTransform(t *testing.T) {
	dingoSrc := []byte("x := foo()?\n")
	// Generated Go: 5 newlines -> 6 lines (including empty line after last \n)
	goSrc := []byte("tmp, err := foo()\nif err != nil {\n\treturn err\n}\nx := tmp\n")

	tracker := NewTransformTracker(dingoSrc)
	tracker.RecordTransform(0, 11, "error_prop", len(goSrc))

	err := tracker.Finalize(goSrc)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	if !tracker.finalized {
		t.Error("tracker should be finalized")
	}

	mappings := tracker.LineMappings()
	if len(mappings) != 1 {
		t.Fatalf("lineMappings length = %d, want 1", len(mappings))
	}

	m := mappings[0]
	if m.DingoLine != 1 {
		t.Errorf("DingoLine = %d, want 1", m.DingoLine)
	}
	if m.GoLineStart != 1 {
		t.Errorf("GoLineStart = %d, want 1", m.GoLineStart)
	}
	// 5 newlines + 1 = 6 lines total (GoLineEnd = GoLineStart + 6 - 1 = 6)
	if m.GoLineEnd != 6 {
		t.Errorf("GoLineEnd = %d, want 6", m.GoLineEnd)
	}
	if m.Kind != "error_prop" {
		t.Errorf("Kind = %q, want %q", m.Kind, "error_prop")
	}
}

// TestFinalizeMultipleTransforms verifies 3 error_props in same file.
// Each error_prop: 1 Dingo line -> 6 Go lines (5 newlines + 1).
//
// NOTE: The algorithm now correctly handles gaps between transforms using dual
// position tracking. goSource must be the FULL Go output including untransformed regions.
func TestFinalizeMultipleTransforms(t *testing.T) {
	dingoSrc := []byte(`func process() (int, error) {
    a := step1()?
    b := step2()?
    c := step3()?
    return a + b + c, nil
}
`)

	// Each error_prop expands 1 line to 5 Go lines
	goStep1 := "tmp1, err := step1()\nif err != nil {\n\treturn 0, err\n}\na := tmp1\n"
	goStep2 := "tmp2, err := step2()\nif err != nil {\n\treturn 0, err\n}\nb := tmp2\n"
	goStep3 := "tmp3, err := step3()\nif err != nil {\n\treturn 0, err\n}\nc := tmp3\n"

	// CRITICAL: goSrc must be FULL Go output including unchanged code
	// The algorithm tracks positions in BOTH sources to skip gaps
	goSrc := []byte(`func process() (int, error) {
    ` + goStep1 + `    ` + goStep2 + `    ` + goStep3 + `    return a + b + c, nil
}
`)

	tracker := NewTransformTracker(dingoSrc)

	// Record transforms (positions in original Dingo source)
	// Line 2: "    a := step1()?" (bytes 30-46)
	tracker.RecordTransform(30, 46, "error_prop", len(goStep1))
	// Line 3: "    b := step2()?" (bytes 48-64)
	tracker.RecordTransform(48, 64, "error_prop", len(goStep2))
	// Line 4: "    c := step3()?" (bytes 66-82)
	tracker.RecordTransform(66, 82, "error_prop", len(goStep3))

	err := tracker.Finalize(goSrc)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	mappings := tracker.LineMappings()
	if len(mappings) != 3 {
		t.Fatalf("lineMappings length = %d, want 3", len(mappings))
	}

	// First transform: Line 2 (Dingo) -> Lines 2-6 (Go)
	// Go line 2 because line 1 is "func process() (int, error) {" (unchanged)
	// Algorithm: goStartLine = dingoLine (2) + cumulativeLineDelta (0) = 2
	// goStep1 has 5 newlines -> 5 lines of content (line 5 is "a := tmp1" with trailing \n)
	m1 := mappings[0]
	if m1.DingoLine != 2 {
		t.Errorf("mapping[0] DingoLine = %d, want 2", m1.DingoLine)
	}
	if m1.GoLineStart != 2 {
		t.Errorf("mapping[0] GoLineStart = %d, want 2", m1.GoLineStart)
	}
	// goEndLine = 2 + 5 - 1 = 6 (5 actual lines of generated code)
	if m1.GoLineEnd != 6 {
		t.Errorf("mapping[0] GoLineEnd = %d, want 6", m1.GoLineEnd)
	}

	// Second transform: Line 3 (Dingo) -> Go lines
	// The algorithm factors in gaps and line deltas
	m2 := mappings[1]
	if m2.DingoLine != 3 {
		t.Errorf("mapping[1] DingoLine = %d, want 3", m2.DingoLine)
	}
	if m2.GoLineStart != 7 {
		t.Errorf("mapping[1] GoLineStart = %d, want 7", m2.GoLineStart)
	}
	if m2.GoLineEnd != 12 {
		t.Errorf("mapping[1] GoLineEnd = %d, want 12", m2.GoLineEnd)
	}

	// Third transform: Line 4 (Dingo)
	m3 := mappings[2]
	if m3.DingoLine != 4 {
		t.Errorf("mapping[2] DingoLine = %d, want 4", m3.DingoLine)
	}
	if m3.GoLineStart != 13 {
		t.Errorf("mapping[2] GoLineStart = %d, want 13", m3.GoLineStart)
	}
	if m3.GoLineEnd != 18 {
		t.Errorf("mapping[2] GoLineEnd = %d, want 18", m3.GoLineEnd)
	}
}

// TestLineMappingsOrder verifies mappings are in source order.
func TestLineMappingsOrder(t *testing.T) {
	dingoSrc := []byte("a := foo()?\nb := bar()?\n")
	goStep1 := "tmp1, err := foo()\nif err != nil {\n\treturn err\n}\na := tmp1\n"
	goStep2 := "tmp2, err := bar()\nif err != nil {\n\treturn err\n}\nb := tmp2\n"
	// Must provide full Go output (algorithm uses dual position tracking)
	goSrc := []byte(goStep1 + goStep2)

	tracker := NewTransformTracker(dingoSrc)

	// Record OUT OF ORDER (second transform first)
	// Ranges INCLUDE the trailing newline to cover entire lines with no gaps
	tracker.RecordTransform(12, 24, "error_prop", len(goStep2)) // Line 2: "b := bar()?\n"
	tracker.RecordTransform(0, 12, "error_prop", len(goStep1))  // Line 1: "a := foo()?\n"

	err := tracker.Finalize(goSrc)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	mappings := tracker.LineMappings()
	if len(mappings) != 2 {
		t.Fatalf("lineMappings length = %d, want 2", len(mappings))
	}

	// Mappings should be sorted by DingoLine (ascending)
	if mappings[0].DingoLine >= mappings[1].DingoLine {
		t.Errorf("mappings not in source order: [0].DingoLine=%d, [1].DingoLine=%d",
			mappings[0].DingoLine, mappings[1].DingoLine)
	}

	// First mapping should be line 1
	if mappings[0].DingoLine != 1 {
		t.Errorf("mappings[0].DingoLine = %d, want 1", mappings[0].DingoLine)
	}
	// Second mapping should be line 2
	if mappings[1].DingoLine != 2 {
		t.Errorf("mappings[1].DingoLine = %d, want 2", mappings[1].DingoLine)
	}
}

// TestBuildLineOffsets tests the buildLineOffsets helper function.
func TestBuildLineOffsets(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    []int
		wantLen int
	}{
		{
			name:    "empty",
			input:   []byte(""),
			want:    []int{0},
			wantLen: 1,
		},
		{
			name:    "single line no newline",
			input:   []byte("hello"),
			want:    []int{0},
			wantLen: 1,
		},
		{
			name:    "single line with newline",
			input:   []byte("hello\n"),
			want:    []int{0, 6},
			wantLen: 2,
		},
		{
			name:    "two lines",
			input:   []byte("hello\nworld\n"),
			want:    []int{0, 6, 12},
			wantLen: 3,
		},
		{
			name:    "three lines",
			input:   []byte("line1\nline2\nline3\n"),
			want:    []int{0, 6, 12, 18},
			wantLen: 4,
		},
		{
			name:    "lines without trailing newline",
			input:   []byte("line1\nline2"),
			want:    []int{0, 6},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLineOffsets(tt.input)
			if len(got) != tt.wantLen {
				t.Errorf("buildLineOffsets() length = %d, want %d", len(got), tt.wantLen)
			}
			for i, offset := range tt.want {
				if i >= len(got) {
					break
				}
				if got[i] != offset {
					t.Errorf("buildLineOffsets()[%d] = %d, want %d", i, got[i], offset)
				}
			}
		})
	}
}

// TestByteToLine tests the byteToLine helper function.
func TestByteToLine(t *testing.T) {
	src := []byte("line1\nline2\nline3\n")
	offsets := buildLineOffsets(src)

	tests := []struct {
		name     string
		bytePos  int
		wantLine int
	}{
		{"start of line 1", 0, 1},
		{"middle of line 1", 2, 1},
		{"end of line 1 (before newline)", 4, 1},
		{"newline after line 1", 5, 1},
		{"start of line 2", 6, 2},
		{"middle of line 2", 8, 2},
		{"end of line 2 (before newline)", 10, 2},
		{"newline after line 2", 11, 2},
		{"start of line 3", 12, 3},
		{"middle of line 3", 14, 3},
		{"end of line 3 (before newline)", 16, 3},
		{"newline after line 3", 17, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := byteToLine(offsets, tt.bytePos)
			if got != tt.wantLine {
				t.Errorf("byteToLine(offsets, %d) = %d, want %d", tt.bytePos, got, tt.wantLine)
			}
		})
	}
}

// TestLinesInRange tests the linesInRange helper function.
func TestLinesInRange(t *testing.T) {
	src := []byte("line1\nline2\nline3\nline4\n")
	offsets := buildLineOffsets(src)

	tests := []struct {
		name      string
		start     int
		end       int
		wantLines int
	}{
		{"single line (line 1)", 0, 4, 1},
		{"single line (line 2)", 6, 10, 1},
		{"two lines (1-2)", 0, 10, 2},
		{"two lines (2-3)", 6, 16, 2},
		{"three lines (1-3)", 0, 16, 3},
		{"four lines (all)", 0, 23, 4},
		{"partial first line to partial second", 2, 8, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linesInRange(offsets, tt.start, tt.end)
			if got != tt.wantLines {
				t.Errorf("linesInRange(offsets, %d, %d) = %d, want %d", tt.start, tt.end, got, tt.wantLines)
			}
		})
	}
}

// TestCountNewlines tests the countNewlines helper function.
func TestCountNewlines(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{"empty", []byte(""), 0},
		{"no newlines", []byte("hello"), 0},
		{"one newline", []byte("hello\n"), 1},
		{"two newlines", []byte("hello\nworld\n"), 2},
		{"three newlines", []byte("line1\nline2\nline3\n"), 3},
		{"newlines only", []byte("\n\n\n"), 3},
		{"mixed content", []byte("a\nb\nc\n"), 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countNewlines(tt.input)
			if got != tt.want {
				t.Errorf("countNewlines(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestFinalizeIdempotent verifies Finalize can be called multiple times safely.
func TestFinalizeIdempotent(t *testing.T) {
	dingoSrc := []byte("x := foo()?\n")
	goSrc := []byte("tmp, err := foo()\nif err != nil {\n\treturn err\n}\nx := tmp\n")

	tracker := NewTransformTracker(dingoSrc)
	tracker.RecordTransform(0, 11, "error_prop", len(goSrc))

	// First finalize
	err := tracker.Finalize(goSrc)
	if err != nil {
		t.Fatalf("first Finalize() error = %v", err)
	}

	mappings1 := tracker.LineMappings()

	// Second finalize (should be no-op)
	err = tracker.Finalize(goSrc)
	if err != nil {
		t.Fatalf("second Finalize() error = %v", err)
	}

	mappings2 := tracker.LineMappings()

	// Mappings should be identical
	if len(mappings1) != len(mappings2) {
		t.Errorf("mappings length changed: %d -> %d", len(mappings1), len(mappings2))
	}
}

// TestFinalizeEmptyTransforms verifies behavior with no transforms.
func TestFinalizeEmptyTransforms(t *testing.T) {
	dingoSrc := []byte("x := 42\n")
	goSrc := []byte("x := 42\n")

	tracker := NewTransformTracker(dingoSrc)
	// No RecordTransform calls

	err := tracker.Finalize(goSrc)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	mappings := tracker.LineMappings()
	if len(mappings) != 0 {
		t.Errorf("lineMappings length = %d, want 0 (no transforms)", len(mappings))
	}
}

// TestMultipleTransformsOnSameLine verifies multiple transforms on same Dingo line.
func TestMultipleTransformsOnSameLine(t *testing.T) {
	dingoSrc := []byte("x := foo()?.bar()?\n")
	// First transform: foo()? (4 newlines = 5 lines)
	goStep1 := "tmp1, err := foo()\nif err != nil {\n\treturn err\n}\n"
	// Second transform: .bar()? (4 newlines = 5 lines)
	goStep2 := "tmp2, err := tmp1.bar()\nif err != nil {\n\treturn err\n}\n"
	goSrc := []byte(goStep1 + goStep2 + "x := tmp2\n")

	tracker := NewTransformTracker(dingoSrc)
	tracker.RecordTransform(5, 11, "error_prop", len(goStep1)) // foo()?
	tracker.RecordTransform(11, 18, "error_prop", len(goStep2)) // .bar()?

	err := tracker.Finalize(goSrc)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	mappings := tracker.LineMappings()
	if len(mappings) != 2 {
		t.Fatalf("lineMappings length = %d, want 2", len(mappings))
	}

	// Both transforms on same Dingo line 1
	if mappings[0].DingoLine != 1 {
		t.Errorf("mappings[0].DingoLine = %d, want 1", mappings[0].DingoLine)
	}
	if mappings[1].DingoLine != 1 {
		t.Errorf("mappings[1].DingoLine = %d, want 1", mappings[1].DingoLine)
	}

	// But different Go line ranges
	// First transform: 4 newlines + 1 = 5 lines (lines 1-5)
	if mappings[0].GoLineStart != 1 {
		t.Errorf("mappings[0].GoLineStart = %d, want 1", mappings[0].GoLineStart)
	}
	if mappings[0].GoLineEnd != 5 {
		t.Errorf("mappings[0].GoLineEnd = %d, want 5", mappings[0].GoLineEnd)
	}
	// Second transform: starts at line 6 (1 + 5), 4 newlines + 1 = 5 lines (lines 6-10)
	// But wait: dingoLine=1, cumulativeDelta after first = 5-1 = 4
	// So: goStartLine = 1 + 4 = 5 (not 6!)
	if mappings[1].GoLineStart != 5 {
		t.Errorf("mappings[1].GoLineStart = %d, want 5", mappings[1].GoLineStart)
	}
	if mappings[1].GoLineEnd != 9 {
		t.Errorf("mappings[1].GoLineEnd = %d, want 9", mappings[1].GoLineEnd)
	}
}

// TestDifferentTransformKinds verifies tracking different transform types.
func TestDifferentTransformKinds(t *testing.T) {
	dingoSrc := []byte("x := foo()?\ny := obj?.method()\n")
	goErrorProp := "tmp, err := foo()\nif err != nil {\n\treturn err\n}\nx := tmp\n"
	goSafeNav := "var y interface{}\nif obj != nil {\n\ty = obj.method()\n}\n"
	goSrc := []byte(goErrorProp + goSafeNav)

	tracker := NewTransformTracker(dingoSrc)
	// Ranges INCLUDE the trailing newline to cover entire lines
	tracker.RecordTransform(0, 12, "error_prop", len(goErrorProp))  // includes \n
	tracker.RecordTransform(12, 32, "safe_nav", len(goSafeNav))     // includes \n

	err := tracker.Finalize(goSrc)
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}

	mappings := tracker.LineMappings()
	if len(mappings) != 2 {
		t.Fatalf("lineMappings length = %d, want 2", len(mappings))
	}

	if mappings[0].Kind != "error_prop" {
		t.Errorf("mappings[0].Kind = %q, want %q", mappings[0].Kind, "error_prop")
	}
	if mappings[1].Kind != "safe_nav" {
		t.Errorf("mappings[1].Kind = %q, want %q", mappings[1].Kind, "safe_nav")
	}
}
