# Dingo CLI Framework Example

A complete, working example of an idiomatic Dingo CLI application demonstrating how Dingo improves Go CLI ergonomics while transpiling to clean, zero-overhead Go code.

## Key Dingo Features Used

### 1. Enum-Backed Subcommands
```dingo
enum Command {
    Add { description: string }
    List
    Done { id: int }
    Remove { id: int }
    Help
}
```

### 2. Pattern Matching with Destructuring
```dingo
func dispatch(cmd Command) error {
    return match cmd {
        Add(description) => handleAdd(description),
        List => handleList(),
        Done(id) => handleDone(id),
        Remove(id) => handleRemove(id),
        Help => handleHelp(),
    }
}
```

### 3. Let Bindings
```dingo
let cmd = args[0]
let rest = args[1:]
let task = Task{ID: nextID, Description: description}
```

### 4. Enum Constructors
```dingo
return Command.Help(), nil
return Command.Add(description), nil
return Command.Done(id), nil
```

### 5. Guard Let for Result Unwrapping
```dingo
// Validation function returns Result[T, error]
func validateDescription(s string) Result[string, error] {
    if len(s) < 2 {
        return Err[string, error](errors.New("task description too short"))
    }
    return Ok[string, error](s)
}

// guard let unwraps Result - returns early on Err
guard let description = validateDescription(input) else |err| {
    return nil, err
}
// 'description' is now available as the unwrapped string
```

**Generated Go:**
```go
tmp := validateDescription(input)
if tmp.IsErr() {
    err := *tmp.Err
    return nil, err
}
description := *tmp.Ok
```

## Building & Running

```bash
# Build the CLI
dingo build examples/15_cli_framework/todo_cli.dingo -o todo

# Or build + run in one step
dingo run examples/15_cli_framework/todo_cli.dingo -- help
```

## Usage

```bash
./todo help                    # Show help
./todo add "Buy groceries"     # Add a task
./todo list                    # List tasks
./todo done 1                  # Complete task #1
./todo remove 2                # Remove task #2
```

## Comparison: Dingo vs Plain Go

### Command Dispatch

**Dingo (5 lines):**
```dingo
return match cmd {
    Add(description) => handleAdd(description),
    List => handleList(),
    Done(id) => handleDone(id),
    Remove(id) => handleRemove(id),
    Help => handleHelp(),
}
```

**Generated Go (13 lines):**
```go
switch c := cmd.(type) {
case CommandAdd:
    return handleAdd(c.description)
case CommandList:
    return handleList()
case CommandDone:
    return handleDone(c.id)
case CommandRemove:
    return handleRemove(c.id)
case CommandHelp:
    return handleHelp()
}
return errors.New("unknown command")
```

### Enum Definitions

**Dingo:**
```dingo
enum Command {
    Add { description: string }
    List
    Done { id: int }
}
```

**Generated Go (auto-generates all boilerplate):**
```go
type Command interface{ isCommand() }

type CommandAdd struct{ description string }
func (CommandAdd) isCommand() {}
func NewCommandAdd(description string) Command {
    return CommandAdd{description: description}
}

type CommandList struct{}
func (CommandList) isCommand() {}
func NewCommandList() Command { return CommandList{} }

// ... etc
```

## Why This Matters

| Aspect | Traditional Go CLI | Dingo CLI |
|--------|-------------------|-----------|
| **Exhaustiveness** | Manual - easy to miss cases | Compiler-enforced via match |
| **Boilerplate** | ~60 lines per enum | ~5 lines per enum |
| **Type Safety** | Runtime parsing errors | Compile-time guarantees |
| **Readability** | Scattered switch statements | Unified match expressions |

## Generated Go Code

The transpiled `.go` file demonstrates clean, idiomatic Go output:
- Interface-based sum type implementation
- Type-safe constructor functions
- No reflection or runtime overhead
- Standard Go error handling patterns
