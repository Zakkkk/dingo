package feature

// BasePlugin provides common functionality for plugins.
// Embed this in your plugin struct to get default implementations.
type BasePlugin struct {
	name         string
	version      string
	pluginType   PluginType
	priority     int
	dependencies []string
	conflicts    []string
}

// NewBasePlugin creates a new base plugin with the given configuration
func NewBasePlugin(name, version string, ptype PluginType, priority int) BasePlugin {
	return BasePlugin{
		name:       name,
		version:    version,
		pluginType: ptype,
		priority:   priority,
	}
}

// Name returns the plugin name
func (b *BasePlugin) Name() string { return b.name }

// Version returns the plugin version
func (b *BasePlugin) Version() string { return b.version }

// Type returns the plugin type
func (b *BasePlugin) Type() PluginType { return b.pluginType }

// Priority returns the plugin priority
func (b *BasePlugin) Priority() int { return b.priority }

// Dependencies returns the plugin dependencies
func (b *BasePlugin) Dependencies() []string { return b.dependencies }

// Conflicts returns the plugin conflicts
func (b *BasePlugin) Conflicts() []string { return b.conflicts }

// SetDependencies sets the plugin dependencies
func (b *BasePlugin) SetDependencies(deps ...string) {
	b.dependencies = deps
}

// SetConflicts sets the plugin conflicts
func (b *BasePlugin) SetConflicts(conflicts ...string) {
	b.conflicts = conflicts
}

// Detect returns empty by default (no syntax detected)
// Override this in your plugin implementation
func (b *BasePlugin) Detect(src []byte) []SyntaxLocation {
	return nil
}
