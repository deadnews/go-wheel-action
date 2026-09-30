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
		{"1.0.0-alpha1", "1.0.0a1"},
		{"1.0.0-beta.0", "1.0.0b0"},
		{"1.0.0-beta.2", "1.0.0b2"},
		{"1.0.0-beta2", "1.0.0b2"},
		{"1.0.0-rc.0", "1.0.0rc0"},
		{"1.0.0-rc.1", "1.0.0rc1"},
		{"1.0.0-rc1", "1.0.0rc1"},
		{"1.0.0-dev.3", "1.0.0.dev3"},
		{"1.0.0-dev3", "1.0.0.dev3"},
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
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error for empty version")
		}
	})

	t.Run("invalid name", func(t *testing.T) {
		t.Setenv("GOWHEEL_VERSION", "1.0.0")
		for _, n := range []string{"foo/bar", "foo bar", "/leading", "trailing/"} {
			t.Setenv("GOWHEEL_NAME", n)
			_, err := LoadConfig()
			if err == nil {
				t.Errorf("expected error for name %q", n)
			}
		}
	})

	t.Run("normalizes version", func(t *testing.T) {
		t.Setenv("GOWHEEL_VERSION", "v1.0.0-alpha.0")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.version != "1.0.0a0" {
			t.Errorf("version = %q, want %q", cfg.version, "1.0.0a0")
		}
	})

	t.Run("defaults", func(t *testing.T) {
		for _, k := range []string{
			"MOD_DIR", "PACKAGE", "LDFLAGS", "NAME", "OUTPUT_DIR",
			"README", "DESCRIPTION", "URL", "LICENSE",
		} {
			t.Setenv("GOWHEEL_"+k, "")
		}
		t.Setenv("GOWHEEL_VERSION", "1.0.0")

		modDir, err := filepath.Abs(".")
		if err != nil {
			t.Fatal(err)
		}

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		want := Config{
			modDir:     modDir,
			outputDir:  "./dist",
			pkg:        ".",
			ldflags:    "-s",
			rawName:    filepath.Base(modDir),
			version:    "1.0.0",
			readmePath: "README.md",
		}
		if *cfg != want {
			t.Errorf("config = %+v, want %+v", *cfg, want)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		modDir := t.TempDir()
		t.Setenv("GOWHEEL_VERSION", "v2.0.0")
		t.Setenv("GOWHEEL_MOD_DIR", modDir)
		t.Setenv("GOWHEEL_PACKAGE", "./cmd/app")
		t.Setenv("GOWHEEL_LDFLAGS", "-s -w")
		t.Setenv("GOWHEEL_NAME", "myapp")
		t.Setenv("GOWHEEL_OUTPUT_DIR", "/tmp/wheels")
		t.Setenv("GOWHEEL_README", "docs/README.md")
		t.Setenv("GOWHEEL_DESCRIPTION", "My tool")
		t.Setenv("GOWHEEL_URL", "https://example.com")
		t.Setenv("GOWHEEL_LICENSE", "MIT")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		want := Config{
			modDir:      modDir,
			outputDir:   "/tmp/wheels",
			pkg:         "./cmd/app",
			ldflags:     "-s -w",
			rawName:     "myapp",
			version:     "2.0.0",
			description: "My tool",
			url:         "https://example.com",
			license:     "MIT",
			readmePath:  "docs/README.md",
		}
		if *cfg != want {
			t.Errorf("config = %+v, want %+v", *cfg, want)
		}
	})

	t.Run("drops NOASSERTION license", func(t *testing.T) {
		t.Setenv("GOWHEEL_VERSION", "1.0.0")
		t.Setenv("GOWHEEL_LICENSE", "NOASSERTION")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.license != "" {
			t.Errorf("license = %q, want empty", cfg.license)
		}
	})

	t.Run("name defaults to modDir basename", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("GOWHEEL_VERSION", "1.0.0")
		t.Setenv("GOWHEEL_MOD_DIR", dir)
		t.Setenv("GOWHEEL_NAME", "")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.rawName != filepath.Base(dir) {
			t.Errorf("rawName = %q, want %q", cfg.rawName, filepath.Base(dir))
		}
	})
}
