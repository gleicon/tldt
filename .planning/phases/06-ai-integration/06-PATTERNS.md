# Phase 6: AI Integration - Pattern Map

**Mapped:** 2026-05-02
**Files analyzed:** 8 new/modified files
**Analogs found:** 6 / 8

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/config/config.go` | config | request-response | `internal/config/config.go` (self — extend) | exact |
| `cmd/tldt/main.go` | utility/CLI entry | request-response | `cmd/tldt/main.go` (self — extend) | exact |
| `internal/installer/embed.go` | utility | file-I/O | none — first go:embed usage | no analog |
| `internal/installer/installer.go` | service | file-I/O | `internal/fetcher/fetcher.go` | role-match |
| `internal/installer/installer_test.go` | test | file-I/O | `internal/config/config_test.go` | role-match |
| `skills/tldt/SKILL.md` | config/template | — | `~/.claude/skills/*/SKILL.md` (live examples) | exact (external) |
| `hooks/tldt-hook.sh` | utility/hook | event-driven | `~/.claude/hooks/*.sh` (live examples) | role-match (external) |
| `Makefile` | config | — | `Makefile` (self — extend) | exact |

---

## Pattern Assignments

### `internal/config/config.go` (config, extend)

**Analog:** `internal/config/config.go` (self — add `Hook` sub-struct)

**Existing imports** (lines 1–11 — no changes needed):
```go
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)
```

**Existing Config struct** (lines 14–19 — extend by adding Hook field):
```go
// Current — extend this
type Config struct {
	Algorithm string `toml:"algorithm"`
	Sentences int    `toml:"sentences"`
	Format    string `toml:"format"`
	Level     string `toml:"level"`
}
```

**New sub-struct to add** (insert before Config, after LevelPresets):
```go
// HookConfig holds settings for the UserPromptSubmit hook.
type HookConfig struct {
	Threshold int `toml:"threshold"`
}
```

**Extended Config struct** (replace existing):
```go
type Config struct {
	Algorithm string     `toml:"algorithm"`
	Sentences int        `toml:"sentences"`
	Format    string     `toml:"format"`
	Level     string     `toml:"level"`
	Hook      HookConfig `toml:"hook"`
}
```

**Extended DefaultConfig** (lines 22–29 — add Hook default):
```go
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
```

**Load function** (lines 43–54 — unchanged; TOML decoder fills Hook.Threshold from `[hook]` section automatically):
```go
func Load(cfgPath string) Config {
	cfg := DefaultConfig()
	_, err := toml.DecodeFile(cfgPath, &cfg)
	if err != nil {
		return DefaultConfig()
	}
	if cfg.Sentences <= 0 {
		cfg.Sentences = DefaultConfig().Sentences
	}
	return cfg
}
```

**Guard to add after Sentences guard** (add threshold guard):
```go
// Guard: zero/negative threshold falls back to default
if cfg.Hook.Threshold <= 0 {
	cfg.Hook.Threshold = DefaultConfig().Hook.Threshold
}
```

---

### `cmd/tldt/main.go` (CLI entry, extend)

**Analog:** `cmd/tldt/main.go` (self — add flags and early-exit dispatches)

**Existing flag declaration block** (lines 20–37 — copy pattern; add two new flags at bottom of block):
```go
// Existing pattern — all flags declared at top of main()
filePath := flag.String("f", "", "input file path")
urlFlag  := flag.String("url", "", "URL of a webpage to fetch and summarize")
// ... existing flags ...

// ADD these two at the bottom of the flag block:
printThreshold := flag.Bool("print-threshold", false, "print configured hook token threshold to stdout and exit")
installSkill   := flag.Bool("install-skill", false, "install tldt Claude Code skill and UserPromptSubmit hook")
skillDir       := flag.String("skill-dir", "", "override skill install directory (default: ~/.claude/skills/tldt/)")
skillTarget    := flag.String("target", "", "install target app: claude|cursor|opencode|agents|all (default: all detected)")
```

**Existing config-load block** (lines 40–47 — unchanged; add dispatches immediately after):
```go
// Load config file — silent fallback to defaults on any error (CFG-03).
cfgPath, _ := config.ConfigPath()
cfg := config.Load(cfgPath)

// Detect which flags the user explicitly provided (CFG-02).
flagsSet := make(map[string]bool)
flag.Visit(func(f *flag.Flag) { flagsSet[f.Name] = true })
```

**New early-exit dispatches** (add immediately after flagsSet block, before effectiveAlgorithm):
```go
// --print-threshold: print integer threshold to stdout and exit (D-10, D-12)
if *printThreshold {
	fmt.Println(cfg.Hook.Threshold)
	os.Exit(0)
}

// --install-skill: install skill + hook files and exit (D-13, D-16)
if *installSkill {
	if err := installer.Install(installer.Options{
		SkillDir: *skillDir,
		Target:   *skillTarget,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "install-skill:", err)
		os.Exit(1)
	}
	os.Exit(0)
}
```

**Import to add** (add to import block):
```go
"github.com/gleicon/tldt/internal/installer"
```

**Existing error handling pattern** (lines 86–98 — copy for installer error):
```go
// Existing error pattern to follow:
if err != nil {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
```

**Existing flag.Usage pattern** (lines 32–37 — update the usage string to include new flags):
```go
flag.Usage = func() {
	fmt.Fprintln(os.Stderr, "Usage: tldt [--print-threshold] [--install-skill [--skill-dir path] [--target app]] ...")
	fmt.Fprintln(os.Stderr, "       cat file.txt | tldt")
	flag.PrintDefaults()
	os.Exit(2) // POSIX convention
}
```

---

### `internal/installer/embed.go` (utility, file-I/O)

**Analog:** none — first go:embed usage in this repo

**Pattern from RESEARCH.md** (Pattern 5 — go:embed, verified from stdlib docs):
```go
// File: internal/installer/embed.go
package installer

import "embed"

// EmbeddedFiles holds the skill and hook templates compiled into the binary.
// Directories must be siblings of this file (go:embed does not support .. paths).
//
//go:embed skills hooks
var EmbeddedFiles embed.FS
```

**Critical constraint** (Pitfall 3 from RESEARCH.md): The `skills/` and `hooks/` directories that are embedded must live at `internal/installer/skills/` and `internal/installer/hooks/` — NOT at repo root — because go:embed paths are relative to the Go source file and `..` is not permitted.

**Directory layout required:**
```
internal/installer/
├── embed.go
├── installer.go
├── installer_test.go
├── skills/
│   └── tldt/
│       └── SKILL.md
└── hooks/
    └── tldt-hook.sh
```

---

### `internal/installer/installer.go` (service, file-I/O)

**Analog:** `internal/fetcher/fetcher.go` (role-match: both are self-contained packages that perform I/O and return errors; fetcher does HTTP I/O, installer does filesystem I/O)

**Package doc + import pattern** from `internal/fetcher/fetcher.go` lines 1–18:
```go
// Package installer writes tldt skill and hook template files to
// Claude Code and other coding assistant directories.
// All errors are returned to the caller — Install() never silently swallows failures.
package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)
```

**Options struct** (new — no analog; follow Config struct style from `internal/config/config.go` lines 14–19):
```go
// Options controls Install() behavior.
type Options struct {
	SkillDir string // override skill install directory; empty = all detected apps
	Target   string // "claude"|"cursor"|"opencode"|"agents"|"all"; empty = all detected
}
```

**Core install function pattern** (modeled after fetcher.go's single exported function with error return):
```go
// Install writes skill files and registers the Claude Code hook.
// It detects installed apps and installs to each unless Options.Target restricts it.
// Returns an error if any required write fails; optional targets are skipped silently.
func Install(opts Options) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving home dir: %w", err)
	}

	// Determine install targets
	targets := resolveTargets(homeDir, opts)

	for _, t := range targets {
		if err := installSkillFile(t.skillDest); err != nil {
			return fmt.Errorf("installing skill to %s: %w", t.name, err)
		}
		if t.hookDest != "" {
			if err := installHookFile(t.hookDest); err != nil {
				return fmt.Errorf("installing hook to %s: %w", t.name, err)
			}
			if err := PatchSettingsJSON(t.settingsPath, t.hookDest); err != nil {
				return fmt.Errorf("patching settings.json: %w", err)
			}
		}
		fmt.Printf("installed to %s: %s\n", t.name, t.skillDest)
	}
	return nil
}
```

**File write helper** (from RESEARCH.md Pattern 5 code example):
```go
func installSkillFile(destPath string) error {
	data, err := EmbeddedFiles.ReadFile("skills/tldt/SKILL.md")
	if err != nil {
		return fmt.Errorf("embedded file not found: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}
```

**settings.json patch pattern** (from RESEARCH.md Pattern 6 — read-merge-write; never overwrite):
```go
// PatchSettingsJSON reads existing settings.json, merges the tldt hook entry
// into hooks.UserPromptSubmit, and writes back. Idempotent.
func PatchSettingsJSON(settingsPath string, hookCmd string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var settings map[string]interface{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("settings.json is not valid JSON: %w", err)
		}
	} else {
		settings = make(map[string]interface{})
	}

	// Navigate/create: settings["hooks"]["UserPromptSubmit"]
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
		settings["hooks"] = hooks
	}

	newEntry := map[string]interface{}{
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": hookCmd,  // MUST be absolute path — Pitfall 6
				"timeout": 30,
			},
		},
	}

	existing, _ := hooks["UserPromptSubmit"].([]interface{})
	// Idempotency: skip if this command is already registered
	for _, e := range existing {
		if m, ok := e.(map[string]interface{}); ok {
			if hs, ok := m["hooks"].([]interface{}); ok {
				for _, h := range hs {
					if hm, ok := h.(map[string]interface{}); ok {
						if hm["command"] == hookCmd {
							return nil // already installed
						}
					}
				}
			}
		}
	}
	hooks["UserPromptSubmit"] = append(existing, newEntry)

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	// Write to temp file then rename for atomicity (Pitfall 4 mitigation)
	tmpPath := settingsPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, settingsPath)
}
```

**Multi-app target detection pattern** (from RESEARCH.md Multi-App Install Targets):
```go
type installTarget struct {
	name         string
	skillDest    string
	hookDest     string // empty = no hook for this app
	settingsPath string // empty = no settings.json for this app
}

func resolveTargets(homeDir string, opts Options) []installTarget {
	// Always install Claude Code (primary)
	skillFilename := filepath.Join(homeDir, ".claude", "skills", "tldt", "SKILL.md")
	hookFilename  := filepath.Join(homeDir, ".claude", "hooks", "tldt-hook.sh")
	settingsFile  := filepath.Join(homeDir, ".claude", "settings.json")

	targets := []installTarget{{
		name:         "claude",
		skillDest:    skillFilename,
		hookDest:     hookFilename,
		settingsPath: settingsFile,
	}}

	// Optional: detect and install Cursor, OpenCode, Agents
	optional := []struct {
		name      string
		detectDir string
		skillDir  string
	}{
		{"cursor",  filepath.Join(homeDir, ".cursor"),              filepath.Join(homeDir, ".cursor", "skills", "tldt", "SKILL.md")},
		{"opencode", filepath.Join(homeDir, ".config", "opencode"), filepath.Join(homeDir, ".config", "opencode", "skills", "tldt", "SKILL.md")},
		{"agents",  filepath.Join(homeDir, ".agents"),              filepath.Join(homeDir, ".agents", "skills", "tldt", "SKILL.md")},
	}
	for _, o := range optional {
		if _, err := os.Stat(o.detectDir); err == nil {
			targets = append(targets, installTarget{
				name:      o.name,
				skillDest: o.skillDir,
				// no hookDest/settingsPath — only Claude Code has UserPromptSubmit hooks
			})
		}
	}

	// --skill-dir overrides all detection
	if opts.SkillDir != "" {
		return []installTarget{{
			name:      "custom",
			skillDest: filepath.Join(opts.SkillDir, "tldt", "SKILL.md"),
		}}
	}

	return targets
}
```

---

### `internal/installer/installer_test.go` (test, file-I/O)

**Analog:** `internal/config/config_test.go` (role-match: both test a package that reads/writes files; config_test.go uses `os.CreateTemp` + `t.Cleanup` pattern)

**Package declaration and imports** from `internal/config/config_test.go` lines 1–7:
```go
package installer

import (
	"os"
	"path/filepath"
	"testing"
)
```

**Temp directory pattern** (adapt from config_test.go `os.CreateTemp` + `t.Cleanup`):
```go
func TestInstallSkillFile_WritesFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-installer-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	destPath := filepath.Join(tmpDir, "skills", "tldt", "SKILL.md")
	if err := installSkillFile(destPath); err != nil {
		t.Fatalf("installSkillFile: unexpected error: %v", err)
	}
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading installed skill: %v", err)
	}
	if len(data) == 0 {
		t.Error("installed SKILL.md is empty")
	}
}
```

**Error case pattern** (from config_test.go TestLoad_MissingFile lines 25–30):
```go
func TestInstallSkillFile_MkdirAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-installer-mkdirall-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Deep path — MkdirAll must create all intermediate dirs
	destPath := filepath.Join(tmpDir, "a", "b", "c", "SKILL.md")
	if err := installSkillFile(destPath); err != nil {
		t.Errorf("installSkillFile(deep path): unexpected error: %v", err)
	}
}
```

**JSON patch idempotency test** (pattern: write, patch, read, verify; patch again, verify no duplicate):
```go
func TestPatchSettingsJSON_Idempotent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tldt-settings-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	settingsPath := filepath.Join(tmpDir, "settings.json")
	hookCmd := "/usr/local/bin/tldt-hook.sh"

	// First patch — creates settings.json
	if err := PatchSettingsJSON(settingsPath, hookCmd); err != nil {
		t.Fatalf("first patch: %v", err)
	}

	// Second patch — must be a no-op (idempotent)
	if err := PatchSettingsJSON(settingsPath, hookCmd); err != nil {
		t.Fatalf("second patch: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	// Count occurrences of hookCmd — must appear exactly once
	count := strings.Count(string(data), hookCmd)
	if count != 1 {
		t.Errorf("hook command appears %d times in settings.json, want 1", count)
	}
}
```

---

### `skills/tldt/SKILL.md` (config/template, file-I/O)

**Analog:** Live examples at `~/.claude/skills/*/SKILL.md` (80+ verified files, per RESEARCH.md)

**SKILL.md format** (verified schema from RESEARCH.md Pattern 1):
```yaml
---
name: tldt
description: "Summarize long text with tldt — reduces token count before sending to AI"
argument-hint: "<text to summarize>"
allowed-tools:
  - Bash
