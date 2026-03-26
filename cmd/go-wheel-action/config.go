package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	modDir      string
	outputDir   string
	pkg         string
	ldflags     string
	rawName     string
	version     string
	description string
	url         string
	license     string
	readmePath  string
}

func loadConfig() (*config, error) {
	version := strings.TrimPrefix(os.Getenv("GOWHEEL_VERSION"), "v")
	if version == "" {
		return nil, errors.New("version input is required")
	}

	absModDir, err := filepath.Abs(cmp.Or(os.Getenv("GOWHEEL_MOD_DIR"), "."))
	if err != nil {
		return nil, fmt.Errorf("resolving mod-dir: %w", err)
	}

	return &config{
		modDir:      absModDir,
		outputDir:   cmp.Or(os.Getenv("GOWHEEL_OUTPUT_DIR"), "./dist"),
		pkg:         cmp.Or(os.Getenv("GOWHEEL_PACKAGE"), "."),
		ldflags:     cmp.Or(os.Getenv("GOWHEEL_LDFLAGS"), "-s"),
		rawName:     cmp.Or(os.Getenv("GOWHEEL_NAME"), filepath.Base(absModDir)),
		version:     version,
		description: os.Getenv("GOWHEEL_DESCRIPTION"),
		url:         os.Getenv("GOWHEEL_URL"),
		license:     os.Getenv("GOWHEEL_LICENSE"),
		readmePath:  cmp.Or(os.Getenv("GOWHEEL_README"), "README.md"),
	}, nil
}
