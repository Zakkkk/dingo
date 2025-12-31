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
	Token       string      `yaml:"token"`       // Token to hover on (used if Character not set)
	Character   int         `yaml:"character"`   // Exact 0-based character position (overrides token search)
	Occurrence  int         `yaml:"occurrence"`  // Which occurrence (1-based, default 1)
	Description string      `yaml:"description"` // Human-readable description
	Expect      Expectation `yaml:"expect"`      // What we expect to see
	Correct     Expectation `yaml:"correct"`     // What the correct behavior should be (for documentation)
}

// Expectation defines what hover result to expect
type Expectation struct {
	Contains        string   `yaml:"contains"`        // Must contain this substring
	ContainsAny     []string `yaml:"containsAny"`     // Must contain any of these
	NotContains     string   `yaml:"notContains"`     // Must not contain this
	AllowAny        bool     `yaml:"allowAny"`        // Accept any result (skip assertion)
	RequireNonEmpty bool     `yaml:"requireNonEmpty"` // Require non-empty hover (for auto-scan)
	Regex           string   `yaml:"regex"`           // Match against regex (optional)
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

// LoadSpec loads a spec from a YAML file (single document only)
// For multi-document YAML files, use LoadSpecs instead
func LoadSpec(path string) (*Spec, error) {
	specs, err := LoadSpecs(path)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("no specs found in %s", path)
	}
	return specs[0], nil
}

// LoadSpecs loads all specs from a multi-document YAML file
// Each document separated by --- is a separate Spec targeting a different file
func LoadSpecs(path string) ([]*Spec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening spec file: %w", err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	var specs []*Spec

	for {
		var spec Spec
		err := decoder.Decode(&spec)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("parsing spec YAML: %w", err)
		}

		// Skip empty documents (comments-only sections)
		if spec.File == "" && len(spec.Cases) == 0 {
			continue
		}

		// Set defaults
		for i := range spec.Cases {
			if spec.Cases[i].Occurrence == 0 {
				spec.Cases[i].Occurrence = 1
			}
		}

		specs = append(specs, &spec)
	}

	return specs, nil
}

// CheckExpectation checks if the hover result matches the expectation
func (e *Expectation) CheckExpectation(hoverText string) (bool, string) {
	if e.AllowAny {
		return true, ""
	}

	// Normalize hover text (remove extra whitespace, handle markdown)
	normalized := normalizeHoverText(hoverText)

	// RequireNonEmpty: just check that we got something
	if e.RequireNonEmpty {
		if normalized != "" {
			return true, ""
		}
		return false, "non-empty hover"
	}

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

// FindTokenColumn finds the character offset (0-based) of a token on a line
// Returns character position at the CENTER of the token for more reliable hover results
// (hovering at token edges can hit adjacent symbols)
// NOTE: LSP uses character offsets, NOT visual columns (tabs count as 1 char, not 4)
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
			// Return byte offset directly as character offset (for ASCII)
			return byteIdx, nil
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
				// Calculate center position of token for more reliable hover
				// (edge positions can hit adjacent symbols)
				// Return byte offset directly as character offset (for ASCII)
				centerCharOffset := actualPos + len(token)/2
				return centerCharOffset, nil
			}
		}

		searchStart = actualPos + len(token)
	}

	if count < occurrence {
		return -1, fmt.Errorf("token %q occurrence %d not found on line (found %d with word boundaries)", token, occurrence, count)
	}

	return -1, fmt.Errorf("token %q occurrence %d not found on line", token, occurrence)
}
