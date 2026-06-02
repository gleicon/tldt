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
	"strings"
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

	// ConfigDir overrides the Claude config directory base.
	// Precedence: ConfigDir > $CLAUDE_CONFIG_DIR > ~/.claude. Empty = use env or default.
	ConfigDir string

	// Project installs repo-locally under ./.claude/ and registers the hook in the
	// gitignored .claude/settings.local.json via $CLAUDE_PROJECT_DIR.
	Project bool
}

// installTarget describes one coding assistant's install locations.
type installTarget struct {
	name         string
	skillDest    string // path to write SKILL.md
	hookDest     string // path to write hook script; empty = no hook for this app
	hookCmd      string // command path registered in settings.json (may differ from hookDest for --project)
	settingsPath string // path to settings.json; empty = no hook registration
	pluginDest   string // path to write the OpenCode advisory plugin; empty = no plugin for this app
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

	// Codex uses a plugin+marketplace mechanism, handled apart from the
	// file-write targets below. A --target codex run installs only Codex.
	if codexTargeted(opts) {
		if err := installCodexPlugin(codexBaseDir(homeDir, opts)); err != nil {
			return fmt.Errorf("installing codex plugin: %w", err)
		}
		if opts.Target == "codex" {
			return nil
		}
	}

	targets, err := resolveTargets(homeDir, opts)
	if err != nil {
		return err
	}
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
			if err := PatchSettingsJSON(t.settingsPath, t.hookCmd); err != nil {
				return fmt.Errorf("patching settings.json for %s: %w", t.name, err)
			}
		}
		if t.pluginDest != "" {
			if err := installOpenCodePlugin(t.pluginDest); err != nil {
				return fmt.Errorf("installing plugin to %s: %w", t.name, err)
			}
		}
		fmt.Printf("installed to %s: %s\n", t.name, t.skillDest)
	}
	return nil
}

// resolveTargets returns the list of coding assistant install targets.
// Claude Code is included on the default run, --target all, or --target claude;
// a specific optional --target installs only that app. Optional apps are included
// if their base directory exists (or is explicitly targeted). opts.SkillDir
// overrides all detection. Returns an error if an explicitly targeted optional
// app's base directory cannot be created.
func resolveTargets(homeDir string, opts Options) ([]installTarget, error) {
	// --skill-dir override: single custom target, no hook registration
	if opts.SkillDir != "" {
		return []installTarget{{
			name:      "custom",
			skillDest: filepath.Join(opts.SkillDir, "tldt", "SKILL.md"),
		}}, nil
	}

	// --project: repo-local Claude install only. Hook is registered in the gitignored
	// settings.local.json via $CLAUDE_PROJECT_DIR so no machine path is committed.
	if opts.Project {
		return []installTarget{{
			name:         "claude",
			skillDest:    filepath.Join(".claude", "skills", "tldt", "SKILL.md"),
			hookDest:     filepath.Join(".claude", "hooks", "tldt-hook.sh"),
			hookCmd:      "$CLAUDE_PROJECT_DIR/.claude/hooks/tldt-hook.sh",
			settingsPath: filepath.Join(".claude", "settings.local.json"),
		}}, nil
	}

	// Claude Code is included on the default/all run or when explicitly targeted.
	// It is the only target that registers the UserPromptSubmit hook. A specific
	// optional target (e.g. --target opencode) must NOT drag in Claude.
	var targets []installTarget
	if opts.Target == "" || opts.Target == "all" || opts.Target == "claude" {
		base := claudeBaseDir(homeDir, opts)
		hookDest := filepath.Join(base, "hooks", "tldt-hook.sh")
		targets = append(targets, installTarget{
			name:         "claude",
			skillDest:    filepath.Join(base, "skills", "tldt", "SKILL.md"),
			hookDest:     hookDest,
			hookCmd:      hookDest,
			settingsPath: filepath.Join(base, "settings.json"),
		})
	}
	if opts.Target == "claude" {
		return targets, nil
	}

	// Optional apps: detect by base directory existence. OpenCode also gets the
	// advisory plugin; per OpenCode docs plugins live in plugins/ alongside
	// skills/. Cursor stays skill-only.
	optional := []struct {
		name       string
		detectDir  string
		skillDest  string
		pluginDest string
	}{
		{
			"cursor",
			filepath.Join(homeDir, ".cursor"),
			filepath.Join(homeDir, ".cursor", "skills", "tldt", "SKILL.md"),
			"",
		},
		{
			"opencode",
			filepath.Join(homeDir, ".config", "opencode"),
			filepath.Join(homeDir, ".config", "opencode", "skills", "tldt", "SKILL.md"),
			filepath.Join(homeDir, ".config", "opencode", "plugins", "tldt-advisory.js"),
		},
		{
			"agents",
			filepath.Join(homeDir, ".agents"),
			filepath.Join(homeDir, ".agents", "skills", "tldt", "SKILL.md"),
			"",
		},
	}

	for _, o := range optional {
		if opts.Target != "" && opts.Target != "all" && opts.Target != o.name {
			continue // --target restricts to specific app
		}
		_, err := os.Stat(o.detectDir)
		dirExists := err == nil
		// Auto-create directory when explicitly targeted (e.g., --target opencode)
		// This enables seamless first-time installation for OpenCode, Cursor, Agents.
		// A failure here on an explicit target is fatal — silently dropping it would
		// report a false success.
		if opts.Target == o.name && !dirExists {
			if err := os.MkdirAll(o.detectDir, 0755); err != nil {
				return nil, fmt.Errorf("creating base directory %q for target %q: %w", o.detectDir, o.name, err)
			}
			dirExists = true
		}
		if dirExists {
			targets = append(targets, installTarget{
				name:       o.name,
				skillDest:  o.skillDest,
				pluginDest: o.pluginDest,
				// No hookDest or settingsPath — these apps don't use the Claude/Codex
				// UserPromptSubmit shell hook; OpenCode gets the advisory plugin instead.
			})
		}
	}

	return targets, nil
}

