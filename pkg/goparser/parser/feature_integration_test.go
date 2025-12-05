package parser

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/feature"
	_ "github.com/MadAppGang/dingo/pkg/feature/builtin" // Register plugins
)

func TestTransformWithFeatureEngine_PassThrough(t *testing.T) {
	// In the new AST-based architecture, TransformWithFeatureEngine is a no-op
	// since transformation happens during parsing, not as a pre-pass
	src := []byte(`package main

func example() {
	let x = 42
}
`)
	result, err := TransformWithFeatureEngine(src, "test.dingo", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Source should pass through unchanged
	if string(result) != string(src) {
		t.Error("TransformWithFeatureEngine should be pass-through in AST architecture")
	}
}

func TestGetEnabledFeatures(t *testing.T) {
	// nil means all enabled
	all := GetEnabledFeatures(nil)
	if len(all) == 0 {
		t.Error("expected some features when all enabled")
	}

	// Check specific feature is in list
	found := false
	for _, name := range all {
		if name == "enum" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'enum' in enabled features")
	}

	// With some disabled
	enabled := feature.EnabledFeatures{"enum": false, "match": false}
	partial := GetEnabledFeatures(enabled)

	// enum and match should not be in list
	for _, name := range partial {
		if name == "enum" || name == "match" {
			t.Errorf("feature '%s' should be disabled", name)
		}
	}
}
