package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type platform struct {
	goos, goarch, tag string
}

func (p platform) ext() string {
	if p.goos == "windows" {
		return ".exe"
	}
	return ""
}

var platforms = []platform{
	{"linux", "amd64", "manylinux_2_17_x86_64"},
	{"linux", "amd64", "musllinux_1_2_x86_64"},
	{"linux", "arm64", "manylinux_2_17_aarch64"},
	{"linux", "arm64", "musllinux_1_2_aarch64"},
	{"darwin", "amd64", "macosx_10_9_x86_64"},
	{"darwin", "arm64", "macosx_11_0_arm64"},
	{"windows", "amd64", "win_amd64"},
	{"windows", "arm64", "win_arm64"},
}

const shimInit = `import os
import stat
import subprocess
import sys
from pathlib import Path


def main():
    binary = Path(__file__).parent / "bin" / "%s"

    if sys.platform != "win32":
        m = binary.stat().st_mode
        if not (m & stat.S_IXUSR):
            binary.chmod(m | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

    b = str(binary)

    if sys.platform == "win32":
        sys.exit(subprocess.call([b, *sys.argv[1:]]))
    else:
        os.execvp(b, [b, *sys.argv[1:]])
`

const shimMain = "from . import main; main()\n"

var specialRunRe = regexp.MustCompile(`[-._]+`)

// normalizeName applies PEP 625 normalization.
func normalizeName(name string) string {
	return strings.ToLower(specialRunRe.ReplaceAllString(name, "_"))
}

func buildMetadata(cfg *config) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Metadata-Version: 2.1\nName: %s\nVersion: %s\n", cfg.rawName, cfg.version)
	if cfg.description != "" {
		fmt.Fprintf(&b, "Summary: %s\n", cfg.description)
	}
	if cfg.url != "" {
		fmt.Fprintf(&b, "Home-page: %s\n", cfg.url)
	}
	if cfg.license != "" {
		fmt.Fprintf(&b, "License: %s\n", cfg.license)
	}
	fmt.Fprint(&b, "Requires-Python: >=3.10\n")

	if data, err := os.ReadFile(cfg.readmePath); err == nil {
		fmt.Fprintf(&b, "Description-Content-Type: text/markdown\n\n%s\n", string(data))
	}

	return b.String()
}

func compileGo(ctx context.Context, modDir, output, goos, goarch, pkg, ldflags string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-ldflags="+ldflags, "-o", output, pkg) //nolint:gosec // intentionally runs go build with user-provided flags
	cmd.Dir = modDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build %s/%s: %w", goos, goarch, err)
	}

	return nil
}

func buildAllWheels(cfg *config) ([]string, error) {
	tmpDir, err := os.MkdirTemp("", "go-wheel-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	normName := normalizeName(cfg.rawName)
	distInfo := fmt.Sprintf("%s-%s.dist-info", normName, cfg.version)
	metadata := buildMetadata(cfg)

	type buildKey struct{ goos, goarch string }
	cache := make(map[buildKey][]byte)
	built := make([]string, 0, len(platforms))

	for _, p := range platforms {
		key := buildKey{p.goos, p.goarch}

		binData, cached := cache[key]
		if !cached {
			binPath := filepath.Join(tmpDir, fmt.Sprintf("%s_%s_%s%s", cfg.rawName, p.goos, p.goarch, p.ext()))

			fmt.Printf("Building %s/%s...\n", p.goos, p.goarch)

			if err := compileGo(context.Background(), cfg.modDir, binPath, p.goos, p.goarch, cfg.pkg, cfg.ldflags); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
				cache[key] = nil

				continue
			}

			var err error

			binData, err = os.ReadFile(binPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
				cache[key] = nil

				continue
			}

			cache[key] = binData
		}

		if binData == nil {
			continue
		}

		binName := cfg.rawName + p.ext()

		files := map[string][]byte{
			normName + "/__init__.py":    fmt.Appendf(nil, shimInit, binName),
			normName + "/__main__.py":    []byte(shimMain),
			normName + "/bin/" + binName: binData,
			distInfo + "/METADATA":       []byte(metadata),
			distInfo + "/WHEEL": fmt.Appendf(nil,
				"Wheel-Version: 1.0\nRoot-Is-Purelib: false\nTag: py3-none-%s\n", p.tag),
			distInfo + "/entry_points.txt": fmt.Appendf(nil,
				"[console_scripts]\n%s = %s:main\n", cfg.rawName, normName),
		}

		whlName, err := buildWheel(files, normName, cfg.version, p.tag, cfg.outputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
			continue
		}

		built = append(built, whlName)
		fmt.Printf("  %s\n", whlName)
	}

	if len(built) == 0 {
		return nil, errors.New("no wheels were built")
	}

	return built, nil
}

func buildWheel(files map[string][]byte, name, version, tag, outputDir string) (string, error) {
	distInfo := fmt.Sprintf("%s-%s.dist-info", name, version)
	recordPath := distInfo + "/RECORD"

	var record strings.Builder
	for _, path := range slices.Sorted(maps.Keys(files)) {
		fmt.Fprintf(&record, "%s,%s,%d\n", path, sha256Base64(files[path]), len(files[path]))
	}
	fmt.Fprintf(&record, "%s,,\n", recordPath)
	files[recordPath] = []byte(record.String())

	whlName := fmt.Sprintf("%s-%s-py3-none-%s.whl", name, version, tag)

	f, err := os.Create(filepath.Join(outputDir, whlName))
	if err != nil {
		return "", fmt.Errorf("creating wheel file: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for _, path := range slices.Sorted(maps.Keys(files)) {
		header := &zip.FileHeader{Name: path, Method: zip.Deflate}
		if strings.Contains(path, "/bin/") {
			header.SetMode(0o755)
		}

		wr, err := w.CreateHeader(header)
		if err != nil {
			return "", fmt.Errorf("writing wheel entry %s: %w", path, err)
		}

		if _, err := wr.Write(files[path]); err != nil {
			return "", fmt.Errorf("writing wheel entry %s: %w", path, err)
		}
	}

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("finalizing wheel: %w", err)
	}

	if err := f.Close(); err != nil {
		return "", fmt.Errorf("closing wheel file: %w", err)
	}

	return whlName, nil
}

func sha256Base64(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256=" + base64.RawURLEncoding.EncodeToString(h[:])
}
