// Package installer writes tldt skill and hook template files to
// Claude Code and other coding assistant directories.
// All errors are returned to the caller — Install() never silently swallows failures
// on required targets (Claude Code). Optional targets (Cursor, OpenCode, Agents)
// are skipped silently when their base directory is absent.
package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Options controls Install() behavior.
type Options struct {
	// SkillDir overrides the skill install directory.
	// When set, installs only to <SkillDir>/tldt/SKILL.md with no hook registration.
	// Empty = auto-detect all installed apps (default).
	SkillDir string

	// Target restricts install to a specific app: "claude", "cursor", "opencode", "agents", "all".
	// Empty = same as "all" (auto-detect).
	Target string
}

// installTarget describes one coding assistant's install locations.
type installTarget struct {
	name         string
	skillDest    string // path to write SKILL.md
	hookDest     string // path to write hook script; empty = no hook for this app
	settingsPath string // path to settings.json; empty = no hook registration
}

// Install writes skill files and registers the Claude Code hook.
// Claude Code is always targeted. Cursor, OpenCode, and Agents are
// targeted if their base directory exists on the filesystem.
// Returns an error if any required write or settings.json patch fails.
func Install(opts Options) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving home dir: %w", err)
	}

	targets := resolveTargets(homeDir, opts)
	if len(targets) == 0 {
		return fmt.Errorf("no install targets found")
	}

	for _, t := range targets {
		if err := installSkillFile(t.skillDest); err != nil {
			return fmt.Errorf("installing skill to %s: %w", t.name, err)
		}
		if t.hookDest != "" {
			if err := installHookFile(t.hookDest); err != nil {
				return fmt.Errorf("installing hook to %s: %w", t.name, err)
			}
			if err := PatchSettingsJSON(t.settingsPath, t.hookDest); err != nil {
				return fmt.Errorf("patching settings.json for %s: %w", t.name, err)
			}
		}
		fmt.Printf("installed to %s: %s\n", t.name, t.skillDest)
	}
	return nil
}

// resolveTargets returns the list of coding assistant install targets.
// Claude Code is always included. Optional apps are included if their
// base directory exists. opts.SkillDir overrides all detection.
func resolveTargets(homeDir string, opts Options) []installTarget {
	// --skill-dir override: single custom target, no hook registration (D-17)
	if opts.SkillDir != "" {
		return []installTarget{{
			name:      "custom",
			skillDest: filepath.Join(opts.SkillDir, "tldt", "SKILL.md"),
		}}
	}

	// Claude Code: always install (D-16); hook registered only here
	hookDest := filepath.Join(homeDir, ".claude", "hooks", "tldt-hook.sh")
	targets := []installTarget{{
		name:         "claude",
		skillDest:    filepath.Join(homeDir, ".claude", "skills", "tldt", "SKILL.md"),
		hookDest:     hookDest,
		settingsPath: filepath.Join(homeDir, ".claude", "settings.json"),
	}}

	// Filter by --target if set
	if opts.Target != "" && opts.Target != "all" {
		if opts.Target == "claude" {
			return targets
		}
		// Only the named target — claude base is still always included
		// but skip optional if target doesn't match
	}

	// Optional apps: detect by base directory existence (D-18)
	optional := []struct {
		name      string
		detectDir string
		skillDest string
	}{
		{
			"cursor",
			filepath.Join(homeDir, ".cursor"),
			filepath.Join(homeDir, ".cursor", "skills", "tldt", "SKILL.md"),
		},
		{
			"opencode",
			filepath.Join(homeDir, ".config", "opencode"),
			filepath.Join(homeDir, ".config", "opencode", "skills", "tldt", "SKILL.md"),
		},
		{
			"agents",
			filepath.Join(homeDir, ".agents"),
			filepath.Join(homeDir, ".agents", "skills", "tldt", "SKILL.md"),
		},
	}

	for _, o := range optional {
		if opts.Target != "" && opts.Target != "all" && opts.Target != o.name {
			continue // --target restricts to specific app
		}
		_, err := os.Stat(o.detectDir)
		dirExists := err == nil
		// Auto-create directory when explicitly targeted (e.g., --target opencode)
		// This enables seamless first-time installation for OpenCode, Cursor, Agents
		if opts.Target == o.name && !dirExists {
			if err := os.MkdirAll(o.detectDir, 0755); err == nil {
				dirExists = true
			}
		}
		if dirExists {
			targets = append(targets, installTarget{
				name:      o.name,
				skillDest: o.skillDest,
				// No hookDest or settingsPath — only Claude Code supports UserPromptSubmit hooks
			})
		}
	}

	return targets
}

// installSkillFile reads the embedded SKILL.md and writes it to destPath.
// Creates all intermediate directories. Overwrites any existing file.
func installSkillFile(destPath string) error {
	data, err := EmbeddedFiles.ReadFile("skills/tldt/SKILL.md")
	if err != nil {
		return fmt.Errorf("embedded SKILL.md not found: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", destPath, err)
	}
	return os.WriteFile(destPath, data, 0644)
}

// installHookFile reads the embedded hook script and writes it to destPath.
// Creates all intermediate directories. Sets mode 0755 (executable).
func installHookFile(destPath string) error {
	data, err := EmbeddedFiles.ReadFile("hooks/tldt-hook.sh")
	if err != nil {
		return fmt.Errorf("embedded tldt-hook.sh not found: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", destPath, err)
	}
	return os.WriteFile(destPath, data, 0755)
}

// PatchSettingsJSON reads the existing settings.json at settingsPath (or starts
// with an empty object if missing), merges the tldt UserPromptSubmit hook entry,
// and writes back using a temp-file-then-rename strategy for atomicity.
// Idempotent: if hookCmd is already registered, returns nil without modifying the file.
// hookCmd MUST be an absolute expanded path (not $HOME/...) per Pitfall 6.
func PatchSettingsJSON(settingsPath string, hookCmd string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading settings.json: %w", err)
	}

	var settings map[string]interface{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("settings.json is not valid JSON: %w", err)
		}
	} else {
		settings = make(map[string]interface{})
	}

	// Navigate/create hooks.UserPromptSubmit array
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
		settings["hooks"] = hooks
	}

	// Idempotency: check if hookCmd is already registered
	existing, _ := hooks["UserPromptSubmit"].([]interface{})
	for _, e := range existing {
		m, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		hs, ok := m["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, h := range hs {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if hm["command"] == hookCmd {
				return nil // already installed — no-op
			}
		}
	}

	// Append new hook entry
	newEntry := map[string]interface{}{
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": hookCmd,
				"timeout": 30,
			},
		},
	}
	hooks["UserPromptSubmit"] = append(existing, newEntry)

	// Marshal with indentation (preserve human-readable format)
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings.json: %w", err)
	}

	// Atomic write: temp file then rename (Pitfall 4 mitigation)
	tmpPath := settingsPath + ".tmp"
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("creating settings.json directory: %w", err)
	}
	if err := os.WriteFile(tmpPath, out, 0644); err != nil {
		return fmt.Errorf("writing temp settings file: %w", err)
	}
	if err := os.Rename(tmpPath, settingsPath); err != nil {
		return fmt.Errorf("renaming temp settings file: %w", err)
	}
	return nil
}

