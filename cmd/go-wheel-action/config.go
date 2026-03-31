package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	version := normalizeVersion(os.Getenv("GOWHEEL_VERSION"))
	if version == "" {
		return nil, errors.New("version input is required")
	}

	absModDir, err := filepath.Abs(cmp.Or(os.Getenv("GOWHEEL_MOD_DIR"), "."))
	if err != nil {
		return nil, fmt.Errorf("resolving mod-dir: %w", err)
	}

	rawName := cmp.Or(os.Getenv("GOWHEEL_NAME"), filepath.Base(absModDir))
	if !nameRe.MatchString(rawName) {
		return nil, fmt.Errorf("name %q contains invalid characters", rawName)
	}

	return &config{
		modDir:      absModDir,
		outputDir:   cmp.Or(os.Getenv("GOWHEEL_OUTPUT_DIR"), "./dist"),
		pkg:         cmp.Or(os.Getenv("GOWHEEL_PACKAGE"), "."),
		ldflags:     cmp.Or(os.Getenv("GOWHEEL_LDFLAGS"), "-s"),
		rawName:     rawName,
		version:     version,
		description: os.Getenv("GOWHEEL_DESCRIPTION"),
		url:         os.Getenv("GOWHEEL_URL"),
		license:     os.Getenv("GOWHEEL_LICENSE"),
		readmePath:  cmp.Or(os.Getenv("GOWHEEL_README"), "README.md"),
	}, nil
}

var (
	preReleaseRe = regexp.MustCompile(`^(\d+(?:\.\d+)*)-(alpha|beta|rc|dev)(?:\.(\d+))?$`)
	nameRe       = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)
)

var pep440Tags = map[string]string{
	"alpha": "a",
	"beta":  "b",
	"rc":    "rc",
	"dev":   ".dev",
}

// normalizeVersion converts a semver version string to PEP 440.
func normalizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	m := preReleaseRe.FindStringSubmatch(v)
	if m == nil {
		return v
	}
	num := m[3]
	if num == "" {
		num = "0"
	}
	return m[1] + pep440Tags[m[2]] + num
}
