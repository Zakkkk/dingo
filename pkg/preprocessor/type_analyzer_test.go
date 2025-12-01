package preprocessor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTypeAnalyzer_AnalyzeFile tests single-file analysis without package context
func TestTypeAnalyzer_AnalyzeFile(t *testing.T) {
	source := `package main

type User struct {
	Name    string
	Age     int
	Profile *Profile
}

type Profile struct {
	Bio string
}

func main() {
	var user *User
	var name string
	age := 42
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Test TypeOf for variables
	tests := []struct {
		name     string
		varName  string
		wantType string
	}{
		{"pointer variable", "user", "*User"},
		{"string variable", "name", "string"},
		{"int variable", "age", "int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, ok := ta.TypeOf(tt.varName)
			if !ok {
				t.Errorf("TypeOf(%s) not found", tt.varName)
				return
			}

			typeName := ta.TypeName(typ)
			// Normalize type names (remove package path)
			typeName = normalizeTypeName(typeName)

			if typeName != tt.wantType {
				t.Errorf("TypeOf(%s) = %s, want %s", tt.varName, typeName, tt.wantType)
			}
		})
	}
}

// TestTypeAnalyzer_IsPointer tests pointer type detection
func TestTypeAnalyzer_IsPointer(t *testing.T) {
	source := `package main

type User struct {
	Name string
}

func main() {
	var user *User
	var name string
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	tests := []struct {
		name       string
		varName    string
		wantPointer bool
	}{
		{"pointer type", "user", true},
		{"non-pointer type", "name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isPointer := ta.IsPointerByName(tt.varName)
			if isPointer != tt.wantPointer {
				t.Errorf("IsPointerByName(%s) = %v, want %v", tt.varName, isPointer, tt.wantPointer)
			}
		})
	}
}

// TestTypeAnalyzer_IsOption tests Option type detection by naming convention
func TestTypeAnalyzer_IsOption(t *testing.T) {
	source := `package main

type UserOption struct {
	isSome bool
	value  User
}

type User struct {
	Name string
}

type StringOption struct {
	isSome bool
	value  string
}

func main() {
	var userOpt UserOption
	var strOpt StringOption
	var user User
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	tests := []struct {
		name       string
		varName    string
		wantOption bool
	}{
		{"Option type (UserOption)", "userOpt", true},
		{"Option type (StringOption)", "strOpt", true},
		{"Non-Option type", "user", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isOption := ta.IsOptionByName(tt.varName)
			if isOption != tt.wantOption {
				t.Errorf("IsOptionByName(%s) = %v, want %v", tt.varName, isOption, tt.wantOption)
			}
		})
	}
}

// TestTypeAnalyzer_FieldType tests field type resolution
func TestTypeAnalyzer_FieldType(t *testing.T) {
	source := `package main

type User struct {
	Name    string
	Age     int
	Profile *Profile
}

type Profile struct {
	Bio string
}

func main() {
	var user User
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Get User type
	userType, ok := ta.TypeOf("user")
	if !ok {
		t.Fatalf("TypeOf(user) not found")
	}

	tests := []struct {
		name      string
		fieldName string
		wantType  string
		wantOK    bool
	}{
		{"string field", "Name", "string", true},
		{"int field", "Age", "int", true},
		{"pointer field", "Profile", "*Profile", true},
		{"non-existent field", "Unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldType, ok := ta.FieldType(userType, tt.fieldName)
			if ok != tt.wantOK {
				t.Errorf("FieldType(%s) ok = %v, want %v", tt.fieldName, ok, tt.wantOK)
				return
			}

			if ok {
				typeName := normalizeTypeName(ta.TypeName(fieldType))
				if typeName != tt.wantType {
					t.Errorf("FieldType(%s) = %s, want %s", tt.fieldName, typeName, tt.wantType)
				}
			}
		})
	}
}

