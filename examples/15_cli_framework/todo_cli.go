// Dingo CLI Framework Example - Todo List Manager
// Demonstrates idiomatic Dingo patterns for CLI applications:
// - Enum-backed subcommands with pattern matching dispatch
// - The ? operator for clean error propagation
// - let bindings for immutable-by-default variables
// - Match expressions with pattern destructuring
// - Zero runtime overhead - transpiles to pure Go
//
// Run: dingo run examples/15_cli_framework/todo_cli.dingo -- help

package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"github.com/MadAppGang/dingo/pkg/dgo"
)

// =============================================================================
// VALIDATION HELPERS
// Return Result types to enable guard let unwrapping
// =============================================================================

func validateDescription(s string) dgo.Result[string, error] {
	if len(s) < 2 {
		return dgo.Err[string, error](errors.New("task description too short"))
	}
	return dgo.Ok[string, error](s)
}

func requireArgs(args []string, message string) dgo.Result[string, error] {
	if len(args) == 0 {
		return dgo.Err[string, error](errors.New(message))
	}
	return dgo.Ok[string, error](args[0])
}

// =============================================================================
// COMMAND DEFINITIONS
// Sum types make subcommands exhaustive - compiler enforces all cases handled
// =============================================================================

type Command interface{ isCommand() }

type CommandAdd struct{ description string }

func (CommandAdd) isCommand()                  {}
func NewCommandAdd(description string) Command { return CommandAdd{description: description} }

type CommandList struct{}

func (CommandList) isCommand() {}
func NewCommandList() Command  { return CommandList{} }

type CommandDone struct{ id int }

func (CommandDone) isCommand()      {}
func NewCommandDone(id int) Command { return CommandDone{id: id} }

type CommandRemove struct{ id int }

func (CommandRemove) isCommand()      {}
func NewCommandRemove(id int) Command { return CommandRemove{id: id} }

type CommandHelp struct{}

func (CommandHelp) isCommand() {}
func NewCommandHelp() Command  { return CommandHelp{} }

// =============================================================================
// DOMAIN TYPES
// =============================================================================

type Task struct {
	ID          int
	Description string
	Completed   bool
}

// In-memory storage - demo only, doesn't persist between runs
// A real app would use file/database storage
var tasks = []Task{}
var nextID = 1

// =============================================================================
// ARGUMENT PARSING
// Returns (Command, error) for clean ? operator usage
// =============================================================================

func parseArgs(args []string) (Command, error) {
	if len(args) == 0 {
		return NewCommandHelp(), nil
	}

	cmd := args[0]
	rest := args[1:]

	// Route to subcommand parsers
	if cmd == "add" {
		return parseAddCommand(rest)
	}
	if cmd == "list" || cmd == "ls" {
		return NewCommandList(), nil
	}
	if cmd == "done" || cmd == "complete" {
		return parseDoneCommand(rest)
	}
	if cmd == "remove" || cmd == "rm" {
		return parseRemoveCommand(rest)
	}
	if cmd == "help" || cmd == "--help" || cmd == "-h" {
		return NewCommandHelp(), nil
	}

	return nil, fmt.Errorf("unknown command: %s\nRun 'todo help' for usage", cmd)
}

func parseAddCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("add requires a task description\nUsage: todo add <description>")
	}

	// guard unwraps Result - returns early on Err
	tmp := validateDescription(strings.Join(args, " "))
	if tmp.IsErr() {
		err := *tmp.Err

		return nil, err

	}
	description := *tmp.Ok

	return NewCommandAdd(description), nil
}

func parseDoneCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("done requires a task ID\nUsage: todo done <id>")
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %s", args[0])
	}
	return NewCommandDone(id), nil
}

func parseRemoveCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return nil, errors.New("remove requires a task ID\nUsage: todo remove <id>")
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %s", args[0])
	}
	return NewCommandRemove(id), nil
}

// =============================================================================
// COMMAND DISPATCH
// Pattern matching ensures ALL commands are handled - no forgotten cases
// =============================================================================

func dispatch(cmd Command) error {
	// Match with pattern destructuring - extracts values directly
	val := cmd
	switch v1 := val.(type) {
	case CommandAdd:
		description := v1.description
		return handleAdd(description)
	case CommandList:
		return handleList()
	case CommandDone:
		id := v1.id
		return handleDone(id)
	case CommandRemove:
		id := v1.id
		return handleRemove(id)
	case CommandHelp:
		return handleHelp()
	}
	panic("unreachable: exhaustive match")
}

// =============================================================================
// COMMAND HANDLERS
// =============================================================================

func handleAdd(description string) error {
	task := Task{
		ID:          nextID,
		Description: description,
		Completed:   false,
	}

	tasks = append(tasks, task)
	nextID++

	fmt.Printf("Added task #%d: %s\n", task.ID, task.Description)
	return nil
}

func handleList() error {
	if len(tasks) == 0 {
		fmt.Println("No tasks yet. Add one with: todo add <description>")
		return nil
	}

	fmt.Printf("Tasks (%d total):\n", len(tasks))
	for _, task := range tasks {
		status := " "
		if task.Completed {
			status = "x"
		}
		fmt.Printf("  [%s] #%d %s\n", status, task.ID, task.Description)
	}

	return nil
}

func handleDone(id int) error {
	idx := findTaskIndex(id)
	if idx < 0 {
		return fmt.Errorf("task #%d not found", id)
	}

	if tasks[idx].Completed {
		fmt.Printf("Task #%d already completed\n", id)
	} else {
		tasks[idx].Completed = true
		fmt.Printf("Completed task #%d: %s\n", id, tasks[idx].Description)
	}
	return nil
}

func handleRemove(id int) error {
	idx := findTaskIndex(id)
	if idx < 0 {
		return fmt.Errorf("task #%d not found", id)
	}

	desc := tasks[idx].Description
	tasks = removeAt(tasks, idx)
	fmt.Printf("Removed task #%d: %s\n", id, desc)
	return nil
}

func handleHelp() error {
	fmt.Println(`todo - A simple task manager

Usage: todo <command> [arguments]

Commands:
  add <description>    Add a new task
  list, ls             List all tasks
  done <id>            Mark a task as complete
  remove, rm <id>      Remove a task
  help                 Show this help

Examples:
  todo add "Buy groceries"
  todo list
  todo done 1
  todo remove 2`)

	return nil
}

// =============================================================================
// HELPERS
// =============================================================================

func findTaskIndex(id int) int {
	for i, task := range tasks {
		if task.ID == id {
			return i
		}
	}
	return -1
}

func removeAt(items []Task, idx int) []Task {
	var result []Task
	for i, item := range items {
		if i != idx {
			result = append(result, item)
		}
	}
	return result
}

// =============================================================================
// MAIN ENTRY POINT
// =============================================================================

func main() {
	cmd, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	err = dispatch(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
