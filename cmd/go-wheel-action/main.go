// Package main provides go-wheel-action, which packages Go binaries as Python wheels.
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
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	built, err := buildAllWheels(cfg)
	if err != nil {
		return err
	}

	fmt.Printf("\nBuilt %d wheel(s) in %s\n", len(built), cfg.outputDir)

	return nil
}
