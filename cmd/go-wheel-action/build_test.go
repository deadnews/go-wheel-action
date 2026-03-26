package main

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlatformExt(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"linux", ""},
		{"darwin", ""},
		{"windows", ".exe"},
	}
	for _, tt := range tests {
		p := platform{goos: tt.goos}
		if got := p.ext(); got != tt.want {
			t.Errorf("platform{goos: %q}.ext() = %q, want %q", tt.goos, got, tt.want)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"myapp", "myapp"},
		{"My-App", "my_app"},
		{"my.app", "my_app"},
		{"my--app", "my_app"},
		{"my-._app", "my_app"},
		{"My..App--Name", "my_app_name"},
	}
	for _, tt := range tests {
		if got := normalizeName(tt.input); got != tt.want {
			t.Errorf("normalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSha256Base64(t *testing.T) {
	got := sha256Base64([]byte("hello"))
	want := "sha256=LPJNul-wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ"
	if got != want {
		t.Errorf("sha256Base64(%q) = %q, want %q", "hello", got, want)
	}
}

func TestBuildMetadata(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		got := buildMetadata(&config{rawName: "myapp", version: "1.0.0", readmePath: "nonexistent.md"})
		want := "Metadata-Version: 2.4\nName: myapp\nVersion: 1.0.0\nRequires-Python: >=3.10\n"
		if got != want {
			t.Errorf("buildMetadata minimal:\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("all fields", func(t *testing.T) {
		got := buildMetadata(&config{
			rawName:     "myapp",
			version:     "2.0.0",
			description: "A tool",
			url:         "https://example.com",
			license:     "MIT",
			readmePath:  "nonexistent.md",
		})
		for _, s := range []string{
			"Name: myapp",
			"Version: 2.0.0",
			"Summary: A tool",
			"Project-URL: Repository, https://example.com",
			"License-Expression: MIT",
			"Requires-Python: >=3.10",
		} {
			if !strings.Contains(got, s) {
				t.Errorf("buildMetadata missing %q in:\n%s", s, got)
			}
		}
	})

	t.Run("with readme", func(t *testing.T) {
		tmp := t.TempDir()
		readme := filepath.Join(tmp, "README.md")
		if err := os.WriteFile(readme, []byte("# Hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		got := buildMetadata(&config{rawName: "myapp", version: "1.0.0", readmePath: readme})
		if !strings.Contains(got, "# Hello") {
			t.Error("buildMetadata should include readme content")
		}
		if !strings.Contains(got, "Description-Content-Type: text/markdown") {
			t.Error("buildMetadata should include content type header")
		}
	})
}

func setupTinyModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tiny\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestCompileGo(t *testing.T) {
	modDir := setupTinyModule(t)
	output := filepath.Join(t.TempDir(), "out")

	t.Run("success", func(t *testing.T) {
		err := compileGo(context.Background(), modDir, output, runtime.GOOS, runtime.GOARCH, ".", "-s")
		if err != nil {
			t.Fatalf("compileGo: %v", err)
		}
		if _, err := os.Stat(output); err != nil {
			t.Fatalf("output binary missing: %v", err)
		}
	})

	t.Run("bad package", func(t *testing.T) {
		err := compileGo(context.Background(), modDir, output, runtime.GOOS, runtime.GOARCH, "./nonexistent", "-s")
		if err == nil {
			t.Fatal("expected error for bad package")
		}
	})
}

func TestBuildAllWheels(t *testing.T) {
	modDir := setupTinyModule(t)
	outputDir := t.TempDir()

	cfg := &config{
		modDir:    modDir,
		outputDir: outputDir,
		pkg:       ".",
		ldflags:   "-s",
		rawName:   "tiny",
		version:   "0.1.0",
	}

	built, err := buildAllWheels(cfg)
	if err != nil {
		t.Fatalf("buildAllWheels: %v", err)
	}
	if len(built) == 0 {
		t.Fatal("expected at least one wheel")
	}

	for _, name := range built {
		path := filepath.Join(outputDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("wheel file missing: %s", path)
		}
	}
}

func TestBuildAllWheelsBadPackage(t *testing.T) {
	modDir := setupTinyModule(t)
	outputDir := t.TempDir()

	cfg := &config{
		modDir:    modDir,
		outputDir: outputDir,
		pkg:       "./nonexistent",
		ldflags:   "-s",
		rawName:   "tiny",
		version:   "0.1.0",
	}

	_, err := buildAllWheels(cfg)
	if err == nil {
		t.Fatal("expected error for bad package")
	}
}

func TestBuildWheelBadOutputDir(t *testing.T) {
	files := map[string][]byte{"pkg/__init__.py": []byte("init")}
	_, err := buildWheel(files, "pkg", "1.0", "manylinux_2_17_x86_64", "/nonexistent/dir")
	if err == nil {
		t.Fatal("expected error for nonexistent output dir")
	}
}

func TestBuildWheel(t *testing.T) {
	outputDir := t.TempDir()

	files := map[string][]byte{
		"pkg/__init__.py":            []byte("init"),
		"pkg/bin/tool":               []byte("binary"),
		"pkg-1.0.dist-info/METADATA": []byte("meta"),
		"pkg-1.0.dist-info/WHEEL":    []byte("wheel"),
	}

	whlName, err := buildWheel(files, "pkg", "1.0", "manylinux_2_17_x86_64", outputDir)
	if err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	if want := "pkg-1.0-py3-none-manylinux_2_17_x86_64.whl"; whlName != want {
		t.Errorf("wheel name = %q, want %q", whlName, want)
	}

	r, err := zip.OpenReader(filepath.Join(outputDir, whlName))
	if err != nil {
		t.Fatalf("opening wheel: %v", err)
	}
	defer r.Close()

	names := make(map[string]bool)
	for _, f := range r.File {
		names[f.Name] = true
	}

	for _, want := range []string{
		"pkg/__init__.py",
		"pkg/bin/tool",
		"pkg-1.0.dist-info/METADATA",
		"pkg-1.0.dist-info/WHEEL",
		"pkg-1.0.dist-info/RECORD",
	} {
		if !names[want] {
			t.Errorf("wheel missing entry %q", want)
		}
	}

	recordFile, err := r.Open("pkg-1.0.dist-info/RECORD")
	if err != nil {
		t.Fatalf("opening RECORD: %v", err)
	}
	defer recordFile.Close()

	recordData, err := io.ReadAll(recordFile)
	if err != nil {
		t.Fatalf("reading RECORD: %v", err)
	}
	record := string(recordData)

	if !strings.Contains(record, "pkg-1.0.dist-info/RECORD,,") {
		t.Error("RECORD should have empty hash for itself")
	}
	if !strings.Contains(record, "sha256=") {
		t.Error("RECORD should contain sha256 hashes")
	}

	for _, f := range r.File {
		if f.Name == "pkg/bin/tool" {
			mode := f.Mode()
			if mode&0o111 == 0 {
				t.Errorf("binary entry mode %o should be executable", mode)
			}
		}
	}
}