// TestTypeAnalyzer_MethodReturnType tests method return type resolution
func TestTypeAnalyzer_MethodReturnType(t *testing.T) {
	source := `package main

type User struct {
	Name string
}

func (u *User) GetName() string {
	return u.Name
}

func (u *User) GetAge() int {
	return 42
}

func main() {
	var user *User
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Get User pointer type
	userType, ok := ta.TypeOf("user")
	if !ok {
		t.Fatalf("TypeOf(user) not found")
	}

	tests := []struct {
		name       string
		methodName string
		wantType   string
		wantOK     bool
	}{
		{"string return", "GetName", "string", true},
		{"int return", "GetAge", "int", true},
		{"non-existent method", "GetUnknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retType, ok := ta.MethodReturnType(userType, tt.methodName)
			if ok != tt.wantOK {
				t.Errorf("MethodReturnType(%s) ok = %v, want %v", tt.methodName, ok, tt.wantOK)
				return
			}

			if ok {
				typeName := normalizeTypeName(ta.TypeName(retType))
				if typeName != tt.wantType {
					t.Errorf("MethodReturnType(%s) = %s, want %s", tt.methodName, typeName, tt.wantType)
				}
			}
		})
	}
}

// TestTypeAnalyzer_ResolveChainType tests chain type resolution
func TestTypeAnalyzer_ResolveChainType(t *testing.T) {
	source := `package main

type User struct {
	Profile *Profile
}

type Profile struct {
	Settings Settings
}

type Settings struct {
	Theme string
}

func (u *User) GetProfile() *Profile {
	return u.Profile
}

func main() {
	var user *User
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	tests := []struct {
		name     string
		baseName string
		chain    []ChainElement
		wantType string
		wantOK   bool
	}{
		{
			name:     "field chain",
			baseName: "user",
			chain: []ChainElement{
				{Name: "Profile", IsMethod: false},
				{Name: "Settings", IsMethod: false},
				{Name: "Theme", IsMethod: false},
			},
			wantType: "string",
			wantOK:   true,
		},
		{
			name:     "method chain",
			baseName: "user",
			chain: []ChainElement{
				{Name: "GetProfile", IsMethod: true},
				{Name: "Settings", IsMethod: false},
			},
			wantType: "Settings",
			wantOK:   true,
		},
		{
			name:     "invalid chain",
			baseName: "user",
			chain: []ChainElement{
				{Name: "NonExistent", IsMethod: false},
			},
			wantType: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultType, ok := ta.ResolveChainType(tt.baseName, tt.chain)
			if ok != tt.wantOK {
				t.Errorf("ResolveChainType() ok = %v, want %v", ok, tt.wantOK)
				return
			}

			if ok {
				typeName := normalizeTypeName(ta.TypeName(resultType))
				if typeName != tt.wantType {
					t.Errorf("ResolveChainType() = %s, want %s", typeName, tt.wantType)
				}
			}
		})
	}
}

// TestTypeAnalyzer_AnalyzePackage tests package-level analysis
func TestTypeAnalyzer_AnalyzePackage(t *testing.T) {
	// Create temporary directory with test files
	tmpDir := t.TempDir()

	// Write main.go
	mainGo := `package main

type User struct {
	Name string
}

func main() {
	var user *User
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644)
	if err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Write helper.go
	helperGo := `package main

type Helper struct {
	Value int
}

func NewHelper() *Helper {
	return &Helper{Value: 42}
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "helper.go"), []byte(helperGo), 0644)
	if err != nil {
		t.Fatalf("Failed to write helper.go: %v", err)
	}

	// Write go.mod
	goMod := `module testpkg

go 1.21
`
	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Analyze the package
	ta := NewTypeAnalyzer()
	err = ta.AnalyzePackage(tmpDir)
	if err != nil {
		t.Fatalf("AnalyzePackage failed: %v", err)
	}

	// Test that types from both files are available
	tests := []struct {
		name    string
		varName string
		wantOK  bool
	}{
		{"type from main.go", "user", true},
		{"function from helper.go", "NewHelper", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := ta.TypeOf(tt.varName)
			if ok != tt.wantOK {
				t.Errorf("TypeOf(%s) ok = %v, want %v", tt.varName, ok, tt.wantOK)
			}
		})
	}
}

