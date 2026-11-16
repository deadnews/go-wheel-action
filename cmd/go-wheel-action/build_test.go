package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestTargetExt(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{"linux", ""},
		{"darwin", ""},
		{"windows", ".exe"},
	}
	for _, tt := range tests {
		tgt := target{goos: tt.goos}
		if got := tgt.ext(); got != tt.want {
			t.Errorf("target{goos: %q}.ext() = %q, want %q", tt.goos, got, tt.want)
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

func TestBuildWheelMetadata(t *testing.T) {
	got := string(buildWheelMetadata([]string{"manylinux_2_17_x86_64", "musllinux_1_2_x86_64"}))
	want := "Wheel-Version: 1.0\nRoot-Is-Purelib: false\n" +
		"Tag: py3-none-manylinux_2_17_x86_64\nTag: py3-none-musllinux_1_2_x86_64\n"
	if got != want {
		t.Errorf("buildWheelMetadata:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestBuildMetadata(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		got := buildMetadata(&Config{rawName: "myapp", version: "1.0.0", readmePath: "nonexistent.md"})
		want := "Metadata-Version: 2.4\nName: myapp\nVersion: 1.0.0\nRequires-Python: >=3.10\n"
		if got != want {
			t.Errorf("buildMetadata minimal:\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("all fields", func(t *testing.T) {
		got := buildMetadata(&Config{
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

	t.Run("missing readme warns", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		stderr := os.Stderr
		os.Stderr = w
		t.Cleanup(func() { os.Stderr = stderr })

		buildMetadata(&Config{rawName: "myapp", version: "1.0.0", readmePath: "nonexistent.md"})
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}

		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		want := "warning: readme not included: open nonexistent.md: "
		if !strings.HasPrefix(string(out), want) {
			t.Errorf("stderr = %q, want prefix %q", out, want)
		}
	})

	t.Run("with readme", func(t *testing.T) {
		tmp := t.TempDir()
		readme := filepath.Join(tmp, "README.md")
		if err := os.WriteFile(readme, []byte("# Hello"), 0o644); err != nil {
			t.Fatal(err)
		}

		got := buildMetadata(&Config{rawName: "myapp", version: "1.0.0", readmePath: readme})
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

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tiny\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestCompile(t *testing.T) {
	modDir := setupTinyModule(t)
	tgt := target{goos: runtime.GOOS, goarch: runtime.GOARCH}

	t.Run("success", func(t *testing.T) {
		cfg := &Config{modDir: modDir, pkg: ".", ldflags: "-s", rawName: "tiny"}
		data, err := compile(cfg, tgt)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("expected binary data")
		}
	})

	t.Run("bad package", func(t *testing.T) {
		cfg := &Config{modDir: modDir, pkg: "./nonexistent", ldflags: "-s", rawName: "tiny"}
		if _, err := compile(cfg, tgt); err == nil {
			t.Fatal("expected error for bad package")
		}
	})
}

func TestBuildAllWheels(t *testing.T) {
	modDir := setupTinyModule(t)
	outputDir := t.TempDir()

	cfg := &Config{
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

	want := []string{
		"tiny-0.1.0-py3-none-manylinux_2_17_x86_64.musllinux_1_2_x86_64.whl",
		"tiny-0.1.0-py3-none-manylinux_2_17_aarch64.musllinux_1_2_aarch64.whl",
		"tiny-0.1.0-py3-none-macosx_10_9_x86_64.whl",
		"tiny-0.1.0-py3-none-macosx_11_0_arm64.whl",
		"tiny-0.1.0-py3-none-win_amd64.whl",
		"tiny-0.1.0-py3-none-win_arm64.whl",
	}
	if !slices.Equal(built, want) {
		t.Fatalf("built = %q, want %q", built, want)
	}

	for _, name := range built {
		bin := "tiny"
		if strings.Contains(name, "-win_") {
			bin += ".exe"
		}
		wantNames := []string{
			"tiny-0.1.0.data/scripts/" + bin,
			"tiny-0.1.0.dist-info/METADATA",
			"tiny-0.1.0.dist-info/WHEEL",
			"tiny-0.1.0.dist-info/RECORD",
		}
		got := zipNames(t, filepath.Join(outputDir, name))
		if !slices.Equal(got, wantNames) {
			t.Errorf("%s entries = %q, want %q", name, got, wantNames)
		}
	}
}

func zipNames(t *testing.T, path string) []string {
	t.Helper()

	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("opening wheel: %v", err)
	}
	defer r.Close()

	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}

	return names
}

func TestBuildAllWheelsBadPackage(t *testing.T) {
	modDir := setupTinyModule(t)
	outputDir := t.TempDir()

	cfg := &Config{
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

func TestBuildWheelBadOutputPath(t *testing.T) {
	entries := []entry{{path: "pkg-1.0.dist-info/METADATA", data: []byte("meta")}}
	err := buildWheel(entries, "pkg-1.0.dist-info/RECORD", "/nonexistent/dir/pkg.whl")
	if err == nil {
		t.Fatal("expected error for nonexistent output dir")
	}
}

func TestBuildWheel(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "pkg-1.0-py3-none-manylinux_2_17_x86_64.whl")

	entries := []entry{
		{path: "pkg-1.0.data/scripts/tool", data: []byte("binary"), exec: true},
		{path: "pkg-1.0.dist-info/METADATA", data: []byte("meta")},
		{path: "pkg-1.0.dist-info/WHEEL", data: []byte("wheel")},
	}

	if err := buildWheel(entries, "pkg-1.0.dist-info/RECORD", outPath); err != nil {
		t.Fatalf("buildWheel: %v", err)
	}

	record := "pkg-1.0.data/scripts/tool,sha256=mjpF0BUxog6JrGrhCwsL6wSSrNchajaKoGLRpf7K-c0,6\n" +
		"pkg-1.0.dist-info/METADATA,sha256=6jvXPitQbgBScjKz7XQ8Bm2oOo4wZvYqcedeubSqHbY,4\n" +
		"pkg-1.0.dist-info/WHEEL,sha256=ulmSYVnSqiVuuHObjafitXS5YOEgLG1iTL6YHO-ZbJE,5\n" +
		"pkg-1.0.dist-info/RECORD,,\n"
	want := slices.Concat(entries, []entry{
		{path: "pkg-1.0.dist-info/RECORD", data: []byte(record)},
	})

	r, err := zip.OpenReader(outPath)
	if err != nil {
		t.Fatalf("opening wheel: %v", err)
	}
	defer r.Close()

	if len(r.File) != len(want) {
		t.Fatalf("wheel has %d entries, want %d", len(r.File), len(want))
	}

	for i, f := range r.File {
		data := readZipFile(t, f)
		w := want[i]
		exec := f.Mode()&0o111 != 0
		if f.Name != w.path || !bytes.Equal(data, w.data) || exec != w.exec {
			t.Errorf("entry %d = {%s %q %t}, want {%s %q %t}",
				i, f.Name, data, exec, w.path, w.data, w.exec)
		}
		if f.Flags&0x8 != 0 {
			t.Errorf("%s has a data descriptor", f.Name)
		}
	}
}

func readZipFile(t *testing.T, f *zip.File) []byte {
	t.Helper()

	rc, err := f.Open()
	if err != nil {
		t.Fatalf("opening %s: %v", f.Name, err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading %s: %v", f.Name, err)
	}

	return data
}
