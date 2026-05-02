// Package config loads per-user defaults from ~/.tldt.toml and exposes
// named level presets. All errors are absorbed; Load always returns a
// usable Config.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// HookConfig holds settings for the UserPromptSubmit auto-trigger hook.
type HookConfig struct {
	Threshold int `toml:"threshold"`
}

// Config holds per-user default flags loaded from ~/.tldt.toml.
type Config struct {
	Algorithm string     `toml:"algorithm"`
	Sentences int        `toml:"sentences"`
	Format    string     `toml:"format"`
	Level     string     `toml:"level"`
	Hook      HookConfig `toml:"hook"`
}

// DefaultConfig returns the built-in default configuration.
func DefaultConfig() Config {
	return Config{
		Algorithm: "lexrank",
		Sentences: 5,
		Format:    "text",
		Level:     "",
		Hook: HookConfig{
			Threshold: 2000,
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
	// Guard: zero/negative sentences in config file falls back to default
	if cfg.Sentences <= 0 {
		cfg.Sentences = DefaultConfig().Sentences
	}
	// Guard: zero/negative threshold falls back to default (D-11)
	if cfg.Hook.Threshold <= 0 {
		cfg.Hook.Threshold = DefaultConfig().Hook.Threshold
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
