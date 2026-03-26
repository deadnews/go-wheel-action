// Build Python wheels from a Go module.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.outputDir, 0o750); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	built, err := buildAllWheels(cfg)
	if err != nil {
		return err
	}

	fmt.Printf("\nBuilt %d wheel(s) in %s\n", len(built), cfg.outputDir)

	return nil
}
