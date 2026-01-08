package typechecker

import (
	"testing"
)

func TestNewEnumRegistry(t *testing.T) {
	r := NewEnumRegistry()
	if r == nil {
		t.Fatal("NewEnumRegistry returned nil")
	}
	if r.Size() != 0 {
		t.Errorf("new registry should be empty, got size %d", r.Size())
	}
	if r.VariantCount() != 0 {
		t.Errorf("new registry should have 0 variants, got %d", r.VariantCount())
	}
}

func TestRegisterEnum_SingleEnum(t *testing.T) {
	r := NewEnumRegistry()

	variants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint"},
		{Name: "Circle", FullName: "ShapeCircle", Fields: []string{"radius"}, FieldTypes: []string{"f64"}},
		{Name: "Rectangle", FullName: "ShapeRectangle", Fields: []string{"width", "height"}, FieldTypes: []string{"f64", "f64"}},
	}

	if err := r.RegisterEnum("Shape", variants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	// Verify enum is registered
	info := r.GetEnum("Shape")
	if info == nil {
		t.Fatal("GetEnum(Shape) returned nil")
	}
	if info.Name != "Shape" {
		t.Errorf("expected Name=Shape, got %s", info.Name)
	}
	if len(info.Variants) != 3 {
		t.Errorf("expected 3 variants, got %d", len(info.Variants))
	}

	// Verify size
	if r.Size() != 1 {
		t.Errorf("expected size 1, got %d", r.Size())
	}
	if r.VariantCount() != 3 {
		t.Errorf("expected 3 variants, got %d", r.VariantCount())
	}
}

func TestRegisterEnum_MultipleEnums(t *testing.T) {
	r := NewEnumRegistry()

	// Register Shape enum
	shapeVariants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint"},
		{Name: "Circle", FullName: "ShapeCircle"},
	}
	if err := r.RegisterEnum("Shape", shapeVariants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	// Register Color enum
	colorVariants := []VariantInfo{
		{Name: "Red", FullName: "ColorRed"},
		{Name: "Green", FullName: "ColorGreen"},
		{Name: "Blue", FullName: "ColorBlue"},
	}
	if err := r.RegisterEnum("Color", colorVariants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	// Verify both enums are registered
	if r.Size() != 2 {
		t.Errorf("expected 2 enums, got %d", r.Size())
	}
	if r.VariantCount() != 5 {
		t.Errorf("expected 5 variants, got %d", r.VariantCount())
	}

	// Verify Shape enum
	shape := r.GetEnum("Shape")
	if shape == nil || shape.Name != "Shape" || len(shape.Variants) != 2 {
		t.Error("Shape enum not registered correctly")
	}

	// Verify Color enum
	color := r.GetEnum("Color")
	if color == nil || color.Name != "Color" || len(color.Variants) != 3 {
		t.Error("Color enum not registered correctly")
	}
}

func TestGetEnumForVariant(t *testing.T) {
	r := NewEnumRegistry()

	variants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint"},
		{Name: "Circle", FullName: "ShapeCircle"},
	}
	if err := r.RegisterEnum("Shape", variants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	tests := []struct {
		variantName string
		wantEnum    string
		wantNil     bool
	}{
		{"Point", "Shape", false},
		{"Circle", "Shape", false},
		{"Unknown", "", true},
		{"ShapePoint", "", true}, // Full name, not variant name
	}

	for _, tt := range tests {
		t.Run(tt.variantName, func(t *testing.T) {
			info := r.GetEnumForVariant(tt.variantName)
			if tt.wantNil {
				if info != nil {
					t.Errorf("GetEnumForVariant(%s) should return nil, got %v", tt.variantName, info)
				}
			} else {
				if info == nil {
					t.Fatalf("GetEnumForVariant(%s) returned nil", tt.variantName)
				}
				if info.Name != tt.wantEnum {
					t.Errorf("GetEnumForVariant(%s) = %s, want %s", tt.variantName, info.Name, tt.wantEnum)
				}
			}
		})
	}
}

func TestGetEnumForFullName(t *testing.T) {
	r := NewEnumRegistry()

	variants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint"},
		{Name: "Circle", FullName: "ShapeCircle"},
	}
	if err := r.RegisterEnum("Shape", variants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	tests := []struct {
		fullName string
		wantEnum string
		wantNil  bool
	}{
		{"ShapePoint", "Shape", false},
		{"ShapeCircle", "Shape", false},
		{"Point", "", true},       // Variant name, not full name
		{"Unknown", "", true},     // Not registered
		{"ShapeSquare", "", true}, // Not a variant of Shape
	}

	for _, tt := range tests {
		t.Run(tt.fullName, func(t *testing.T) {
			info := r.GetEnumForFullName(tt.fullName)
			if tt.wantNil {
				if info != nil {
					t.Errorf("GetEnumForFullName(%s) should return nil, got %v", tt.fullName, info)
				}
			} else {
				if info == nil {
					t.Fatalf("GetEnumForFullName(%s) returned nil", tt.fullName)
				}
				if info.Name != tt.wantEnum {
					t.Errorf("GetEnumForFullName(%s) = %s, want %s", tt.fullName, info.Name, tt.wantEnum)
				}
			}
		})
	}
}

func TestGetAllVariants(t *testing.T) {
	r := NewEnumRegistry()

	variants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint"},
		{Name: "Circle", FullName: "ShapeCircle"},
		{Name: "Rectangle", FullName: "ShapeRectangle"},
	}
	if err := r.RegisterEnum("Shape", variants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	result := r.GetAllVariants("Shape")
	if len(result) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(result))
	}

	expected := []string{"Point", "Circle", "Rectangle"}
	for i, want := range expected {
		if result[i] != want {
			t.Errorf("variants[%d] = %s, want %s", i, result[i], want)
		}
	}

	// Test unknown enum
	unknown := r.GetAllVariants("Unknown")
	if unknown != nil {
		t.Errorf("GetAllVariants(Unknown) should return nil, got %v", unknown)
	}
}

