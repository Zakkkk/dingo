package semantic

import (
	"testing"
)

func TestNewMap_Empty(t *testing.T) {
	m := NewMap(nil)
	if m.Count() != 0 {
		t.Errorf("Expected empty map, got %d entities", m.Count())
	}

	if entity := m.FindAt(1, 1); entity != nil {
		t.Error("Expected nil for empty map, got entity")
	}
}

func TestNewMap_Sorts(t *testing.T) {
	entities := []SemanticEntity{
		{Line: 2, Col: 5, EndCol: 10, Kind: KindIdent},
		{Line: 1, Col: 1, EndCol: 5, Kind: KindIdent},
		{Line: 1, Col: 10, EndCol: 15, Kind: KindCall},
		{Line: 3, Col: 1, EndCol: 3, Kind: KindOperator},
	}

	m := NewMap(entities)

	// Verify sorting: line 1, col 1 should be first
	if m.entities[0].Line != 1 || m.entities[0].Col != 1 {
		t.Errorf("First entity should be line 1, col 1, got line %d, col %d",
			m.entities[0].Line, m.entities[0].Col)
	}

	// Verify line 1, col 10 is second
	if m.entities[1].Line != 1 || m.entities[1].Col != 10 {
		t.Errorf("Second entity should be line 1, col 10, got line %d, col %d",
			m.entities[1].Line, m.entities[1].Col)
	}

	// Verify line 2 is third
	if m.entities[2].Line != 2 {
		t.Errorf("Third entity should be line 2, got line %d", m.entities[2].Line)
	}
}

func TestFindAt_ExactMatch(t *testing.T) {
	// EndCol is exclusive (half-open range: [Col, EndCol))
	// Entity at Col: 5, EndCol: 11 spans columns 5, 6, 7, 8, 9, 10
	entities := []SemanticEntity{
		{Line: 1, Col: 5, EndCol: 11, Kind: KindIdent},   // 6 chars wide
		{Line: 1, Col: 15, EndCol: 21, Kind: KindCall},   // 6 chars wide
		{Line: 2, Col: 1, EndCol: 6, Kind: KindOperator}, // 5 chars wide
	}

	m := NewMap(entities)

	tests := []struct {
		name     string
		line     int
		col      int
		wantKind *SemanticKind
	}{
		{
			name:     "start of entity",
			line:     1,
			col:      5,
			wantKind: ptr(KindIdent),
		},
		{
			name:     "middle of entity",
			line:     1,
			col:      7,
			wantKind: ptr(KindIdent),
		},
		{
			name:     "end of entity",
			line:     1,
			col:      10,
			wantKind: ptr(KindIdent),
		},
		{
			name:     "second entity on same line",
			line:     1,
			col:      17,
			wantKind: ptr(KindCall),
		},
		{
			name:     "different line",
			line:     2,
			col:      3,
			wantKind: ptr(KindOperator),
		},
		{
			name:     "no match - before entity",
			line:     1,
			col:      3,
			wantKind: nil,
		},
		{
			name:     "no match - between entities",
			line:     1,
			col:      12,
			wantKind: nil,
		},
		{
			name:     "no match - nonexistent line",
			line:     5,
			col:      1,
			wantKind: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := m.FindAt(tt.line, tt.col)

			if tt.wantKind == nil {
				if entity != nil {
					t.Errorf("Expected nil, got entity with kind %v", entity.Kind)
				}
			} else {
				if entity == nil {
					t.Errorf("Expected entity with kind %v, got nil", *tt.wantKind)
				} else if entity.Kind != *tt.wantKind {
					t.Errorf("Expected kind %v, got %v", *tt.wantKind, entity.Kind)
				}
			}
		})
	}
}

func TestFindAt_SingleCharOperator(t *testing.T) {
	// EndCol is exclusive (half-open range: [Col, EndCol))
	// Single char '?' at col 10 has EndCol: 11 (spans only col 10)
	// Two char '??' at col 15 has EndCol: 17 (spans cols 15, 16)
	entities := []SemanticEntity{
		{Line: 1, Col: 10, EndCol: 11, Kind: KindOperator}, // Single char ?
		{Line: 1, Col: 15, EndCol: 17, Kind: KindOperator}, // Two char ??
	}

	m := NewMap(entities)

	// Should find single-char operator at exact position
	entity := m.FindAt(1, 10)
	if entity == nil || entity.Kind != KindOperator {
		t.Errorf("Expected to find operator at col 10")
	}

	// Should NOT find single-char operator at col 11 (EndCol is exclusive)
	entity = m.FindAt(1, 11)
	if entity != nil {
		t.Errorf("Should not find operator at col 11 (exclusive EndCol)")
	}

	// Should find two-char operator
	entity = m.FindAt(1, 15)
	if entity == nil || entity.Kind != KindOperator {
		t.Errorf("Expected to find operator at col 15")
	}

	entity = m.FindAt(1, 16)
	if entity == nil || entity.Kind != KindOperator {
		t.Errorf("Expected to find operator at col 16")
	}

	// Should NOT find two-char operator at col 17 (EndCol is exclusive)
	entity = m.FindAt(1, 17)
	if entity != nil {
		t.Errorf("Should not find operator at col 17 (exclusive EndCol)")
	}
}

