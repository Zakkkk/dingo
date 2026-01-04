package format

import (
	"bytes"
	"fmt"
	"os"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// Formatter formats Dingo source code
type Formatter struct {
	Config *Config
}

// New creates a new formatter with the given configuration
func New(cfg *Config) *Formatter {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Formatter{Config: cfg}
}

// Format formats the given Dingo source code
// Returns the formatted source or an error
func (f *Formatter) Format(src []byte) ([]byte, error) {
	// Validate input
	if src == nil {
		return nil, fmt.Errorf("source is nil")
	}
	if len(src) == 0 {
		return []byte{}, nil // Empty input returns empty output
	}

	// Create tokenizer
	tok := tokenizer.New(src)

	// Tokenize entire source
	tokens, err := tok.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	// Create writer
	var out bytes.Buffer
	writer := newWriter(&out, f.Config, src)

	// Process tokens
	if err := writer.writeTokens(tokens); err != nil {
		return nil, fmt.Errorf("formatting failed: %w", err)
	}

	return out.Bytes(), nil
}

// FormatFile is a convenience method that reads, formats, and writes back a file.
// I3: Implement the method instead of returning "not implemented"
// Returns (changed bool, err error) where changed indicates if the file was modified.
func (f *Formatter) FormatFile(filename string) (changed bool, err error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return false, fmt.Errorf("read file: %w", err)
	}

	formatted, err := f.Format(src)
	if err != nil {
		// Syntax error: return original unchanged (graceful degradation)
		return false, nil
	}

	// Only write if content changed
	if bytes.Equal(src, formatted) {
		return false, nil
	}

	// Preserve file permissions (I4 pattern applied here too)
	info, err := os.Stat(filename)
	if err != nil {
		return false, fmt.Errorf("stat file: %w", err)
	}

	if err := os.WriteFile(filename, formatted, info.Mode().Perm()); err != nil {
		return false, fmt.Errorf("write file: %w", err)
	}

	return true, nil
}
