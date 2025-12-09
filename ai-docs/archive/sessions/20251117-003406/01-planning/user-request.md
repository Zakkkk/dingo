# User Request: Functional Utilities for Dingo Standard Library

## Feature Request
Implement functional utilities (map, filter, reduce) for the Dingo standard library.

## Important Context
- There is a parallel development session running
- This feature should be developed in a **git worktree** to avoid conflicts
- The implementation should be clean and simple (no backward compatibility needed as there are no releases yet)

## Deliverables
1. Map utility - transform collections
2. Filter utility - select elements from collections
3. Reduce utility - aggregate collections into a single value

## Implementation Notes
- Follow Dingo design principles: simplicity, readability, zero runtime overhead
- Generate clean, idiomatic Go code
- Ensure full Go ecosystem compatibility
- Consider how these utilities work with Dingo's Result[T, E] and Option[T] types