func TestFindNearest(t *testing.T) {
	// EndCol is exclusive (half-open range: [Col, EndCol))
	// Ident at [10,15) spans cols 10-14, Call at [20,25) spans cols 20-24
	entities := []SemanticEntity{
		{Line: 1, Col: 10, EndCol: 15, Kind: KindIdent}, // cols 10-14
		{Line: 1, Col: 20, EndCol: 25, Kind: KindCall},  // cols 20-24
		{Line: 2, Col: 5, EndCol: 8, Kind: KindField},   // cols 5-7
	}

	m := NewMap(entities)

	tests := []struct {
		name        string
		line        int
		col         int
		maxDistance int
		wantKind    *SemanticKind
	}{
		{
			name:        "exact match returns immediately",
			line:        1,
			col:         12, // within [10,15)
			maxDistance: 5,
			wantKind:    ptr(KindIdent),
		},
		{
			name:        "nearest within distance - before",
			line:        1,
			col:         8, // 2 columns before 10
			maxDistance: 3,
			wantKind:    ptr(KindIdent),
		},
		{
			name:        "nearest within distance - after",
			line:        1,
			col:         17, // 3 columns after 14 (last col of ident), 3 columns before 20 (first col of call)
			maxDistance: 5,  // dist = col - EndCol + 1 = 17 - 15 + 1 = 3 for ident, dist = 20 - 17 = 3 for call
			wantKind:    ptr(KindIdent), // Equidistant (both dist=3), ident wins because it's checked first
		},
		{
			name:        "too far - returns nil",
			line:        1,
			col:         8, // 2 columns before 10
			maxDistance: 1,
			wantKind:    nil,
		},
		{
			name:        "different line - no match",
			line:        3,
			col:         5,
			maxDistance: 10,
			wantKind:    nil,
		},
		{
			name:        "closest of two entities - closer to call",
			line:        1,
			col:         18, // 4 from ident end (18-15+1=4), 2 from call start (20-18=2)
			maxDistance: 5,
			wantKind:    ptr(KindCall), // Call is closer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := m.FindNearest(tt.line, tt.col, tt.maxDistance)

			if tt.wantKind == nil {
				if entity != nil {
					t.Errorf("Expected nil, got entity with kind %v at col %d-%d",
						entity.Kind, entity.Col, entity.EndCol)
				}
			} else {
				if entity == nil {
					t.Errorf("Expected entity with kind %v, got nil", *tt.wantKind)
				} else if entity.Kind != *tt.wantKind {
					t.Errorf("Expected kind %v, got %v", *tt.wantKind, entity.Kind)
				}
			}
		})
	}
}

func TestFindNearest_EmptyMap(t *testing.T) {
	m := NewMap(nil)
	entity := m.FindNearest(1, 5, 10)
	if entity != nil {
		t.Errorf("Expected nil for empty map, got entity")
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		name     string
		entities []SemanticEntity
		want     int
	}{
		{
			name:     "empty",
			entities: nil,
			want:     0,
		},
		{
			name: "single entity",
			entities: []SemanticEntity{
				{Line: 1, Col: 1, EndCol: 5},
			},
			want: 1,
		},
		{
			name: "multiple entities",
			entities: []SemanticEntity{
				{Line: 1, Col: 1, EndCol: 5},
				{Line: 2, Col: 1, EndCol: 5},
				{Line: 3, Col: 1, EndCol: 5},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMap(tt.entities)
			if got := m.Count(); got != tt.want {
				t.Errorf("Count() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLineIndex(t *testing.T) {
	entities := []SemanticEntity{
		{Line: 1, Col: 1, EndCol: 5},
		{Line: 1, Col: 10, EndCol: 15},
		{Line: 1, Col: 20, EndCol: 25},
		{Line: 2, Col: 5, EndCol: 10},
		{Line: 3, Col: 1, EndCol: 3},
	}

	m := NewMap(entities)

	// Line 1 should have 3 entities
	if len(m.lineIndex[1]) != 3 {
		t.Errorf("Line 1 should have 3 entities, got %d", len(m.lineIndex[1]))
	}

	// Line 2 should have 1 entity
	if len(m.lineIndex[2]) != 1 {
		t.Errorf("Line 2 should have 1 entity, got %d", len(m.lineIndex[2]))
	}

	// Line 4 should not exist
	if _, exists := m.lineIndex[4]; exists {
		t.Error("Line 4 should not be in index")
	}
}

// Helper function to create pointer to value
func ptr(k SemanticKind) *SemanticKind {
	return &k
}