func TestNormalizePatternName_PascalCase(t *testing.T) {
	r := NewEnumRegistry()

	variants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint"},
		{Name: "Circle", FullName: "ShapeCircle"},
	}
	if err := r.RegisterEnum("Shape", variants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	tests := []struct {
		pattern     string
		wantEnum    string
		wantVariant string
		wantOk      bool
	}{
		// Full name lookup
		{"ShapePoint", "Shape", "Point", true},
		{"ShapeCircle", "Shape", "Circle", true},

		// Variant name lookup
		{"Point", "Shape", "Point", true},
		{"Circle", "Shape", "Circle", true},

		// Unknown patterns
		{"Unknown", "", "", false},
		{"ShapeSquare", "", "", false},

		// Invalid patterns
		{"point", "", "", false},       // Not PascalCase (lowercase)
		{"", "", "", false},            // Empty
		{"Shape_Point", "", "", false}, // Underscore (deprecated)
		{"_Point", "", "", false},      // Leading underscore
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			enumName, variantName, ok := r.NormalizePatternName(tt.pattern)
			if ok != tt.wantOk {
				t.Errorf("NormalizePatternName(%s) ok = %v, want %v", tt.pattern, ok, tt.wantOk)
			}
			if enumName != tt.wantEnum {
				t.Errorf("NormalizePatternName(%s) enumName = %s, want %s", tt.pattern, enumName, tt.wantEnum)
			}
			if variantName != tt.wantVariant {
				t.Errorf("NormalizePatternName(%s) variantName = %s, want %s", tt.pattern, variantName, tt.wantVariant)
			}
		})
	}
}

func TestNormalizePatternName_RejectUnderscore(t *testing.T) {
	r := NewEnumRegistry()

	variants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint"},
	}
	if err := r.RegisterEnum("Shape", variants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	// Underscore syntax should be rejected
	underscorePatterns := []string{
		"Shape_Point",
		"_Point",
		"Point_",
		"Some_Long_Name",
	}

	for _, pattern := range underscorePatterns {
		t.Run(pattern, func(t *testing.T) {
			enumName, variantName, ok := r.NormalizePatternName(pattern)
			if ok {
				t.Errorf("NormalizePatternName(%s) should reject underscore syntax, got ok=true", pattern)
			}
			if enumName != "" {
				t.Errorf("NormalizePatternName(%s) should return empty enumName, got %s", pattern, enumName)
			}
			if variantName != "" {
				t.Errorf("NormalizePatternName(%s) should return empty variantName, got %s", pattern, variantName)
			}
		})
	}
}

func TestValidatePatternName(t *testing.T) {
	r := NewEnumRegistry()

	variants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint"},
		{Name: "Circle", FullName: "ShapeCircle"},
	}
	if err := r.RegisterEnum("Shape", variants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	tests := []struct {
		pattern   string
		wantError bool
		errorMsg  string
	}{
		{"ShapePoint", false, ""},
		{"Point", false, ""},
		{"Circle", false, ""},

		// Underscore syntax
		{"Shape_Point", true, "deprecated pattern syntax"},
		{"_Point", true, "deprecated pattern syntax"},

		// Not PascalCase
		{"point", true, "must be PascalCase"},
		{"shapePoint", true, "must be PascalCase"},

		// Unknown
		{"Unknown", true, "unknown pattern name"},

		// Empty
		{"", true, "cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			err := r.ValidatePatternName(tt.pattern)
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidatePatternName(%s) should return error, got nil", tt.pattern)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidatePatternName(%s) error = %q, should contain %q", tt.pattern, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePatternName(%s) should return nil, got %v", tt.pattern, err)
				}
			}
		})
	}
}