// claudeBaseDir resolves the Claude config directory base:
// explicit --config-dir > $CLAUDE_CONFIG_DIR > the ~/.claude platform default.
func claudeBaseDir(homeDir string, opts Options) string {
	if opts.ConfigDir != "" {
		return opts.ConfigDir
	}
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return v
	}
	return filepath.Join(homeDir, ".claude")
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

// installOpenCodePlugin reads the embedded OpenCode advisory plugin and writes it
// to destPath. Creates all intermediate directories. Overwrites any existing file.
func installOpenCodePlugin(destPath string) error {
	data, err := EmbeddedFiles.ReadFile("opencode/tldt-advisory.js")
	if err != nil {
		return fmt.Errorf("embedded tldt-advisory.js not found: %w", err)
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
// Idempotent: any prior tldt registration is dropped and replaced, so the file
// always ends with exactly one tldt hook registration.
// hookCmd MUST be an absolute expanded path, or a portable $CLAUDE_PROJECT_DIR/-rooted
// path for --project installs. Truly relative paths are rejected.
func PatchSettingsJSON(settingsPath string, hookCmd string) error {
	if !filepath.IsAbs(hookCmd) && !strings.HasPrefix(hookCmd, "$CLAUDE_PROJECT_DIR/") {
		return fmt.Errorf("hookCmd must be an absolute or $CLAUDE_PROJECT_DIR-rooted path, got %q", hookCmd)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading settings.json: %w", err)
	}

	var settings map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("settings.json is not valid JSON: %w", err)
		}
	} else {
		settings = make(map[string]any)
	}

	// Navigate/create hooks. A present-but-wrong-typed "hooks" key is a user
	// config we must not silently overwrite — fail loudly instead.
	var hooks map[string]any
	if raw, present := settings["hooks"]; present {
		h, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("settings.json %q: hooks must be a JSON object, got %T; refusing to overwrite", settingsPath, raw)
		}
		hooks = h
	} else {
		hooks = make(map[string]any)
		settings["hooks"] = hooks
	}

	// Idempotency: check if hookCmd is already registered. A present-but-wrong-typed
	// UserPromptSubmit is likewise refused rather than clobbered.
	var existing []any
	if raw, present := hooks["UserPromptSubmit"]; present {
		ups, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("settings.json %q: hooks.UserPromptSubmit must be a JSON array, got %T; refusing to overwrite", settingsPath, raw)
		}
		existing = ups
	}
	// Drop any prior tldt registration (any command referencing tldt-hook.sh),
	// regardless of its exact path/format, so re-running upgrades in place and
	// leaves exactly one tldt registration. Co-located non-tldt
	// hooks in the same entry are preserved — we never clobber user config.
	var kept []any
	for _, e := range existing {
		m, ok := e.(map[string]any)
		if !ok {
			kept = append(kept, e)
			continue
		}
		hs, ok := m["hooks"].([]any)
		if !ok {
			kept = append(kept, e)
			continue
		}
		var keptHooks []any
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if !ok {
				keptHooks = append(keptHooks, h)
				continue
			}
			if cmd, _ := hm["command"].(string); strings.Contains(cmd, "tldt-hook.sh") {
				continue // stale tldt hook — drop it
			}
			keptHooks = append(keptHooks, h)
		}
		if len(keptHooks) == 0 {
			continue // entry held only tldt hooks — drop the whole entry
		}
		m["hooks"] = keptHooks
		kept = append(kept, m)
	}

	// Append exactly one current registration.
	newEntry := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCmd,
				"timeout": 30,
			},
		},
	}
	hooks["UserPromptSubmit"] = append(kept, newEntry)

	// Marshal with indentation (preserve human-readable format)
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings.json: %w", err)
	}

	// Atomic write: temp file then rename
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
