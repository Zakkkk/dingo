package lsp

// LogEntry represents a structured log entry with all fields
type LogEntry struct {
	Timestamp int64  // Unix timestamp in milliseconds
	Level     string // D/I/W/E
	Component string // ide/gpl/trp/smap/hdl/watch/init/lint
	Method    string // LSP method code: C/H/D/R/F/A/S/O/X/T/P/G/I/SD
	Direction string // >/</=/!/> g/<g
	RequestID int64  // For correlation
	File      string // Filename (no path)
	Line      int    // Line number
	Col       int    // Column number
	DingoPos  string // "L:C" format
	GoPos     string // "L:C" format
	Duration  int64  // Milliseconds
	Message   string // Log message
	Error     string // Error details if any
	Context   string // JSON context blob
}

// StructuredLogger extends Logger with structured field methods
type StructuredLogger interface {
	Logger

	// WithFields returns a new logger with preset fields
	WithFields(entry LogEntry) StructuredLogger

	// WithComponent sets the component field
	WithComponent(comp string) StructuredLogger

	// WithMethod sets the LSP method code
	WithMethod(method string) StructuredLogger

	// WithRequestID sets the request ID for correlation
	WithRequestID(rid int64) StructuredLogger

	// WithPosition sets file/line/col
	WithPosition(file string, line, col int) StructuredLogger

	// WithDuration sets the duration in ms
	WithDuration(dur int64) StructuredLogger

	// WithTranslation sets dpos/gpos for position translation debugging
	WithTranslation(dingoPos, goPos string) StructuredLogger

	// LogEntry writes a fully structured entry
	LogEntry(entry LogEntry)
}

// Component codes
const (
	CompIDE   = "ide"   // IDE connection handling
	CompGopls = "gpl"   // gopls communication
	CompTrans = "trp"   // Transpiler
	CompSmap  = "smap"  // Source map cache
	CompHdl   = "hdl"   // LSP handlers
	CompWatch = "watch" // File watcher
	CompInit  = "init"  // Initialization
	CompLint  = "lint"  // Linter
)

// LSP Method codes
const (
	MthCompletion  = "C"
	MthHover       = "H"
	MthDefinition  = "D"
	MthReferences  = "R"
	MthFormatting  = "F"
	MthCodeAction  = "A"
	MthDocSymbol   = "S"
	MthDidOpen     = "O"
	MthDidClose    = "X"
	MthDidChange   = "T"
	MthDiagnostics = "P"
	MthInitialize  = "G"
	MthInitialized = "I"
	MthDidSave     = "SD"
)

// Direction codes
const (
	DirFromIDE   = ">"  // Request from IDE
	DirToIDE     = "<"  // Response to IDE
	DirInternal  = "="  // Internal event
	DirError     = "!"  // Error
	DirToGopls   = ">g" // Forward to gopls
	DirFromGopls = "<g" // Response from gopls
)
