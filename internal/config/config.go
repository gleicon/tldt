// Package config loads per-user defaults from ~/.tldt.toml and exposes
// named level presets. All errors are absorbed; Load always returns a
// usable Config.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// StatsConfig holds settings for usage logging (~/.tldt/usage.jsonl).
type StatsConfig struct {
	Enabled bool `toml:"enabled"`
}

// SecurityConfig holds default flags for the security detection pipeline.
// All fields default to off; set to true to enable without a CLI flag.
type SecurityConfig struct {
	DetectInjection    bool    `toml:"detect_injection"`
	InjectionThreshold float64 `toml:"injection_threshold"`
	DetectPII          bool    `toml:"detect_pii"`
	Sanitize           bool    `toml:"sanitize"`
	SanitizePII        bool    `toml:"sanitize_pii"`
}

// AIDetectionConfig holds defaults for --detect-ai.
type AIDetectionConfig struct {
	Enabled     bool   `toml:"enabled"`
	Lang        string `toml:"lang"`
	WordlistDir string `toml:"wordlist_dir"`
}

// Config holds per-user default flags loaded from ~/.tldt.toml.
type Config struct {
	Algorithm   string            `toml:"algorithm"`
	Sentences   int               `toml:"sentences"`
	Format      string            `toml:"format"`
	Level       string            `toml:"level"`
	Stats       StatsConfig       `toml:"stats"`
	Security    SecurityConfig    `toml:"security"`
	AIDetection AIDetectionConfig `toml:"ai_detection"`
}

// DefaultConfig returns the built-in default configuration.
func DefaultConfig() Config {
	return Config{
		Algorithm: "lexrank",
		Sentences: 5,
		Format:    "text",
		Level:     "",
		Stats: StatsConfig{
			Enabled: true,
		},
		Security: SecurityConfig{
			InjectionThreshold: 0.99,
		},
		AIDetection: AIDetectionConfig{
			Lang: "en",
		},
	}
}

// LevelPresets maps named compression levels to sentence counts.
// "aggressive" means most compression (fewest sentences),
// "lite" means least compression (most sentences).
var LevelPresets = map[string]int{
	"lite":       10,
	"standard":   5,
	"aggressive": 3,
}

// Load reads cfgPath and returns the parsed Config. If the file does not
// exist or is malformed TOML, Load returns a fresh DefaultConfig() — it
// never returns an error. Unset fields in a valid file receive default values.
func Load(cfgPath string) Config {
	cfg := DefaultConfig()
	_, err := toml.DecodeFile(cfgPath, &cfg)
	if err != nil {
		return DefaultConfig()
	}
	if cfg.Sentences <= 0 {
		cfg.Sentences = DefaultConfig().Sentences
	}
	if cfg.Security.InjectionThreshold <= 0 {
		cfg.Security.InjectionThreshold = DefaultConfig().Security.InjectionThreshold
	}
	if cfg.AIDetection.Lang == "" {
		cfg.AIDetection.Lang = DefaultConfig().AIDetection.Lang
	}
	return cfg
}

// ConfigPath returns the path to the user config file (~/.tldt.toml).
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tldt.toml"), nil
}
