package main

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec defines expected diagnostics for a file
type Spec struct {
	File        string               `yaml:"file"`
	Description string               `yaml:"description"`
	Expected    []ExpectedDiagnostic `yaml:"expected"`
	ExpectNone  bool                 `yaml:"expect_none"` // Expect no diagnostics
}

// ExpectedDiagnostic defines a single expected diagnostic
type ExpectedDiagnostic struct {
	Description string `yaml:"description"`
	Line        int    `yaml:"line"`     // 1-based line number
	Severity    string `yaml:"severity"` // "error", "warning", "info", "hint"
	Source      string `yaml:"source"`   // "dingo", "dingo-lint", "gopls"
	Contains    string `yaml:"contains"` // Substring to match in message
	Code        string `yaml:"code"`     // Diagnostic code (e.g., "D001")
}

// LoadSpecs loads specs from a YAML file (supports multi-document YAML)
func LoadSpecs(path string) ([]*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var specs []*Spec
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))

	for {
		var spec Spec
		err := decoder.Decode(&spec)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
		if spec.File != "" {
			specs = append(specs, &spec)
		}
	}

	return specs, nil
}
