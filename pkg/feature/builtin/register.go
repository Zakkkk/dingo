// Package builtin provides the built-in Dingo language feature plugins.
package builtin

import (
	"github.com/MadAppGang/dingo/pkg/feature"
)

// Register all built-in plugins.
// This is called automatically via init().
func init() {
	// Character-level plugins (in order of priority)
	feature.Register(&EnumPlugin{})           // 10
	feature.Register(&MatchPlugin{})          // 20
	feature.Register(&EnumConstructorsPlugin{}) // 30
	feature.Register(&ErrorPropPlugin{})      // 40
	feature.Register(&GuardLetPlugin{})       // 50
	feature.Register(&SafeNavStatementsPlugin{}) // 55
	feature.Register(&SafeNavPlugin{})        // 60
	feature.Register(&NullCoalescePlugin{})   // 70
	feature.Register(&LambdasPlugin{})        // 80

	// Token-level plugins
	feature.Register(&GenericsPlugin{})        // 110
	feature.Register(&LetBindingPlugin{})      // 120
}
