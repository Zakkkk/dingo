package builtin

// WireTransforms connects the actual transform functions from the parser package
// to the plugin system. This must be called during initialization.
//
// Usage (in parser package):
//
//	func init() {
//	    builtin.WireTransforms(builtin.TransformFuncs{
//	        Enum:              transformEnum,
//	        Match:             transformMatch,
//	        EnumConstructors:  transformEnumConstructors,
//	        ErrorProp:         transformErrorProp,
//	        Guard:          transformGuard,
//	        SafeNavStatements: transformSafeNavStatements,
//	        SafeNav:           transformSafeNav,
//	        NullCoalesce:      transformNullCoalesce,
//	        Lambdas:           transformLambdas,
//	    })
//	}
func WireTransforms(funcs TransformFuncs) {
	Transforms = funcs
}

// IsWired returns true if transform functions have been wired
func IsWired() bool {
	return Transforms.Enum != nil
}