// TestTypeAnalyzer_PointerFieldAccess tests field access on pointer types
func TestTypeAnalyzer_PointerFieldAccess(t *testing.T) {
	source := `package main

type User struct {
	Profile *Profile
}

type Profile struct {
	Bio string
}

func main() {
	var user *User
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Get User pointer type
	userType, ok := ta.TypeOf("user")
	if !ok {
		t.Fatalf("TypeOf(user) not found")
	}

	// Should resolve field even though user is *User (pointer)
	profileType, ok := ta.FieldType(userType, "Profile")
	if !ok {
		t.Errorf("FieldType(Profile) should work on pointer receiver")
		return
	}

	typeName := normalizeTypeName(ta.TypeName(profileType))
	if typeName != "*Profile" {
		t.Errorf("FieldType(Profile) = %s, want *Profile", typeName)
	}
}

// TestTypeAnalyzer_EmptySource tests handling of empty/invalid source
func TestTypeAnalyzer_EmptySource(t *testing.T) {
	ta := NewTypeAnalyzer()

	// Empty source
	err := ta.AnalyzeFile("")
	if err == nil {
		t.Error("AnalyzeFile('') should return error for empty source")
	}

	// Invalid syntax
	err = ta.AnalyzeFile("this is not valid go code")
	if err == nil {
		t.Error("AnalyzeFile(invalid) should return error")
	}
}

// TestTypeAnalyzer_HasTypeInfo tests type info availability check
func TestTypeAnalyzer_HasTypeInfo(t *testing.T) {
	ta := NewTypeAnalyzer()

	// Before analysis
	if ta.HasTypeInfo() {
		t.Error("HasTypeInfo() should be false before analysis")
	}

	// After analysis
	source := `package main

func main() {}
`
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if !ta.HasTypeInfo() {
		t.Error("HasTypeInfo() should be true after successful analysis")
	}
}

// TestTypeAnalyzer_OptionTypePrefixSuffix tests Option type detection variants
func TestTypeAnalyzer_OptionTypePrefixSuffix(t *testing.T) {
	source := `package main

// Different Option naming conventions
type UserOption struct{}      // Suffix
type OptionString struct{}    // Prefix
type Option struct{}          // Exact
type NotAnOption struct{}     // No match

func main() {
	var userOpt UserOption
	var strOpt OptionString
	var opt Option
	var notOpt NotAnOption
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	tests := []struct {
		name       string
		varName    string
		wantOption bool
	}{
		{"suffix Option", "userOpt", true},
		{"prefix Option", "strOpt", true},
		{"exact Option", "opt", true},
		{"not an Option", "notOpt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isOption := ta.IsOptionByName(tt.varName)
			if isOption != tt.wantOption {
				t.Errorf("IsOptionByName(%s) = %v, want %v", tt.varName, isOption, tt.wantOption)
			}
		})
	}
}

// normalizeTypeName removes package path from type name for comparison
func normalizeTypeName(typeName string) string {
	// Handle pointer types: "*main.User" -> "*User"
	if strings.HasPrefix(typeName, "*") {
		// Recursively normalize the element type
		return "*" + normalizeTypeName(typeName[1:])
	}

	// Remove package path: "main.User" -> "User"
	if idx := strings.LastIndex(typeName, "."); idx != -1 {
		typeName = typeName[idx+1:]
	}
	return typeName
}

// TestTypeAnalyzer_TypeOf_NotFound tests TypeOf with non-existent identifier
func TestTypeAnalyzer_TypeOf_NotFound(t *testing.T) {
	source := `package main

func main() {
	var user string
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	_, ok := ta.TypeOf("nonExistent")
	if ok {
		t.Error("TypeOf(nonExistent) should return false")
	}
}

// TestTypeAnalyzer_IsPointer_DirectCall tests IsPointer with types.Type
func TestTypeAnalyzer_IsPointer_DirectCall(t *testing.T) {
	source := `package main

func main() {
	var user *string
	var name string
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	userType, _ := ta.TypeOf("user")
	nameType, _ := ta.TypeOf("name")

	if !ta.IsPointer(userType) {
		t.Error("IsPointer(user) should be true")
	}

	if ta.IsPointer(nameType) {
		t.Error("IsPointer(name) should be false")
	}
}

// TestTypeAnalyzer_IsOption_DirectCall tests IsOption with types.Type
func TestTypeAnalyzer_IsOption_DirectCall(t *testing.T) {
	source := `package main

type UserOption struct{}
type User struct{}

func main() {
	var userOpt UserOption
	var user User
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	userOptType, _ := ta.TypeOf("userOpt")
	userType, _ := ta.TypeOf("user")

	if !ta.IsOption(userOptType) {
		t.Error("IsOption(UserOption) should be true")
	}

	if ta.IsOption(userType) {
		t.Error("IsOption(User) should be false")
	}
}

// TestTypeAnalyzer_MethodReturnType_NoReturn tests method with no return type
func TestTypeAnalyzer_MethodReturnType_NoReturn(t *testing.T) {
	source := `package main

type User struct {
	Name string
}

func (u *User) SetName(name string) {
	u.Name = name
}

func main() {
	var user *User
}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	userType, ok := ta.TypeOf("user")
	if !ok {
		t.Fatalf("TypeOf(user) not found")
	}

	// Method with no return type should return false
	_, ok = ta.MethodReturnType(userType, "SetName")
	if ok {
		t.Error("MethodReturnType(SetName) should return false for void method")
	}
}

// TestTypeAnalyzer_PackagePath tests package path retrieval
func TestTypeAnalyzer_PackagePath(t *testing.T) {
	source := `package main

func main() {}
`

	ta := NewTypeAnalyzer()
	err := ta.AnalyzeFile(source)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	pkgPath := ta.PackagePath()
	if pkgPath != "main" {
		t.Errorf("PackagePath() = %s, want main", pkgPath)
	}
}
