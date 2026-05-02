package config

import (
	"os"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Algorithm != "lexrank" {
		t.Errorf("DefaultConfig.Algorithm = %q, want %q", cfg.Algorithm, "lexrank")
	}
	if cfg.Sentences != 5 {
		t.Errorf("DefaultConfig.Sentences = %d, want 5", cfg.Sentences)
	}
	if cfg.Format != "text" {
		t.Errorf("DefaultConfig.Format = %q, want %q", cfg.Format, "text")
	}
	if cfg.Level != "" {
		t.Errorf("DefaultConfig.Level = %q, want %q", cfg.Level, "")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg := Load("/nonexistent/path/.tldt.toml")
	want := DefaultConfig()
	if cfg != want {
		t.Errorf("Load(missing file) = %+v, want %+v", cfg, want)
	}
}

func TestLoad_MalformedTOML(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-test-malformed-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })

	// Write malformed TOML — should cause a parse error
	if _, err := f.WriteString("algorithm = bad toml [[["); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()

	cfg := Load(f.Name())
	want := DefaultConfig()
	if cfg != want {
		t.Errorf("Load(malformed TOML) = %+v, want %+v (should return fresh DefaultConfig)", cfg, want)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-test-valid-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })

	content := "algorithm = \"textrank\"\nsentences = 7\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()

	cfg := Load(f.Name())
	if cfg.Algorithm != "textrank" {
		t.Errorf("Load(valid).Algorithm = %q, want %q", cfg.Algorithm, "textrank")
	}
	if cfg.Sentences != 7 {
		t.Errorf("Load(valid).Sentences = %d, want 7", cfg.Sentences)
	}
	// Unset fields get defaults
	if cfg.Format != "text" {
		t.Errorf("Load(valid).Format = %q, want %q (default)", cfg.Format, "text")
	}
	if cfg.Level != "" {
		t.Errorf("Load(valid).Level = %q, want %q (default)", cfg.Level, "")
	}
}

func TestLoad_PartialConfig(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-test-partial-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })

	// Only set algorithm — sentences must remain at default (5), not 0
	if _, err := f.WriteString("algorithm = \"ensemble\"\n"); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()

	cfg := Load(f.Name())
	if cfg.Algorithm != "ensemble" {
		t.Errorf("Load(partial).Algorithm = %q, want %q", cfg.Algorithm, "ensemble")
	}
	if cfg.Sentences != 5 {
		t.Errorf("Load(partial).Sentences = %d, want 5 (default, not zero)", cfg.Sentences)
	}
	if cfg.Format != "text" {
		t.Errorf("Load(partial).Format = %q, want %q (default)", cfg.Format, "text")
	}
	if cfg.Level != "" {
		t.Errorf("Load(partial).Level = %q, want %q (default)", cfg.Level, "")
	}
}

func TestLoad_ZeroSentences(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-test-zero-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.WriteString("sentences = 0\n"); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()
	cfg := Load(f.Name())
	if cfg.Sentences <= 0 {
		t.Errorf("Load(sentences=0): Sentences = %d, want > 0", cfg.Sentences)
	}
}

func TestLoad_UnknownKeys(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-test-unknown-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })

	content := "algorithm = \"lexrank\"\nunknown_key = \"ignored\"\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()

	cfg := Load(f.Name())
	if cfg.Algorithm != "lexrank" {
		t.Errorf("Load(unknown keys).Algorithm = %q, want %q", cfg.Algorithm, "lexrank")
	}
}

func TestLoad_LevelField(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-test-level-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })

	if _, err := f.WriteString("level = \"aggressive\"\n"); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()

	cfg := Load(f.Name())
	if cfg.Level != "aggressive" {
		t.Errorf("Load(level field).Level = %q, want %q", cfg.Level, "aggressive")
	}
	// Other fields should have defaults
	if cfg.Algorithm != "lexrank" {
		t.Errorf("Load(level field).Algorithm = %q, want %q (default)", cfg.Algorithm, "lexrank")
	}
	if cfg.Sentences != 5 {
		t.Errorf("Load(level field).Sentences = %d, want 5 (default)", cfg.Sentences)
	}
}

func TestLevelPresets(t *testing.T) {
	// aggressive = most compression = fewest sentences
	if v := LevelPresets["lite"]; v != 10 {
		t.Errorf("LevelPresets[\"lite\"] = %d, want 10", v)
	}
	if v := LevelPresets["standard"]; v != 5 {
		t.Errorf("LevelPresets[\"standard\"] = %d, want 5", v)
	}
	if v := LevelPresets["aggressive"]; v != 3 {
		t.Errorf("LevelPresets[\"aggressive\"] = %d, want 3", v)
	}
}

func TestLevelPresets_Unknown(t *testing.T) {
	v, ok := LevelPresets["bogus"]
	if ok {
		t.Errorf("LevelPresets[\"bogus\"] should not be present, got %d", v)
	}
	if v != 0 {
		t.Errorf("LevelPresets[\"bogus\"] = %d, want 0 (zero value for missing key)", v)
	}
}

func TestConfigPath(t *testing.T) {
	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() returned error: %v", err)
	}
	if !strings.HasSuffix(path, ".tldt.toml") {
		t.Errorf("ConfigPath() = %q, want path ending in \".tldt.toml\"", path)
	}
}

func TestHookConfig(t *testing.T) {
	// Default threshold
	d := DefaultConfig()
	if d.Hook.Threshold != 2000 {
		t.Errorf("DefaultConfig().Hook.Threshold = %d, want 2000", d.Hook.Threshold)
	}

	// TOML [hook] section loads correctly
	f, err := os.CreateTemp("", "tldt-hook-test-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	_, _ = f.WriteString("[hook]\nthreshold = 1500\n")
	f.Close()

	cfg := Load(f.Name())
	if cfg.Hook.Threshold != 1500 {
		t.Errorf("Load([hook] threshold=1500): Hook.Threshold = %d, want 1500", cfg.Hook.Threshold)
	}

	// Zero threshold guard
	f2, err := os.CreateTemp("", "tldt-hook-zero-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f2.Name()) })
	_, _ = f2.WriteString("[hook]\nthreshold = 0\n")
	f2.Close()

	cfg2 := Load(f2.Name())
	if cfg2.Hook.Threshold != 2000 {
		t.Errorf("Load([hook] threshold=0): Hook.Threshold = %d, want 2000 (guard)", cfg2.Hook.Threshold)
	}

	// Negative threshold guard
	f3, err := os.CreateTemp("", "tldt-hook-neg-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f3.Name()) })
	_, _ = f3.WriteString("[hook]\nthreshold = -5\n")
	f3.Close()

	cfg3 := Load(f3.Name())
	if cfg3.Hook.Threshold != 2000 {
		t.Errorf("Load([hook] threshold=-5): Hook.Threshold = %d, want 2000 (guard)", cfg3.Hook.Threshold)
	}
}
