package feature

import (
	"testing"
)

// MockPlugin for testing
type mockPlugin struct {
	name         string
	version      string
	ptype        PluginType
	priority     int
	dependencies []string
	conflicts    []string
	detectFn     func([]byte) []SyntaxLocation
	transformFn  func([]byte, *Context) ([]byte, error)
}

func (p *mockPlugin) Name() string               { return p.name }
func (p *mockPlugin) Version() string            { return p.version }
func (p *mockPlugin) Type() PluginType           { return p.ptype }
func (p *mockPlugin) Priority() int              { return p.priority }
func (p *mockPlugin) Dependencies() []string     { return p.dependencies }
func (p *mockPlugin) Conflicts() []string        { return p.conflicts }
func (p *mockPlugin) Detect(src []byte) []SyntaxLocation {
	if p.detectFn != nil {
		return p.detectFn(src)
	}
	return nil
}
func (p *mockPlugin) Transform(src []byte, ctx *Context) ([]byte, error) {
	if p.transformFn != nil {
		return p.transformFn(src, ctx)
	}
	return src, nil
}

func TestPluginRegistration(t *testing.T) {
	// Clear existing plugins for test isolation
	pluginsMu.Lock()
	oldPlugins := plugins
	plugins = make(map[string]Plugin)
	pluginsMu.Unlock()
	defer func() {
		pluginsMu.Lock()
		plugins = oldPlugins
		pluginsMu.Unlock()
	}()

	// Register test plugins
	p1 := &mockPlugin{name: "test1", priority: 10}
	p2 := &mockPlugin{name: "test2", priority: 20}
	Register(p1)
	Register(p2)

	// Check registration
	got, ok := GetPlugin("test1")
	if !ok {
		t.Fatal("expected plugin 'test1' to be registered")
	}
	if got.Name() != "test1" {
		t.Errorf("expected name 'test1', got %q", got.Name())
	}

	// Check listing
	names := ListPluginNames()
	if len(names) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(names))
	}

	// Check priority ordering
	list := ListPlugins()
	if list[0].Priority() > list[1].Priority() {
		t.Error("plugins should be sorted by priority")
	}
}

func TestEngineTransform(t *testing.T) {
	// Clear existing plugins for test isolation
	pluginsMu.Lock()
	oldPlugins := plugins
	plugins = make(map[string]Plugin)
	pluginsMu.Unlock()
	defer func() {
		pluginsMu.Lock()
		plugins = oldPlugins
		pluginsMu.Unlock()
	}()

	// Register a transform plugin
	transformed := false
	p := &mockPlugin{
		name:     "transformer",
		priority: 10,
		ptype:    CharacterLevel,
		transformFn: func(src []byte, ctx *Context) ([]byte, error) {
			transformed = true
			return append(src, []byte("_transformed")...), nil
		},
	}
	Register(p)

	// Create engine
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Transform
	result, err := engine.Transform([]byte("test"), "test.dingo")
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}

	if !transformed {
		t.Error("transform function was not called")
	}

	if string(result) != "test_transformed" {
		t.Errorf("expected 'test_transformed', got %q", string(result))
	}
}

func TestEngineDisabledFeature(t *testing.T) {
	// Clear existing plugins for test isolation
	pluginsMu.Lock()
	oldPlugins := plugins
	plugins = make(map[string]Plugin)
	pluginsMu.Unlock()
	defer func() {
		pluginsMu.Lock()
		plugins = oldPlugins
		pluginsMu.Unlock()
	}()

	// Register a plugin that detects "FEATURE" keyword
	p := &mockPlugin{
		name:     "feature_x",
		priority: 10,
		ptype:    CharacterLevel,
		detectFn: func(src []byte) []SyntaxLocation {
			// Detect "FEATURE" in source
			for i := 0; i < len(src)-7; i++ {
				if string(src[i:i+7]) == "FEATURE" {
					return []SyntaxLocation{{Line: 1, Column: i + 1, Snippet: "FEATURE"}}
				}
			}
			return nil
		},
	}
	Register(p)

	// Create engine with feature disabled
	enabled := EnabledFeatures{"feature_x": false}
	engine, err := NewEngine(enabled)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Transform should fail because disabled feature is detected
	_, err = engine.Transform([]byte("some FEATURE here"), "test.dingo")
	if err == nil {
		t.Fatal("expected error for disabled feature")
	}

	// Check error type
	disabledErr, ok := err.(*DisabledFeatureError)
	if !ok {
		t.Fatalf("expected DisabledFeatureError, got %T", err)
	}
	if disabledErr.Feature != "feature_x" {
		t.Errorf("expected feature 'feature_x', got %q", disabledErr.Feature)
	}
}

