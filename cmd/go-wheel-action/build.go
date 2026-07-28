package main

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// target is one cross-compilation, shared by every wheel tag listed under it.
type target struct {
	goos, goarch string
	tags         []string
}

func (t target) ext() string {
	if t.goos == "windows" {
		return ".exe"
	}
	return ""
}

var targets = []target{
	{"linux", "amd64", []string{"manylinux_2_17_x86_64", "musllinux_1_2_x86_64"}},
	{"linux", "arm64", []string{"manylinux_2_17_aarch64", "musllinux_1_2_aarch64"}},
	{"darwin", "amd64", []string{"macosx_10_9_x86_64"}},
	{"darwin", "arm64", []string{"macosx_11_0_arm64"}},
	{"windows", "amd64", []string{"win_amd64"}},
	{"windows", "arm64", []string{"win_arm64"}},
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

const wheelTmpl = `Wheel-Version: 1.0
Root-Is-Purelib: false
Tag: py3-none-%s
`

const entryPointsTmpl = `[console_scripts]
%s = %s:main
`

// entry is one file in the wheel zip.
type entry struct {
	path string
	data []byte
	exec bool
}

var nameSepRe = regexp.MustCompile(`[-._]+`)

// normalizeName applies PEP 625 normalization.
func normalizeName(name string) string {
	return strings.ToLower(nameSepRe.ReplaceAllString(name, "_"))
}

func buildMetadata(cfg *Config) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Metadata-Version: 2.4\nName: %s\nVersion: %s\n", cfg.rawName, cfg.version)
	if cfg.description != "" {
		fmt.Fprintf(&b, "Summary: %s\n", cfg.description)
	}
	if cfg.url != "" {
		fmt.Fprintf(&b, "Project-URL: Repository, %s\n", cfg.url)
	}
	if cfg.license != "" {
		fmt.Fprintf(&b, "License-Expression: %s\n", cfg.license)
	}
	fmt.Fprint(&b, "Requires-Python: >=3.10\n")

	data, err := os.ReadFile(cfg.readmePath)
	switch {
	case err == nil:
		fmt.Fprintf(&b, "Description-Content-Type: text/markdown\n\n%s\n", data)
	case !os.IsNotExist(err):
		fmt.Fprintf(os.Stderr, "warning: reading readme %s: %v\n", cfg.readmePath, err)
	}

	return b.String()
}

// compile cross-compiles cfg.pkg and returns the binary bytes.
func compile(cfg *Config, t target) ([]byte, error) {
	fmt.Printf("Building %s/%s...\n", t.goos, t.goarch)

	tmpDir, err := os.MkdirTemp("", "go-wheel-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, cfg.rawName+t.ext())

	cmd := exec.CommandContext(context.Background(), "go", "build", "-ldflags="+cfg.ldflags, "-o", binPath, cfg.pkg) //nolint:gosec // intentionally runs go build with user-provided flags
	cmd.Dir = cfg.modDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+t.goos, "GOARCH="+t.goarch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go build %s/%s: %w", t.goos, t.goarch, err)
	}

	data, err := os.ReadFile(binPath) //nolint:gosec // G304: binPath is created in a private temp dir
	if err != nil {
		return nil, fmt.Errorf("read binary: %w", err)
	}

	return data, nil
}

func buildAllWheels(cfg *Config) ([]string, error) {
	if err := os.MkdirAll(cfg.outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	normName := normalizeName(cfg.rawName)
	distInfo := fmt.Sprintf("%s-%s.dist-info", normName, cfg.version)
	metadata := buildMetadata(cfg)

	var built []string

	for _, t := range targets {
		binData, err := compile(cfg, t)
		if err != nil {
			return nil, err
		}

		binName := cfg.rawName + t.ext()

		for _, tag := range t.tags {
			entries := []entry{
				{path: normName + "/__init__.py", data: fmt.Appendf(nil, shimInit, binName)},
				{path: normName + "/__main__.py", data: []byte(shimMain)},
				{path: normName + "/bin/" + binName, data: binData, exec: true},
				{path: distInfo + "/METADATA", data: []byte(metadata)},
				{path: distInfo + "/WHEEL", data: fmt.Appendf(nil, wheelTmpl, tag)},
				{path: distInfo + "/entry_points.txt", data: fmt.Appendf(nil, entryPointsTmpl, cfg.rawName, normName)},
			}

			whlName := fmt.Sprintf("%s-%s-py3-none-%s.whl", normName, cfg.version, tag)
			outPath := filepath.Join(cfg.outputDir, whlName)

			if err := buildWheel(entries, distInfo+"/RECORD", outPath); err != nil {
				return nil, err
			}

			built = append(built, whlName)
			fmt.Printf("  %s\n", whlName)
		}
	}

	return built, nil
}

func buildWheel(entries []entry, recordPath, outPath string) error {
	// RECORD lists each file's hash and size; its own entry has an empty hash.
	var record strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&record, "%s,%s,%d\n", e.path, sha256Base64(e.data), len(e.data))
	}
	fmt.Fprintf(&record, "%s,,\n", recordPath)

	f, err := os.Create(outPath) //nolint:gosec // G304: output path is user-provided by design
	if err != nil {
		return fmt.Errorf("create wheel file: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for _, e := range entries {
		if err := writeEntry(w, e); err != nil {
			return err
		}
	}

	if err := writeEntry(w, entry{path: recordPath, data: []byte(record.String())}); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("finalize wheel: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close wheel file: %w", err)
	}

	return nil
}

func writeEntry(w *zip.Writer, e entry) error {
	compressed := deflate(e.data)
	header := &zip.FileHeader{
		Name:               e.path,
		Method:             zip.Deflate,
		CRC32:              crc32.ChecksumIEEE(e.data),
		CompressedSize64:   uint64(len(compressed)),
		UncompressedSize64: uint64(len(e.data)),
	}
	if e.exec {
		header.SetMode(0o755)
	}

	// CreateRaw avoids ZIP data descriptors that PyPI rejects.
	wr, err := w.CreateRaw(header)
	if err != nil {
		return fmt.Errorf("write wheel entry %s: %w", e.path, err)
	}

	if _, err := wr.Write(compressed); err != nil {
		return fmt.Errorf("write wheel entry %s: %w", e.path, err)
	}

	return nil
}

func deflate(data []byte) []byte {
	var b bytes.Buffer
	w, _ := flate.NewWriter(&b, flate.DefaultCompression)
	w.Write(data) //nolint:errcheck,gosec // writes to bytes.Buffer cannot fail
	w.Close()     //nolint:errcheck,gosec // writes to bytes.Buffer cannot fail
	return b.Bytes()
}

func sha256Base64(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256=" + base64.RawURLEncoding.EncodeToString(h[:])
}
