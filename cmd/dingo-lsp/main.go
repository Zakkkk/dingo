package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/MadAppGang/dingo/pkg/lsp"
	"github.com/MadAppGang/dingo/pkg/version"
	"go.lsp.dev/jsonrpc2"
)

const maxLogSize = 5 * 1024 * 1024 // 5MB max log file size

var logger lsp.Logger

func main() {
	// Handle --version flag
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("dingo-lsp %s\n", version.Version)
		return
	}

	// Configure logging from environment variable
	logLevel := os.Getenv("DINGO_LSP_LOG")
	if logLevel == "" {
		logLevel = "info"
	}

	// Configure log output - stderr by default, optionally to file
	var logOutput io.Writer = os.Stderr
	logFile := os.Getenv("DINGO_LSP_LOGFILE")
	if logFile != "" {
		// Rotate log file if too large
		rotateLogFile(logFile)

		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			// Write to both stderr and file for debugging
			logOutput = io.MultiWriter(os.Stderr, f)
			defer f.Close()
		}
	}

	logger = lsp.NewLogger(logLevel, logOutput)

	// Add SQLite logger if enabled via environment variable
	sqliteLogPath := os.Getenv("DINGO_LSP_SQLITE")
	if sqliteLogPath != "" {
		sqliteLogger, err := lsp.NewSQLiteLogger(sqliteLogPath, logLevel)
		if err != nil {
			// Fall back to standard logger only - log error but don't fail
			logger.Warnf("SQLite logging initialization failed: %v", err)
		} else {
			// Combine text and SQLite loggers
			multiLogger := lsp.NewMultiLogger(logger, sqliteLogger)
			logger = multiLogger
			// Ensure cleanup on shutdown
			defer func() {
				if err := sqliteLogger.Close(); err != nil {
					logger.Warnf("Failed to close SQLite logger: %v", err)
				}
			}()
			logger.Infof("SQLite logging enabled: %s", sqliteLogPath)
		}
	}

	logger.Infof("Starting dingo-lsp server (log level: %s)", logLevel)

	// Find gopls in $PATH
	goplsPath := findGopls(logger)
	if goplsPath == "" {
		logger.Fatalf("gopls not found in $PATH. Install: go install golang.org/x/tools/gopls@latest")
	}

	// Create LSP proxy server
	server, err := lsp.NewServer(lsp.ServerConfig{
		Logger:        logger,
		GoplsPath:     goplsPath,
		AutoTranspile: true, // Default from user decision
	})
	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}

	// Create stdio transport using ReadWriteCloser wrapper
	logger.Infof("Creating stdin/stdout ReadWriteCloser")
	rwc := &stdinoutCloser{stdin: os.Stdin, stdout: os.Stdout, logger: logger}
	logger.Infof("Creating JSON-RPC2 stream")
	stream := jsonrpc2.NewStream(rwc)
	logger.Infof("Creating JSON-RPC2 connection")
	conn := jsonrpc2.NewConn(stream)
	logger.Infof("JSON-RPC2 connection created: %p", conn)

	// Start serving with cancellable context (Gemini fix)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// CRITICAL FIX (Sherlock): Store connection BEFORE starting handler
	// This prevents race condition where handlers try to use nil ideConn
	logger.Infof("Storing connection in server")
	server.SetConn(conn, ctx)

	// Create handler and start connection
	handler := server.Handler()
	logger.Infof("Starting JSON-RPC2 connection handler")
	conn.Go(ctx, handler)
	logger.Infof("JSON-RPC2 connection handler started")

	// Wait for connection to close
	<-conn.Done()
	logger.Infof("JSON-RPC2 connection handler finished")
	logger.Infof("Server stopped")
}

// findGopls looks for gopls binary in $PATH
func findGopls(logger lsp.Logger) string {
	path, err := exec.LookPath("gopls")
	if err != nil {
		logger.Debugf("gopls not found in $PATH: %v", err)
		return ""
	}
	logger.Infof("Found gopls at: %s", path)
	return path
}

// stdinoutCloser wraps os.Stdin and os.Stdout as ReadWriteCloser
type stdinoutCloser struct {
	stdin  *os.File
	stdout *os.File
	logger lsp.Logger
}

func (s *stdinoutCloser) Read(p []byte) (n int, err error) {
	n, err = s.stdin.Read(p)
	s.logger.Debugf("stdinoutCloser.Read: n=%d, err=%v", n, err)
	return n, err
}

func (s *stdinoutCloser) Write(p []byte) (n int, err error) {
	n, err = s.stdout.Write(p)
	if err == nil {
		// Explicitly flush to ensure VS Code receives the response immediately
		s.stdout.Sync()
	}
	s.logger.Debugf("stdinoutCloser.Write: n=%d, err=%v", n, err)
	return n, err
}

func (s *stdinoutCloser) Close() error {
	s.logger.Infof("stdinoutCloser.Close called")
	// Don't actually close stdin/stdout, but log the event
	return nil
}

var _ io.ReadWriteCloser = (*stdinoutCloser)(nil)

// rotateLogFile checks if log file exceeds maxLogSize and rotates it
// Keeps one backup (.old) for reference
func rotateLogFile(logFile string) {
	info, err := os.Stat(logFile)
	if err != nil {
		return // File doesn't exist, nothing to rotate
	}

	if info.Size() < maxLogSize {
		return // File is small enough
	}

	// Rotate: delete .old, rename current to .old
	oldFile := logFile + ".old"
	_ = os.Remove(oldFile)          // Ignore error if doesn't exist
	_ = os.Rename(logFile, oldFile) // Ignore error, will just append
}

