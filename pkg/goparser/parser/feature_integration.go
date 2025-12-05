// Package parser provides feature integration for the AST-based parser.
// This file is a placeholder - the old string-based feature transforms
// have been replaced with AST-based parsing in pkg/parser/.
package parser

import (
	"github.com/MadAppGang/dingo/pkg/feature"
)

// TransformWithFeatureEngine is a compatibility stub.
// In the new AST-based architecture, features are handled during parsing,
// not as string transformation passes.
//
// If enabled is nil, all features are enabled.
func TransformWithFeatureEngine(src []byte, filename string, enabled feature.EnabledFeatures) ([]byte, error) {
	// In the AST-based pipeline, source transformation happens during parsing.
	// This function is kept for backward compatibility but is essentially a no-op.
	return src, nil
}

// GetEnabledFeatures returns the list of enabled feature names
func GetEnabledFeatures(enabled feature.EnabledFeatures) []string {
	if enabled == nil {
		// All features enabled
		return feature.ListPluginNames()
	}

	var names []string
	for _, p := range feature.ListPlugins() {
		if en, ok := enabled[p.Name()]; ok && !en {
			continue // Explicitly disabled
		}
		names = append(names, p.Name())
	}
	return names
}
