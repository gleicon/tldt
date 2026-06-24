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
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	// Write malformed TOML — should cause a parse error
	if _, err := f.WriteString("algorithm = bad toml [[["); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = f.Close()

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
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	content := "algorithm = \"textrank\"\nsentences = 7\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = f.Close()

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
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	// Only set algorithm — sentences must remain at default (5), not 0
	if _, err := f.WriteString("algorithm = \"ensemble\"\n"); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = f.Close()

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
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	if _, err := f.WriteString("sentences = 0\n"); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = f.Close()
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
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	content := "algorithm = \"lexrank\"\nunknown_key = \"ignored\"\n"
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = f.Close()

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
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	if _, err := f.WriteString("level = \"aggressive\"\n"); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = f.Close()

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

func TestStatsConfig(t *testing.T) {
	if !DefaultConfig().Stats.Enabled {
		t.Error("DefaultConfig().Stats.Enabled = false, want true")
	}

	// Absent [stats] section keeps the default (enabled).
	f, err := os.CreateTemp("", "tldt-stats-absent-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, _ = f.WriteString("sentences = 3\n")
	_ = f.Close()
	if !Load(f.Name()).Stats.Enabled {
		t.Error("Load(no [stats]): Stats.Enabled = false, want true (default)")
	}

	// Explicit opt-out flips it off.
	f2, err := os.CreateTemp("", "tldt-stats-off-*.toml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f2.Name()) })
	_, _ = f2.WriteString("[stats]\nenabled = false\n")
	_ = f2.Close()
	if Load(f2.Name()).Stats.Enabled {
		t.Error("Load([stats] enabled=false): Stats.Enabled = true, want false")
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

// ── [security] section ────────────────────────────────────────────────────────

func TestDefaultConfig_SecurityDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Security.DetectInjection {
		t.Error("default detect_injection must be false")
	}
	if cfg.Security.DetectPII {
		t.Error("default detect_pii must be false")
	}
	if cfg.Security.Sanitize {
		t.Error("default sanitize must be false")
	}
	if cfg.Security.SanitizePII {
		t.Error("default sanitize_pii must be false")
	}
	if cfg.Security.InjectionThreshold != 0.99 {
		t.Errorf("default injection_threshold = %.2f, want 0.99", cfg.Security.InjectionThreshold)
	}
}

func TestLoad_SecuritySection(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-security-*.toml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, _ = f.WriteString("[security]\ndetect_injection = true\ndetect_pii = true\ninjection_threshold = 0.85\n")
	_ = f.Close()

	cfg := Load(f.Name())
	if !cfg.Security.DetectInjection {
		t.Error("detect_injection: want true")
	}
	if !cfg.Security.DetectPII {
		t.Error("detect_pii: want true")
	}
	if cfg.Security.InjectionThreshold != 0.85 {
		t.Errorf("injection_threshold: want 0.85, got %.2f", cfg.Security.InjectionThreshold)
	}
}

func TestLoad_SecuritySection_ZeroThresholdFallsBack(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-threshold-zero-*.toml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, _ = f.WriteString("[security]\ninjection_threshold = 0\n")
	_ = f.Close()

	cfg := Load(f.Name())
	if cfg.Security.InjectionThreshold <= 0 {
		t.Errorf("zero threshold must fall back to default, got %.2f", cfg.Security.InjectionThreshold)
	}
}

func TestLoad_AbsentSecuritySectionKeepsDefaults(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-no-security-*.toml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, _ = f.WriteString("sentences = 3\n")
	_ = f.Close()

	cfg := Load(f.Name())
	if cfg.Security.DetectInjection || cfg.Security.DetectPII {
		t.Error("absent [security]: detection flags must remain false")
	}
	if cfg.Security.InjectionThreshold != 0.99 {
		t.Errorf("absent [security]: threshold = %.2f, want 0.99", cfg.Security.InjectionThreshold)
	}
}

// ── [ai_detection] section ────────────────────────────────────────────────────

func TestDefaultConfig_AIDetectionDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.AIDetection.Enabled {
		t.Error("default ai_detection.enabled must be false")
	}
	if cfg.AIDetection.Lang != "en" {
		t.Errorf("default ai_detection.lang = %q, want 'en'", cfg.AIDetection.Lang)
	}
	if cfg.AIDetection.WordlistDir != "" {
		t.Errorf("default ai_detection.wordlist_dir = %q, want ''", cfg.AIDetection.WordlistDir)
	}
}

func TestLoad_AIDetectionSection(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-aidetect-*.toml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, _ = f.WriteString("[ai_detection]\nenabled = true\nlang = \"pt-BR\"\nwordlist_dir = \"/custom/lists\"\n")
	_ = f.Close()

	cfg := Load(f.Name())
	if !cfg.AIDetection.Enabled {
		t.Error("ai_detection.enabled: want true")
	}
	if cfg.AIDetection.Lang != "pt-BR" {
		t.Errorf("ai_detection.lang: want 'pt-BR', got %q", cfg.AIDetection.Lang)
	}
	if cfg.AIDetection.WordlistDir != "/custom/lists" {
		t.Errorf("ai_detection.wordlist_dir: want '/custom/lists', got %q", cfg.AIDetection.WordlistDir)
	}
}

func TestLoad_AIDetectionLangEmpty_FallsBack(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-aidetect-nolang-*.toml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, _ = f.WriteString("[ai_detection]\nenabled = true\n")
	_ = f.Close()

	cfg := Load(f.Name())
	if cfg.AIDetection.Lang != "en" {
		t.Errorf("empty lang must fall back to 'en', got %q", cfg.AIDetection.Lang)
	}
}

func TestLoad_AbsentAIDetectionSectionKeepsDefaults(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-no-aidetect-*.toml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, _ = f.WriteString("sentences = 3\n")
	_ = f.Close()

	cfg := Load(f.Name())
	if cfg.AIDetection.Enabled {
		t.Error("absent [ai_detection]: enabled must remain false")
	}
	if cfg.AIDetection.Lang != "en" {
		t.Errorf("absent [ai_detection]: lang = %q, want 'en'", cfg.AIDetection.Lang)
	}
}

func TestLoad_FullConfig(t *testing.T) {
	f, err := os.CreateTemp("", "tldt-full-*.toml")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, _ = f.WriteString(strings.Join([]string{
		`algorithm = "textrank"`,
		`sentences = 8`,
		`format = "json"`,
		`[stats]`,
		`enabled = false`,
		`[security]`,
		`detect_injection = true`,
		`injection_threshold = 0.95`,
		`detect_pii = true`,
		`sanitize = true`,
		`sanitize_pii = false`,
		`[ai_detection]`,
		`enabled = true`,
		`lang = "es"`,
		`wordlist_dir = "/my/lists"`,
	}, "\n"))
	_ = f.Close()

	cfg := Load(f.Name())
	if cfg.Algorithm != "textrank" {
		t.Errorf("algorithm: want textrank, got %q", cfg.Algorithm)
	}
	if cfg.Sentences != 8 {
		t.Errorf("sentences: want 8, got %d", cfg.Sentences)
	}
	if cfg.Format != "json" {
		t.Errorf("format: want json, got %q", cfg.Format)
	}
	if cfg.Stats.Enabled {
		t.Error("stats.enabled: want false")
	}
	if !cfg.Security.DetectInjection {
		t.Error("security.detect_injection: want true")
	}
	if cfg.Security.InjectionThreshold != 0.95 {
		t.Errorf("security.injection_threshold: want 0.95, got %.2f", cfg.Security.InjectionThreshold)
	}
	if !cfg.Security.DetectPII {
		t.Error("security.detect_pii: want true")
	}
	if !cfg.Security.Sanitize {
		t.Error("security.sanitize: want true")
	}
	if cfg.Security.SanitizePII {
		t.Error("security.sanitize_pii: want false")
	}
	if !cfg.AIDetection.Enabled {
		t.Error("ai_detection.enabled: want true")
	}
	if cfg.AIDetection.Lang != "es" {
		t.Errorf("ai_detection.lang: want 'es', got %q", cfg.AIDetection.Lang)
	}
	if cfg.AIDetection.WordlistDir != "/my/lists" {
		t.Errorf("ai_detection.wordlist_dir: want '/my/lists', got %q", cfg.AIDetection.WordlistDir)
	}
}
