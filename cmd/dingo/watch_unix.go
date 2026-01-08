//go:build !windows

package main

import (
	"os"
	"syscall"
)

// interruptProcess sends SIGINT to the process for graceful shutdown
func interruptProcess(p *os.Process) error {
	return p.Signal(syscall.SIGINT)
}
