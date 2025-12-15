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

// NewMap creates a Map from a slice of entities
// Entities are sorted by line, then column for efficient binary search
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

	// Sort by line, then by column
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Line != sorted[j].Line {
			return sorted[i].Line < sorted[j].Line
		}
		return sorted[i].Col < sorted[j].Col
	})

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
