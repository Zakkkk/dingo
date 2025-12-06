compilerOptions: {
  "plugins": [
    {"name": "ts-plugin-name", "option1": "value1", "option2": true}
  ]
}

# Babel configuration
{
  "plugins": [
    ["plugin-name", {"option": "value"}],
    "@babel/plugin-transform-runtime"
  ]
}

# Dingo configuration (current)
[features]
error_propagation_syntax = "question"
reuse_err_variable = true
nil_safety_checks = "on"

[features.result_type]
enabled = true
go_interop = "opt-in"

# Recommended Architecture for Dingo

## Hybrid Approach: AST Transformation + LSP Extension

### Core Plugin Interface

type SyntaxPlugin interface {
    Name() string
    Version() string
    Dependencies() []string
    
    // Token extensions (new syntax)
    TokenExtensions() []TokenExtension
    
    // AST transformations
    Transform(node ast.Node, ctx *TransformContext) (ast.Node, error)
    
    // LSP extensions for IDE support
    LSPExtensions() []LSPExtension
}

### Integration Points
1. Parser integration for token extensions and AST transformations
2. LSP integration for IDE features
3. Configuration system for plugin management

### Configuration Extension

[plugins]
enabled = ["result-type", "pattern-matching", "async-await"]

[plugins.result-type]
error_syntax = "question"
go_interop = "opt-in"

[plugins.pattern-matching]
exhaustiveness_check = true
allow_wildcards = true

## Security Considerations

- Plugin validation before loading
- Sandboxed execution environment
- Limited API surface for plugins
- Semantic versioning for plugin APIs
- Feature flags for gradual rollout

## Benefits

- Extensible: Easy to add new syntax features
- Safe: Sandboxed plugin execution with validation
- Performant: Minimal overhead through lazy loading
- Compatible: Works with existing Go tooling
- Discoverable: Clear plugin development patterns

This approach positions Dingo as a truly extensible meta-language for Go, allowing third-party developers to add syntax features without modifying the core parser.