package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// simpleLogger implements plugin.Logger
type simpleLogger struct{}

func (l *simpleLogger) Info(msg string)                   { fmt.Println("INFO:", msg) }
func (l *simpleLogger) Error(msg string)                  { fmt.Println("ERROR:", msg) }
func (l *simpleLogger) Debugf(format string, args ...any) { fmt.Printf("DEBUG: "+format+"\n", args...) }
func (l *simpleLogger) Warnf(format string, args ...any)  { fmt.Printf("WARN: "+format+"\n", args...) }

func runRegenerate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: regenerate <file.dingo>")
	}

	dingoFile := args[0]

	// Read Dingo source
	dingoSrc, err := os.ReadFile(dingoFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", dingoFile, err)
	}

	fset := token.NewFileSet()

	// Load config if exists
	var cfg *config.Config
	baseName := filepath.Base(dingoFile)
	baseName = baseName[:len(baseName)-len(".dingo")]
	testConfigDir := filepath.Join(filepath.Dir(dingoFile), baseName)
	testConfigPath := filepath.Join(testConfigDir, "dingo.toml")
	if _, err := os.Stat(testConfigPath); err == nil {
		cfg = config.DefaultConfig()
		if _, err := toml.DecodeFile(testConfigPath, cfg); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Create cache for unqualified import inference
	pkgDir := filepath.Dir(dingoFile)
	cache := preprocessor.NewFunctionExclusionCache(pkgDir)
	err = cache.ScanPackage([]string{dingoFile})

	// Create preprocessor
	var preprocessorInst *preprocessor.Preprocessor
	if err != nil {
		// Cache scan failed, fall back to no cache
		if cfg != nil {
			preprocessorInst = preprocessor.NewWithMainConfig(dingoSrc, cfg)
		} else {
			preprocessorInst = preprocessor.New(dingoSrc)
		}
	} else {
		// Cache scan successful, use it for unqualified imports
		preprocessorInst = preprocessor.NewWithCache(dingoSrc, cache)
	}

	// Preprocess
	preprocessed, _, err := preprocessorInst.Process()
	if err != nil {
		return fmt.Errorf("preprocessing failed: %w", err)
	}

	// Parse
	file, err := parser.ParseFile(fset, dingoFile, []byte(preprocessed), parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}

	// Create generator
	registry := plugin.NewRegistry()
	logger := &simpleLogger{}
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}

	// Generate Go code
	output, err := gen.Generate(file)
	if err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	// Write .go.golden file
	goldenFile := filepath.Join(filepath.Dir(dingoFile), baseName+".go.golden")

	if err := os.WriteFile(goldenFile, output, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", goldenFile, err)
	}

	fmt.Printf("✓ Regenerated %s\n", goldenFile)
	return nil
}
