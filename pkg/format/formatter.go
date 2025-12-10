package format

import (
	"bytes"
	"fmt"

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

// FormatFile is a convenience method that reads, formats, and writes back
// This will be used by the CLI
func (f *Formatter) FormatFile(filename string) error {
	// Implementation will be in CLI layer
	// This is just the signature
	return fmt.Errorf("not implemented")
}
