package registry

import "fmt"

// Scope represents a lexical scope in the code
type Scope struct {
	// Name is a descriptive name for this scope (e.g., "func:main", "block:if")
	Name string

	// Level is the nesting depth (0 = package level, 1 = function, 2+ = nested blocks)
	Level int

	// Parent is the enclosing scope (nil for package scope)
	Parent *Scope

	// Variables maps variable names to their info
	Variables map[string]VariableInfo

	// Functions maps function names to their info (only at package scope)
	Functions map[string]FunctionInfo
}

// NewScope creates a new scope with the given name and parent
func NewScope(name string, parent *Scope) *Scope {
	level := 0
	if parent != nil {
		level = parent.Level + 1
	}

	return &Scope{
		Name:      name,
		Level:     level,
		Parent:    parent,
		Variables: make(map[string]VariableInfo),
		Functions: make(map[string]FunctionInfo),
	}
}

// Lookup searches for a variable in this scope and all parent scopes
func (s *Scope) Lookup(name string) (VariableInfo, bool) {
	if info, ok := s.Variables[name]; ok {
		return info, true
	}

	if s.Parent != nil {
		return s.Parent.Lookup(name)
	}

	return VariableInfo{}, false
}

// LookupFunction searches for a function in this scope and all parent scopes
func (s *Scope) LookupFunction(name string) (FunctionInfo, bool) {
	if info, ok := s.Functions[name]; ok {
		return info, true
	}

	if s.Parent != nil {
		return s.Parent.LookupFunction(name)
	}

	return FunctionInfo{}, false
}

// Register adds a variable to this scope
func (s *Scope) Register(info VariableInfo) {
	info.Scope = s.Level
	s.Variables[info.Name] = info
}

// RegisterFunction adds a function to this scope (typically package scope)
func (s *Scope) RegisterFunction(info FunctionInfo) {
	s.Functions[info.Name] = info
}

// String returns a human-readable representation of this scope
func (s *Scope) String() string {
	return fmt.Sprintf("Scope{name=%s, level=%d, vars=%d}", s.Name, s.Level, len(s.Variables))
}

// ScopeManager manages the scope stack during AST traversal
type ScopeManager struct {
	// current is the currently active scope
	current *Scope

	// scopeHistory tracks all scopes for debugging
	scopeHistory []*Scope
}

// NewScopeManager creates a new scope manager with a package-level scope
func NewScopeManager(packageName string) *ScopeManager {
	pkgScope := NewScope("package:"+packageName, nil)
	return &ScopeManager{
		current:      pkgScope,
		scopeHistory: []*Scope{pkgScope},
	}
}

// EnterScope creates and enters a new child scope
func (sm *ScopeManager) EnterScope(name string) {
	newScope := NewScope(name, sm.current)
	sm.current = newScope
	sm.scopeHistory = append(sm.scopeHistory, newScope)
}

// ExitScope returns to the parent scope
func (sm *ScopeManager) ExitScope() error {
	if sm.current.Parent == nil {
		return fmt.Errorf("cannot exit package scope")
	}
	sm.current = sm.current.Parent
	return nil
}

// CurrentScope returns the currently active scope
func (sm *ScopeManager) CurrentScope() *Scope {
	return sm.current
}

// Lookup searches for a variable in the current scope and all parent scopes
func (sm *ScopeManager) Lookup(name string) (VariableInfo, bool) {
	return sm.current.Lookup(name)
}

// LookupFunction searches for a function in the current scope and all parent scopes
func (sm *ScopeManager) LookupFunction(name string) (FunctionInfo, bool) {
	return sm.current.LookupFunction(name)
}

// Register adds a variable to the current scope
func (sm *ScopeManager) Register(info VariableInfo) {
	sm.current.Register(info)
}

// RegisterFunction adds a function to the current scope
func (sm *ScopeManager) RegisterFunction(info FunctionInfo) {
	sm.current.RegisterFunction(info)
}

// Reset resets the scope manager to a fresh package scope
func (sm *ScopeManager) Reset(packageName string) {
	pkgScope := NewScope("package:"+packageName, nil)
	sm.current = pkgScope
	sm.scopeHistory = []*Scope{pkgScope}
}

// GetScopeLevel returns the current scope nesting level
func (sm *ScopeManager) GetScopeLevel() int {
	return sm.current.Level
}

// String returns a human-readable representation of the scope stack
func (sm *ScopeManager) String() string {
	scopeChain := ""
	scope := sm.current
	for scope != nil {
		if scopeChain != "" {
			scopeChain = " -> " + scopeChain
		}
		scopeChain = scope.Name + scopeChain
		scope = scope.Parent
	}
	return fmt.Sprintf("ScopeManager{current=%s, depth=%d}", scopeChain, sm.current.Level)
}
