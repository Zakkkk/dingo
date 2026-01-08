//go:build windows

package main

import (
	"os"
)

// interruptProcess on Windows sends a Ctrl+C event to the process
// Since Windows doesn't have SIGINT, we just kill the process directly
// A more sophisticated approach would use GenerateConsoleCtrlEvent,
// but that requires CGO or unsafe syscalls which we want to avoid.
func interruptProcess(p *os.Process) error {
	// On Windows, we simply kill the process
	// The graceful shutdown timeout in Stop() will still apply
	return p.Kill()
}