func TestEngineDependencyValidation(t *testing.T) {
	// Clear existing plugins for test isolation
	pluginsMu.Lock()
	oldPlugins := plugins
	plugins = make(map[string]Plugin)
	pluginsMu.Unlock()
	defer func() {
		pluginsMu.Lock()
		plugins = oldPlugins
		pluginsMu.Unlock()
	}()

	// Register plugins with dependencies
	p1 := &mockPlugin{name: "base", priority: 10}
	p2 := &mockPlugin{name: "dependent", priority: 20, dependencies: []string{"base"}}
	Register(p1)
	Register(p2)

	// Both enabled - should work
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("expected success with both plugins enabled, got: %v", err)
	}
	if len(engine.EnabledPluginNames()) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(engine.EnabledPluginNames()))
	}

	// Disable base - dependent should fail
	enabled := EnabledFeatures{"base": false, "dependent": true}
	_, err = NewEngine(enabled)
	if err == nil {
		t.Fatal("expected error when dependency is disabled")
	}
	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("expected DependencyError, got %T", err)
	}
	if depErr.Plugin != "dependent" {
		t.Errorf("expected plugin 'dependent', got %q", depErr.Plugin)
	}
}

func TestEngineConflictValidation(t *testing.T) {
	// Clear existing plugins for test isolation
	pluginsMu.Lock()
	oldPlugins := plugins
	plugins = make(map[string]Plugin)
	pluginsMu.Unlock()
	defer func() {
		pluginsMu.Lock()
		plugins = oldPlugins
		pluginsMu.Unlock()
	}()

	// Register conflicting plugins
	p1 := &mockPlugin{name: "plugin_a", priority: 10}
	p2 := &mockPlugin{name: "plugin_b", priority: 20, conflicts: []string{"plugin_a"}}
	Register(p1)
	Register(p2)

	// Both enabled - should fail
	_, err := NewEngine(nil)
	if err == nil {
		t.Fatal("expected error when conflicting plugins are enabled")
	}
	conflictErr, ok := err.(*ConflictError)
	if !ok {
		t.Fatalf("expected ConflictError, got %T", err)
	}
	if conflictErr.Plugin != "plugin_b" {
		t.Errorf("expected plugin 'plugin_b', got %q", conflictErr.Plugin)
	}

	// Disable one - should work
	enabled := EnabledFeatures{"plugin_a": false}
	engine, err := NewEngine(enabled)
	if err != nil {
		t.Fatalf("expected success with conflict resolved, got: %v", err)
	}
	if len(engine.EnabledPluginNames()) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(engine.EnabledPluginNames()))
	}
}

func TestSharedRegistry(t *testing.T) {
	reg := NewSharedRegistry()

	// Test Set and Get
	reg.Set("key1", "value1")
	v, ok := reg.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if v != "value1" {
		t.Errorf("expected 'value1', got %v", v)
	}

	// Test GetString
	reg.Set("string_key", "hello")
	s := reg.GetString("string_key")
	if s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}

	// Test GetStringSlice
	reg.Set("slice_key", []string{"a", "b", "c"})
	slice := reg.GetStringSlice("slice_key")
	if len(slice) != 3 {
		t.Errorf("expected 3 elements, got %d", len(slice))
	}

	// Test GetMap
	reg.Set("map_key", map[string]interface{}{"foo": "bar"})
	m := reg.GetMap("map_key")
	if m["foo"] != "bar" {
		t.Errorf("expected 'bar', got %v", m["foo"])
	}
}
