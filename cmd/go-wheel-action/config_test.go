package main

import (
	"path/filepath"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"1.0", "1.0"},
		{"1.0.0", "1.0.0"},
		{"v1.0.0", "1.0.0"},
		{"v1.0.0-alpha.0", "1.0.0a0"},
		{"1.0.0-alpha.0", "1.0.0a0"},
		{"1.0.0-alpha.1", "1.0.0a1"},
		{"1.0.0-alpha", "1.0.0a0"},
		{"1.0.0-beta.0", "1.0.0b0"},
		{"1.0.0-beta.2", "1.0.0b2"},
		{"1.0.0-rc.0", "1.0.0rc0"},
		{"1.0.0-rc.1", "1.0.0rc1"},
		{"1.0.0-dev.3", "1.0.0.dev3"},
		{"1.0.0a1", "1.0.0a1"},
		{"1.0.0b2", "1.0.0b2"},
		{"1.0.0rc1", "1.0.0rc1"},
		{"1.0.0.dev0", "1.0.0.dev0"},
		{"1.0.post1", "1.0.post1"},
		{"1.0.dev1", "1.0.dev1"},
		{"2013.10", "2013.10"},
	}
	for _, tt := range tests {
		if got := normalizeVersion(tt.input); got != tt.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("version required", func(t *testing.T) {
		t.Setenv("GOWHEEL_VERSION", "")
		_, err := loadConfig()
		if err == nil {
			t.Fatal("expected error for empty version")
		}
	})

	t.Run("invalid name", func(t *testing.T) {
		t.Setenv("GOWHEEL_VERSION", "1.0.0")
		for _, n := range []string{"foo/bar", "foo bar", "/leading", "trailing/"} {
			t.Setenv("GOWHEEL_NAME", n)
			_, err := loadConfig()
			if err == nil {
				t.Errorf("expected error for name %q", n)
			}
		}
	})

	t.Run("normalizes version", func(t *testing.T) {
		t.Setenv("GOWHEEL_VERSION", "v1.0.0-alpha.0")
		cfg, err := loadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.version != "1.0.0a0" {
			t.Errorf("version = %q, want %q", cfg.version, "1.0.0a0")
		}
	})

	t.Run("defaults", func(t *testing.T) {
		t.Setenv("GOWHEEL_VERSION", "1.0.0")
		t.Setenv("GOWHEEL_MOD_DIR", "")
		t.Setenv("GOWHEEL_PACKAGE", "")
		t.Setenv("GOWHEEL_LDFLAGS", "")
		t.Setenv("GOWHEEL_NAME", "")
		t.Setenv("GOWHEEL_OUTPUT_DIR", "")
		t.Setenv("GOWHEEL_README", "")

		cfg, err := loadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.pkg != "." {
			t.Errorf("pkg = %q, want %q", cfg.pkg, ".")
		}
		if cfg.ldflags != "-s" {
			t.Errorf("ldflags = %q, want %q", cfg.ldflags, "-s")
		}
		if cfg.outputDir != "./dist" {
			t.Errorf("outputDir = %q, want %q", cfg.outputDir, "./dist")
		}
		if cfg.readmePath != "README.md" {
			t.Errorf("readmePath = %q, want %q", cfg.readmePath, "README.md")
		}
		if !filepath.IsAbs(cfg.modDir) {
			t.Errorf("modDir = %q, want absolute path", cfg.modDir)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		t.Setenv("GOWHEEL_VERSION", "v2.0.0")
		t.Setenv("GOWHEEL_MOD_DIR", t.TempDir())
		t.Setenv("GOWHEEL_PACKAGE", "./cmd/app")
		t.Setenv("GOWHEEL_LDFLAGS", "-s -w")
		t.Setenv("GOWHEEL_NAME", "myapp")
		t.Setenv("GOWHEEL_OUTPUT_DIR", "/tmp/wheels")
		t.Setenv("GOWHEEL_README", "docs/README.md")
		t.Setenv("GOWHEEL_DESCRIPTION", "My tool")
		t.Setenv("GOWHEEL_URL", "https://example.com")
		t.Setenv("GOWHEEL_LICENSE", "MIT")

		cfg, err := loadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.version != "2.0.0" {
			t.Errorf("version = %q, want %q", cfg.version, "2.0.0")
		}
		if cfg.pkg != "./cmd/app" {
			t.Errorf("pkg = %q, want %q", cfg.pkg, "./cmd/app")
		}
		if cfg.ldflags != "-s -w" {
			t.Errorf("ldflags = %q, want %q", cfg.ldflags, "-s -w")
		}
		if cfg.rawName != "myapp" {
			t.Errorf("rawName = %q, want %q", cfg.rawName, "myapp")
		}
		if cfg.outputDir != "/tmp/wheels" {
			t.Errorf("outputDir = %q, want %q", cfg.outputDir, "/tmp/wheels")
		}
		if cfg.description != "My tool" {
			t.Errorf("description = %q, want %q", cfg.description, "My tool")
		}
		if cfg.url != "https://example.com" {
			t.Errorf("url = %q, want %q", cfg.url, "https://example.com")
		}
		if cfg.license != "MIT" {
			t.Errorf("license = %q, want %q", cfg.license, "MIT")
		}
	})

	t.Run("name defaults to modDir basename", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("GOWHEEL_VERSION", "1.0.0")
		t.Setenv("GOWHEEL_MOD_DIR", dir)
		t.Setenv("GOWHEEL_NAME", "")

		cfg, err := loadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.rawName != filepath.Base(dir) {
			t.Errorf("rawName = %q, want %q", cfg.rawName, filepath.Base(dir))
		}
	})
}
