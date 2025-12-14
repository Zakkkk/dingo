package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec represents a hover test specification file
type Spec struct {
	File  string     `yaml:"file"`
	Cases []TestCase `yaml:"cases"`
}

// TestCase represents a single hover test case
type TestCase struct {
	ID          int         `yaml:"id"`
	Line        int         `yaml:"line"`        // 1-based line number
	Token       string      `yaml:"token"`       // Token to hover on
	Occurrence  int         `yaml:"occurrence"`  // Which occurrence (1-based, default 1)
	Description string      `yaml:"description"` // Human-readable description
	Expect      Expectation `yaml:"expect"`      // What we expect to see
	Correct     Expectation `yaml:"correct"`     // What the correct behavior should be (for documentation)
}

// Expectation defines what hover result to expect
type Expectation struct {
	Contains    string   `yaml:"contains"`    // Must contain this substring
	ContainsAny []string `yaml:"containsAny"` // Must contain any of these
	NotContains string   `yaml:"notContains"` // Must not contain this
	AllowAny    bool     `yaml:"allowAny"`    // Accept any result (skip assertion)
	Regex       string   `yaml:"regex"`       // Match against regex (optional)
}

// CaseResult holds the result of running a test case
type CaseResult struct {
	ID          int    `json:"id"`
	Line        int    `json:"line"`
	Token       string `json:"token"`
	Column      int    `json:"column"`
	Passed      bool   `json:"passed"`
	Got         string `json:"got"`
	Expected    string `json:"expected"`
	Error       string `json:"error,omitempty"`
	Description string `json:"description,omitempty"`
}

// LoadSpec loads a spec from a YAML file
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}

	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing spec YAML: %w", err)
	}

	// Set defaults
	for i := range spec.Cases {
		if spec.Cases[i].Occurrence == 0 {
			spec.Cases[i].Occurrence = 1
		}
	}

	return &spec, nil
}

// CheckExpectation checks if the hover result matches the expectation
func (e *Expectation) CheckExpectation(hoverText string) (bool, string) {
	if e.AllowAny {
		return true, ""
	}

	// Normalize hover text (remove extra whitespace, handle markdown)
	normalized := normalizeHoverText(hoverText)

	if e.Contains != "" {
		if strings.Contains(normalized, e.Contains) {
			return true, ""
		}
		return false, e.Contains
	}

	if len(e.ContainsAny) > 0 {
		for _, s := range e.ContainsAny {
			if strings.Contains(normalized, s) {
				return true, ""
			}
		}
		return false, strings.Join(e.ContainsAny, " OR ")
	}

	if e.NotContains != "" {
		if strings.Contains(normalized, e.NotContains) {
			return false, fmt.Sprintf("should not contain %q", e.NotContains)
		}
		return true, ""
	}

	// No expectation defined - pass by default
	return true, ""
}

// normalizeHoverText cleans up hover text for comparison
func normalizeHoverText(text string) string {
	// Remove markdown code blocks
	text = strings.ReplaceAll(text, "```go\n", "")
	text = strings.ReplaceAll(text, "```\n", "")
	text = strings.ReplaceAll(text, "```", "")

	// Normalize whitespace
	text = strings.TrimSpace(text)

	return text
}

// FindTokenColumn finds the column (0-based, UTF-16 code units) of a token on a line
// Returns UTF-16 column position as required by LSP specification
func FindTokenColumn(lineText, token string, occurrence int) (int, error) {
	if occurrence < 1 {
		occurrence = 1
	}

	// Handle special tokens
	if token == "?" {
		// Find the ? that's not part of ?.
		byteIdx := -1
		count := 0
		for i := 0; i < len(lineText); i++ {
			if lineText[i] == '?' {
				// Check it's not ?. (safe navigation)
				if i+1 < len(lineText) && lineText[i+1] == '.' {
					continue
				}
				count++
				if count == occurrence {
					byteIdx = i
					break
				}
			}
		}
		if byteIdx >= 0 {
			// Convert byte offset to UTF-16 code units
			return ByteOffsetToUTF16(lineText, byteIdx), nil
		}
		return -1, fmt.Errorf("token %q occurrence %d not found on line", token, occurrence)
	}

	// For regular tokens, find the Nth occurrence with word boundary checks
	count := 0
	searchStart := 0

	for {
		pos := strings.Index(lineText[searchStart:], token)
		if pos == -1 {
			break
		}

		actualPos := searchStart + pos

		// Check word boundaries to avoid matching inside larger identifiers
		// (e.g., "id" inside "userID")
		if IsWordBoundary(lineText, actualPos, len(token)) {
			count++
			if count == occurrence {
				// Convert byte offset to UTF-16 code units
				return ByteOffsetToUTF16(lineText, actualPos), nil
			}
		}

		searchStart = actualPos + len(token)
	}

	if count < occurrence {
		return -1, fmt.Errorf("token %q occurrence %d not found on line (found %d with word boundaries)", token, occurrence, count)
	}

	return -1, fmt.Errorf("token %q occurrence %d not found on line", token, occurrence)
}
