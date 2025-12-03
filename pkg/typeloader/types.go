package typeloader

// LoaderConfig configures the type loader
type LoaderConfig struct {
	// WorkingDir is the directory context for go/packages
	WorkingDir string

	// BuildTags are build tags to consider when loading packages
	BuildTags []string

	// FailFast stops on first package load error (default: true)
	FailFast bool
}

// LoadResult contains the result of loading package types
type LoadResult struct {
	// Functions contains all exported functions by qualified name
	// Key format: "package.Function" (e.g., "os.Open", "json.Marshal")
	Functions map[string]*FunctionSignature

	// Methods contains all exported methods by receiver type
	// Key format: "Type.Method" (e.g., "File.Close", "Rows.Scan")
	Methods map[string]*FunctionSignature

	// LocalFunctions contains functions defined in the same package
	// Key format: "FunctionName" (e.g., "getUserData", "processFile")
	LocalFunctions map[string]*FunctionSignature
}

// FunctionSignature represents a function or method signature
type FunctionSignature struct {
	Name       string
	Package    string    // Full import path (empty for local functions)
	Receiver   *TypeRef  // nil for functions
	Parameters []TypeRef
	Results    []TypeRef
}

// TypeRef represents a type reference
type TypeRef struct {
	Name      string
	Package   string
	IsPointer bool
	IsError   bool // True if this is the error interface
}
