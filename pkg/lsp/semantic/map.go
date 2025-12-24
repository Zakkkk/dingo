package semantic

import (
	"sort"
)

// Map provides O(log n) lookup of semantic entities by position
type Map struct {
	// Entities sorted by line, then by column
	entities []SemanticEntity

	// Line index for fast lookup: line -> slice indices
	lineIndex map[int][]int
}

// entityPriority returns priority for deduplication (higher = preferred)
// KindType has highest priority because it represents instantiated generics
func entityPriority(kind SemanticKind) int {
	switch kind {
	case KindType:
		return 100 // Highest: instantiated generic types like Result[User, DBError]
	case KindOperator:
		return 90 // Dingo operators
	case KindLambda:
		return 80 // Lambda parameters
	case KindCall:
		return 70 // Function calls
	case KindField:
		return 60 // Field access
	case KindIdent:
		return 50 // Regular identifiers (lowest among named entities)
	default:
		return 0
	}
}

// NewMap creates a Map from a slice of entities
// Entities are sorted by line, then column for efficient binary search
// When entities overlap, prefers KindType (instantiated generics) over KindIdent
func NewMap(entities []SemanticEntity) *Map {
	if len(entities) == 0 {
		return &Map{
			entities:  []SemanticEntity{},
			lineIndex: make(map[int][]int),
		}
	}

	// Make a copy to avoid mutating input
	sorted := make([]SemanticEntity, len(entities))
	copy(sorted, entities)

	// Sort by line, then by column, then by kind priority
	// KindType (instantiated generics) should come before KindIdent
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Line != sorted[j].Line {
			return sorted[i].Line < sorted[j].Line
		}
		if sorted[i].Col != sorted[j].Col {
			return sorted[i].Col < sorted[j].Col
		}
		// Same position: prefer KindType over KindIdent (higher priority first)
		return entityPriority(sorted[i].Kind) > entityPriority(sorted[j].Kind)
	})

	// Deduplicate: when entities have same Line and Col, keep highest priority
	deduped := make([]SemanticEntity, 0, len(sorted))
	for i, e := range sorted {
		if i == 0 {
			deduped = append(deduped, e)
			continue
		}
		prev := deduped[len(deduped)-1]
		// If same position, skip (we already have higher priority one)
		if e.Line == prev.Line && e.Col == prev.Col {
			continue
		}
		deduped = append(deduped, e)
	}
	sorted = deduped

	// Build line index
	lineIndex := make(map[int][]int)
	for i, entity := range sorted {
		lineIndex[entity.Line] = append(lineIndex[entity.Line], i)
	}

	return &Map{
		entities:  sorted,
		lineIndex: lineIndex,
	}
}

// FindAt returns the entity containing the position, or nil
// Position is 1-indexed (Dingo source coordinates)
// For operators like ?, this returns the operator entity
// Uses binary search for O(log n) lookup per line
func (m *Map) FindAt(line, col int) *SemanticEntity {
	// Get entities on this line
	indices, ok := m.lineIndex[line]
	if !ok {
		return nil
	}

	// Binary search: find first entity where Col > col
	// Then check if the previous entity contains col
	i := sort.Search(len(indices), func(i int) bool {
		return m.entities[indices[i]].Col > col
	})

	// Check entity at i-1 (last entity with Col <= col)
	if i > 0 {
		entity := &m.entities[indices[i-1]]
		// EndCol is exclusive (half-open range: [Col, EndCol))
		if col >= entity.Col && col < entity.EndCol {
			return entity
		}
	}

	// Also check entity at i (in case of exact match at start)
	if i < len(indices) {
		entity := &m.entities[indices[i]]
		if col >= entity.Col && col < entity.EndCol {
			return entity
		}
	}

	return nil
}

// FindNearest returns entity at position or nearest within maxDistance
// Distance is measured in columns only (same line)
// Returns nil if no entity within maxDistance
func (m *Map) FindNearest(line, col, maxDistance int) *SemanticEntity {
	// First try exact match
	if entity := m.FindAt(line, col); entity != nil {
		return entity
	}

	// Get entities on this line
	indices, ok := m.lineIndex[line]
	if !ok {
		return nil
	}

	// Find nearest entity within maxDistance
	var nearest *SemanticEntity
	minDist := maxDistance + 1

	for _, idx := range indices {
		entity := &m.entities[idx]

		// Calculate distance to entity (EndCol is exclusive)
		dist := 0
		if col < entity.Col {
			dist = entity.Col - col
		} else if col >= entity.EndCol {
			// col is at or past end (EndCol is exclusive, so >= is correct)
			dist = col - entity.EndCol + 1
		} else {
			// Point is within entity (shouldn't happen since FindAt failed)
			return entity
		}

		// Update nearest if closer
		if dist < minDist {
			minDist = dist
			nearest = entity
		}
	}

	return nearest
}

// Count returns the number of entities in the map
func (m *Map) Count() int {
	return len(m.entities)
}

// EntityAt returns the entity at the given index, or nil if out of bounds
func (m *Map) EntityAt(i int) *SemanticEntity {
	if i < 0 || i >= len(m.entities) {
		return nil
	}
	return &m.entities[i]
}