func TestClone(t *testing.T) {
	r := NewEnumRegistry()

	variants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint", Fields: []string{"x", "y"}, FieldTypes: []string{"int", "int"}},
		{Name: "Circle", FullName: "ShapeCircle", Fields: []string{"radius"}, FieldTypes: []string{"f64"}},
	}
	if err := r.RegisterEnum("Shape", variants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	// Clone the registry
	clone := r.Clone()

	// Verify clone has same data
	if clone.Size() != r.Size() {
		t.Errorf("clone size = %d, want %d", clone.Size(), r.Size())
	}
	if clone.VariantCount() != r.VariantCount() {
		t.Errorf("clone variant count = %d, want %d", clone.VariantCount(), r.VariantCount())
	}

	// Verify deep copy (modifying clone doesn't affect original)
	cloneInfo := clone.GetEnum("Shape")
	if cloneInfo == nil {
		t.Fatal("clone GetEnum(Shape) returned nil")
	}

	// Modify clone
	newVariants := []VariantInfo{
		{Name: "Square", FullName: "ShapeSquare"},
	}
	if err := clone.RegisterEnum("NewEnum", newVariants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	// Original should be unchanged
	if r.Size() != 1 {
		t.Errorf("original size changed after clone modification, got %d", r.Size())
	}
	if clone.Size() != 2 {
		t.Errorf("clone size = %d, want 2 after adding enum", clone.Size())
	}
}

func TestVariantInfo_FieldsAndTypes(t *testing.T) {
	r := NewEnumRegistry()

	variants := []VariantInfo{
		{
			Name:       "Point",
			FullName:   "ShapePoint",
			Fields:     []string{"x", "y"},
			FieldTypes: []string{"int", "int"},
		},
		{
			Name:       "Circle",
			FullName:   "ShapeCircle",
			Fields:     []string{"radius"},
			FieldTypes: []string{"f64"},
		},
		{
			Name:     "Unit",
			FullName: "ShapeUnit",
			// No fields
		},
	}
	if err := r.RegisterEnum("Shape", variants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	info := r.GetEnum("Shape")
	if info == nil {
		t.Fatal("GetEnum(Shape) returned nil")
	}

	// Verify Point variant
	point := info.Variants[0]
	if point.Name != "Point" {
		t.Errorf("variant 0 name = %s, want Point", point.Name)
	}
	if len(point.Fields) != 2 {
		t.Errorf("Point fields count = %d, want 2", len(point.Fields))
	}
	if len(point.FieldTypes) != 2 {
		t.Errorf("Point field types count = %d, want 2", len(point.FieldTypes))
	}

	// Verify Circle variant
	circle := info.Variants[1]
	if circle.Name != "Circle" {
		t.Errorf("variant 1 name = %s, want Circle", circle.Name)
	}
	if len(circle.Fields) != 1 {
		t.Errorf("Circle fields count = %d, want 1", len(circle.Fields))
	}

	// Verify Unit variant (no fields)
	unit := info.Variants[2]
	if unit.Name != "Unit" {
		t.Errorf("variant 2 name = %s, want Unit", unit.Name)
	}
	if len(unit.Fields) != 0 {
		t.Errorf("Unit fields count = %d, want 0", len(unit.Fields))
	}
	if len(unit.FieldTypes) != 0 {
		t.Errorf("Unit field types count = %d, want 0", len(unit.FieldTypes))
	}
}

func TestMultipleEnums_VariantIsolation(t *testing.T) {
	r := NewEnumRegistry()

	// Register Shape enum
	shapeVariants := []VariantInfo{
		{Name: "Point", FullName: "ShapePoint"},
	}
	if err := r.RegisterEnum("Shape", shapeVariants); err != nil {
		t.Fatalf("RegisterEnum failed: %v", err)
	}

	// Attempt to register Event enum with same "Point" variant - should fail
	eventVariants := []VariantInfo{
		{Name: "Point", FullName: "EventPoint"}, // Same variant name, different enum
	}
	err := r.RegisterEnum("Event", eventVariants)
	if err == nil {
		t.Fatal("expected error for variant name collision, got nil")
	}
	// Verify error message mentions collision
	expectedMsg := "variant name collision"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error message = %q, want to contain %q", err.Error(), expectedMsg)
	}

	// GetEnumForVariant("Point") should still return Shape (first registration succeeded)
	info := r.GetEnumForVariant("Point")
	if info == nil {
		t.Fatal("GetEnumForVariant(Point) returned nil")
	}
	if info.Name != "Shape" {
		t.Errorf("GetEnumForVariant(Point) returned %s, want Shape", info.Name)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
