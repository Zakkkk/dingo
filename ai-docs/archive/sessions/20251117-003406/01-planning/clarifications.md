# User Clarifications

## Questions and Answers

### 1. Lambda Syntax Coordination
**Decision**: Wait and coordinate with lambda implementation

The user wants to coordinate with the parallel lambda implementation session. Current status:
- Lambda feature is marked as "Not Started" in features/lambdas.md
- Lambda is being implemented in a parallel session (likely in a separate worktree)
- We need to design functional utilities to work with both Go function literals AND lambda syntax
- The feature spec shows multiple lambda styles (Rust pipes, TS arrows, Kotlin trailing, Swift dollar signs)

**Action**: Design the functional utilities plugin to accept BOTH function literal AST nodes and future lambda AST nodes. The transpilation output will be identical.

### 2. Initial Scope
**Decision**: Core + helpers (sum, count, all, any, find)

The user wants a more complete initial release including:
- Core operations: map, filter, reduce
- Helper utilities: sum, count, all, any, find
- This provides better user experience from the start

### 3. Result/Option Integration
**Decision**: Yes, they exist - integrate with them

Result[T, E] and Option[T] types are already implemented in the codebase. We should:
- Implement Result/Option-aware functional utilities
- Add operations like mapResult, filterSome, etc.
- Provide deep integration with Dingo's type system

### 4. Additional Context Discovered

**Feature Specification**: The features/functional-utilities.md file shows:
- Method-style syntax: `numbers.map(|x| x * 2)`
- Chaining support: `users.filter(...).map(...).sorted()`
- Transpiles to explicit Go loops

**Git Worktree Strategy**:
- No existing worktrees found (checked with `git worktree list`)
- Need to create a new worktree for this feature to avoid conflicts with parallel lambda work

**Package Architecture**:
- The plan suggests pkg/stdlib for the standard library
- This needs to align with the existing project structure
