// Package config loads per-user defaults from ~/.tldt.toml and exposes
// named level presets. All errors are absorbed; Load always returns a
// usable Config.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds per-user default flags loaded from ~/.tldt.toml.
type Config struct {
	Algorithm string `toml:"algorithm"`
	Sentences int    `toml:"sentences"`
	Format    string `toml:"format"`
	Level     string `toml:"level"`
}

// DefaultConfig returns the built-in default configuration.
func DefaultConfig() Config {
	return Config{
		Algorithm: "lexrank",
		Sentences: 5,
		Format:    "text",
		Level:     "",
	}
}

// LevelPresets maps named compression levels to sentence counts.
var LevelPresets = map[string]int{
	"lite":       3,
	"standard":   5,
	"aggressive": 10,
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