---

Summarize the text in $ARGUMENTS using the local tldt binary.

Run this command:

```bash
echo "$ARGUMENTS" | tldt --verbose 2>&1
```

The first line of output shows token savings (~X -> ~Y tokens, Z% reduction).
The remaining lines are the extractive summary.

Return the complete output — savings line first, then summary.
```

**Critical:** `allowed-tools: [Bash]` is required because the skill runs a shell command. `--verbose` is required (Pitfall 1 from RESEARCH.md: stats only print when `--verbose` is set, per main.go lines 158–169).

**File location in repo:** `internal/installer/skills/tldt/SKILL.md` (inside the installer package so go:embed can reference it — NOT at repo root; see Pitfall 3).

---

### `hooks/tldt-hook.sh` (utility, event-driven)

**Analog:** `~/.claude/hooks/*.sh` (live GSD hook examples; bash, event-driven, D-05)

**Hook script structure** (from RESEARCH.md Pattern 4 — verified bash pattern):
```bash
#!/usr/bin/env bash
# tldt-hook.sh — UserPromptSubmit hook for Claude Code
# Auto-summarizes prompts exceeding the configured token threshold.
# Exit 0 silently if tldt is not in PATH (D-08).

set -euo pipefail

# Require tldt in PATH — exit 0 silently if absent (D-08)
if ! command -v tldt >/dev/null 2>&1; then
  exit 0
fi

# Read JSON from stdin (Claude Code provides event as JSON on stdin, D-07)
INPUT=$(cat)

# Extract prompt text — jq primary, python3 fallback (Pitfall 2: JSON must be safe-encoded)
if command -v jq >/dev/null 2>&1; then
  PROMPT=$(printf '%s' "$INPUT" | jq -r '.prompt // empty' 2>/dev/null)
else
  PROMPT=$(printf '%s' "$INPUT" | python3 -c \
    "import json,sys; d=json.load(sys.stdin); print(d.get('prompt',''), end='')" 2>/dev/null || true)
fi

# Empty prompt — no-op
if [ -z "$PROMPT" ]; then
  exit 0
fi

# Token estimate: chars / 4 heuristic (D-10, matches tldt's TokenizeSentences)
CHAR_COUNT=$(printf '%s' "$PROMPT" | wc -c | tr -d ' ')
TOKEN_ESTIMATE=$(( CHAR_COUNT / 4 ))

# Get threshold from tldt config (reads ~/.tldt.toml [hook] threshold, D-10)
THRESHOLD=$(tldt --print-threshold 2>/dev/null || echo "2000")

# Below threshold — pass through silently
if [ "$TOKEN_ESTIMATE" -lt "$THRESHOLD" ]; then
  exit 0
fi

# Summarize — capture stderr (savings line) and stdout (summary) (D-04, D-06)
STATS_FILE=$(mktemp)
SUMMARY=$(printf '%s' "$PROMPT" | tldt --verbose 2>"$STATS_FILE" || true)
SAVINGS=$(cat "$STATS_FILE")
rm -f "$STATS_FILE"

# If summarization failed or returned empty, pass through silently
if [ -z "$SUMMARY" ]; then
  exit 0
fi

# Build replacement context: savings line first, then summary (D-04, D-06)
REPLACEMENT="${SAVINGS}

${SUMMARY}"

# Output hookSpecificOutput JSON for Claude Code (use python3 to safely encode, Pitfall 2)
printf '%s' "$REPLACEMENT" | python3 -c "
import json, sys
content = sys.stdin.read()
output = {
  'hookSpecificOutput': {
    'hookEventName': 'UserPromptSubmit',
    'additionalContext': content
  }
}
print(json.dumps(output))
"
```

**File location in repo:** `internal/installer/hooks/tldt-hook.sh` (inside installer package for go:embed).

**Key constraints:**
- `--verbose` flag required on tldt call (Pitfall 1 — stats only printed with `--verbose`)
- python3 for JSON encoding, not bash string interpolation (Pitfall 2 — special chars break naive JSON)
- Must use absolute path when registered in settings.json (Pitfall 6 — subdirectory bug)

---

### `Makefile` (config, extend)

**Analog:** `Makefile` (self — add `install-skill` target)

**Existing target pattern** (lines 45–47 — copy style):
```makefile
## install: install binary to GOPATH/bin
install:
	go install $(CMD)
```

**New target to add** (follow same `## comment: description` + single tab indent convention):
```makefile
## install-skill: install tldt Claude Code skill and hook
install-skill: build
	./$(BINARY) --install-skill
```

**Existing `.PHONY` line** (line 1 — add `install-skill`):
```makefile
.PHONY: build test test-uat install install-skill clean deps lint run help
```

---

## Shared Patterns

### Error Handling
**Source:** `internal/fetcher/fetcher.go` and `cmd/tldt/main.go`
**Apply to:** `internal/installer/installer.go`, `cmd/tldt/main.go` additions

Pattern: wrap errors with `fmt.Errorf("context: %w", err)`; never swallow errors in library code; only `main()` calls `os.Exit`.

```go
// Library code: return wrapped error
if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
	return fmt.Errorf("creating directory for %q: %w", destPath, err)
}

// main(): print to stderr and exit 1
if err := installer.Install(opts); err != nil {
	fmt.Fprintln(os.Stderr, "install-skill:", err)
	os.Exit(1)
}
```

### Silent Fallback (no-op on absent optional resource)
**Source:** `internal/config/config.go` `Load()` lines 43–54
**Apply to:** `internal/installer/installer.go` optional app detection

Config silently returns `DefaultConfig()` when file is missing. Installer silently skips optional apps when their directory is absent. Both follow "never fail on optional resource" principle.

```go
// Config pattern (lines 43–48):
func Load(cfgPath string) Config {
	cfg := DefaultConfig()
	_, err := toml.DecodeFile(cfgPath, &cfg)
	if err != nil {
		return DefaultConfig() // silent fallback
	}
	// ...
}

// Installer analog: skip optional app if dir absent
if _, err := os.Stat(o.detectDir); err == nil {
	targets = append(targets, ...) // only if dir exists
}
// no else — silently skip
```

### Stdout vs Stderr discipline
**Source:** `cmd/tldt/main.go` lines 158–169
**Apply to:** `internal/installer/installer.go` (print installed paths), `cmd/tldt/main.go` new flags

Rule: stdout = machine-readable output only (threshold integer, install path list). stderr = errors. `--print-threshold` must print only the bare integer to stdout (no label text).

```go
// --print-threshold: bare integer only on stdout (D-12)
if *printThreshold {
	fmt.Println(cfg.Hook.Threshold) // stdout — bare integer, no label
	os.Exit(0)
}

// installer errors go to stderr (main.go pattern lines 86–90):
if err != nil {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
```

### Test Temp File Pattern
**Source:** `internal/config/config_test.go` lines 26–52
**Apply to:** `internal/installer/installer_test.go`

Use `os.CreateTemp` / `os.MkdirTemp` + `t.Cleanup(func() { os.Remove/RemoveAll(...) })`. Never leave temp files on disk.

```go
// config_test.go lines 26-31 (canonical pattern):
f, err := os.CreateTemp("", "tldt-test-*.toml")
if err != nil {
	t.Fatalf("creating temp file: %v", err)
}
t.Cleanup(func() { os.Remove(f.Name()) })
```

### Flag Early-Exit Dispatch
**Source:** `cmd/tldt/main.go` — implied by existing `flag.Visit` + flagsSet pattern (lines 44–84)
**Apply to:** `cmd/tldt/main.go` new `--print-threshold` and `--install-skill` flags

Pattern: load config first (lines 40–47), then dispatch early-exit flags before computing effective parameters. New dispatches go immediately after the `flag.Visit` block (before `effectiveAlgorithm` declarations).

```go
// Existing dispatch zone (after flag.Visit block, lines ~48-84):
// Effective parameter resolution follows flagsSet...
// NEW: insert early exits here, before effectiveAlgorithm =
if *printThreshold {
	fmt.Println(cfg.Hook.Threshold)
	os.Exit(0)
}
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/installer/embed.go` | utility | file-I/O | First use of `go:embed` in this repo; no existing embed pattern to copy from |

**Planner note:** Use RESEARCH.md Pattern 5 directly for `embed.go` — the pattern is simple (4 lines) and fully specified there.

---

## Metadata

**Analog search scope:** `internal/`, `cmd/`, `Makefile`, live `~/.claude/skills/*/SKILL.md`
**Files scanned:** 12 Go source files + Makefile + live Claude Code skill examples
**Pattern extraction date:** 2026-05-02
