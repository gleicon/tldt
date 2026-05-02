# Phase 6: AI Integration — Research

**Researched:** 2026-05-02
**Domain:** Claude Code skill file format, hook event system, Go embed, multi-app install targets
**Confidence:** HIGH (all critical format claims verified against live examples on this machine or official docs)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Skill invokes tldt via stdin pipe — `echo "$text" | tldt`. Uses existing pipe-safe design.
- **D-02:** User triggers via `/tldt` slash command inside Claude Code.
- **D-03:** Skill passes no explicit flags to tldt — reads `~/.tldt.toml`.
- **D-04:** Skill captures stderr and shows it in conversation; token savings line appears first.
- **D-05:** Hook script in bash — zero dependencies.
- **D-06:** When threshold exceeded, hook replaces prompt with summary + savings line. Original discarded.
- **D-07:** Hook receives prompt text via stdin.
- **D-08:** If tldt not in PATH, hook exits 0 silently.
- **D-09:** Token threshold in `~/.tldt.toml` under `[hook]` section: `threshold = 2000`.
- **D-10:** Hook reads threshold by calling `tldt --print-threshold`.
- **D-11:** Default threshold: 2000 tokens.
- **D-12:** Config struct gains `Hook struct { Threshold int }` (default 2000); `--print-threshold` prints it.
- **D-13:** Primary install: `tldt --install-skill` CLI flag.
- **D-14:** Skill + hook templates embedded via `go:embed`.
- **D-15:** Source files at `skills/` and `hooks/` at repo root.
- **D-16:** Default install: `~/.claude/skills/tldt/SKILL.md` + hook in `~/.claude/settings.json`.
- **D-17:** `--skill-dir <path>` flag for non-default targets.
- **D-18:** Researcher MUST investigate multi-app install targets.

### Claude's Discretion

- Hook bash token counting: use `wc -c` divided by 4 (chars/4 heuristic).
- Skill file SKILL.md structure: follow standard Claude Code skill format.
- JSON structure for hook registration in `~/.claude/settings.json`.

### Deferred Ideas (OUT OF SCOPE)

- MCP server mode — deferred to v3+
- Clipboard auto-read (`pbpaste`/`xclip`) — deferred to v3+
- `--url` authentication headers / cookie support — deferred to v3+
- TOML validation/lint command (`tldt --check-config`) — deferred to v3+
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| AI-01 | User can install a Claude Code skill file that invokes the local `tldt` binary on selected or pasted text | SKILL.md format verified from live examples; install path confirmed |
| AI-02 | AI skill passes text to `tldt` via stdin and returns the summary inline in the conversation | Pipe pattern `echo "$text" \| tldt 2>&1` documented below |
| AI-03 | Auto-trigger hook fires when input text exceeds a configurable token count threshold | UserPromptSubmit hook format verified from live `settings.json` |
| AI-04 | Auto-trigger summarizes the oversized input and reports token savings before inserting the summary into the AI context | `additionalContext` injection pattern confirmed; savings line from tldt stderr |
</phase_requirements>

---

## Summary

Phase 6 delivers three artifacts: a Claude Code SKILL.md file (and compatible copies for Cursor and OpenCode), a bash hook script registered under `UserPromptSubmit` in `~/.claude/settings.json`, and a `tldt --install-skill` CLI command backed by `go:embed` that writes all files and patches settings.json.

The SKILL.md format is **verified from 80+ live examples** on this machine at `~/.claude/skills/*/SKILL.md`. The format is straightforward YAML frontmatter (name, description, optional argument-hint and allowed-tools) followed by markdown body. No bash blocks are used — the body is instructions to Claude, not shell code.

The UserPromptSubmit hook receives the entire Claude Code event as JSON on stdin. The key field is `.prompt` — the raw text the user submitted. The hook reads this via `jq -r '.prompt'` (or a portable bash fallback using `python3 -c`), counts tokens with `wc -c / 4`, compares against `tldt --print-threshold`, summarizes via pipe if over threshold, and outputs a `hookSpecificOutput` JSON blob that injects the savings line + summary as `additionalContext`.

**Primary recommendation:** Use the exact SKILL.md schema and settings.json hook format verified from live GSD examples on this machine. Do not invent schemas — follow what works.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Skill file (SKILL.md) | Filesystem / Claude Code runtime | — | Claude Code reads skill files from well-known dirs at session start |
| Auto-trigger hook | Claude Code hook runtime | Bash subprocess | Hook runs as external process; Claude Code provides JSON context |
| Token threshold config | Config file (`~/.tldt.toml`) | Go binary (`--print-threshold`) | Config is user-side; binary abstracts TOML parsing from bash hook |
| Install command | Go binary (CLI) | Filesystem writes | Binary embeds templates, detects app dirs, patches JSON |
| JSON patching (settings.json) | Go binary (`--install-skill`) | — | Must handle idempotency; Go's `encoding/json` handles it safely |
| Multi-app install | Go binary (`--install-skill`) | Detection heuristics | Binary detects installed apps by checking `~/.cursor`, `~/.config/opencode`, etc. |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `embed` (stdlib) | Go 1.16+ (project uses 1.26.2) | Embed skill/hook templates in binary | Zero-dep; available since go 1.16; no third-party needed |
| `encoding/json` (stdlib) | Go stdlib | Read/patch `~/.claude/settings.json` | Required for safe JSON manipulation; `jq` not reliable in Go |
| `os`, `path/filepath` (stdlib) | Go stdlib | File I/O, path resolution, dir creation | Standard pattern throughout this codebase |

[VERIFIED: live go.mod — go 1.26.2; embed available since 1.16]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `jq` (bash tool) | 1.5 (installed) | Hook reads `.prompt` from JSON stdin | Hook script — available on this machine; include python3 fallback |
| `python3` (bash tool) | Available | JSON parsing fallback if jq absent | Add as fallback in hook script since `jq` may not be universal |

[VERIFIED: `jq --version` → jq-1.5; `python3 --version` → available on this machine]

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `jq` in hook | Pure bash string parsing | `jq` is cleaner but not always installed; python3 fallback covers the gap |
| Full JSON rewrite for settings.json | `jq` CLI | Go stdlib is more portable; avoids requiring `jq` at install time |
| Single SKILL.md | Per-app SKILL.md variants | One SKILL.md works for Claude Code, Cursor, and OpenCode — all use identical format |

**Installation:**
```bash
# No new Go dependencies needed — embed and encoding/json are stdlib
go build ./...
```

---

## Architecture Patterns

### System Architecture Diagram

```
User runs: tldt --install-skill [--target <app>] [--skill-dir <path>]
                    │
                    ▼
        embed.FS (compiled into binary)
        ├── skills/tldt/SKILL.md
        └── hooks/tldt-hook.sh
                    │
            ┌───────┴──────────────┐
            ▼                      ▼
    Detect installed apps   Write skill files
    (check ~/.cursor,       to per-app dirs
     ~/.config/opencode,    (or --skill-dir)
     ~/.claude)
            │                      │
            ▼                      ▼
    Patch ~/.claude/        mkdir -p + write
    settings.json           SKILL.md file
    (add UserPromptSubmit
     hook entry)
            │
            ▼
    Print install summary
    (paths written, apps targeted)
```

```
Claude Code session — UserPromptSubmit hook fires:
    User submits prompt
            │
            ▼
    ~/.claude/hooks/tldt-hook.sh
    ├── Read stdin JSON → .prompt field
    ├── Count chars: wc -c / 4 = estimated tokens
    ├── Get threshold: tldt --print-threshold (reads ~/.tldt.toml [hook] threshold)
    ├── If tokens < threshold → exit 0 (no-op, Claude proceeds normally)
    └── If tokens >= threshold:
            │
            ▼
        echo "$PROMPT" | tldt 2>/tmp/tldt-stats.txt
        (summary → stdout, savings line → stderr captured)
            │
            ▼
        Build hookSpecificOutput JSON:
        { "hookSpecificOutput": {
            "hookEventName": "UserPromptSubmit",
            "additionalContext": "<savings line>\n<summary>"
          }
        }
        → output to stdout → Claude sees it as injected context
```

```
User types /tldt in Claude Code session:
    /tldt invoked (ARGUMENTS = selected or typed text)
            │
            ▼
    SKILL.md instructions tell Claude to:
    echo "$ARGUMENTS" | tldt 2>&1
    (bash block executed via Claude Code)
            │
            ▼
    stdout: summary text
    stderr (merged via 2>&1): savings line ~X -> ~Y tokens (Z% reduction)
            │
            ▼
    Claude shows both in conversation:
    savings line first, then summary
```

### Recommended Project Structure

```
tldt/
├── skills/
│   └── tldt/
│       └── SKILL.md          # skill template — go:embed target
├── hooks/
│   └── tldt-hook.sh          # UserPromptSubmit hook template — go:embed target
├── internal/
│   ├── config/
│   │   └── config.go         # ADD: Hook struct with Threshold int
│   └── installer/
│       └── installer.go      # NEW: InstallSkill(), PatchSettingsJSON()
├── cmd/
│   └── tldt/
│       ├── main.go           # ADD: --install-skill, --print-threshold flags
│       └── embed.go          # NEW: //go:embed skills/* hooks/*
```

### Pattern 1: SKILL.md Format (Claude Code / Cursor / OpenCode)

**What:** A markdown file with YAML frontmatter that Claude Code reads as a slash command.
**When to use:** This is the only format — all three apps use identical structure.

**Verified from:** Live examples at `~/.claude/skills/*/SKILL.md` — 80+ files inspected. Specifically verified from `gsd-add-tests/SKILL.md` (simple) and `caveman/SKILL.md` (content-only).

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
echo "$ARGUMENTS" | tldt 2>&1
```

The first line of output shows token savings (~X -> ~Y tokens, Z% reduction).
The remaining lines are the extractive summary.

Return the complete output — savings line first, then summary.
```

**Key schema fields verified from live examples:**

| Field | Type | Required | Verified Values |
|-------|------|----------|-----------------|
| `name` | string | YES | `tldt` — becomes `/tldt` slash command |
| `description` | string | YES | Shown in Claude Code's command picker |
| `argument-hint` | string | NO | Shown as placeholder when user types `/tldt` |
| `allowed-tools` | list | NO | Must include `Bash` if skill runs shell commands |

[VERIFIED: `~/.claude/skills/gsd-add-tests/SKILL.md`, `~/.claude/skills/caveman/SKILL.md` — inspected directly]

**Important:** The SKILL.md body is **instructions to Claude** (markdown prose), not a bash script. If the skill needs to run a command, it instructs Claude to use the `Bash` tool.

### Pattern 2: UserPromptSubmit Hook — settings.json Format

**What:** A JSON entry in the `hooks.UserPromptSubmit` array in `~/.claude/settings.json`.
**When to use:** Auto-trigger when user submits any prompt.

**Verified from:** Live `~/.claude/settings.json` on this machine (read directly). Existing hooks use `PreToolUse` and `PostToolUse` — `UserPromptSubmit` not yet present. Format is structurally identical to existing entries.

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$HOME/.claude/hooks/tldt-hook.sh",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

**Verified structure from existing PreToolUse hooks:**
```json
"PreToolUse": [
  {
    "matcher": "WebFetch",
    "hooks": [
      {
        "type": "command",
        "command": "/path/to/hook.sh"
      }
    ]
  }
]
```

**UserPromptSubmit difference:** No `matcher` field — fires on every prompt submission.

[VERIFIED: Live `~/.claude/settings.json` read directly; `UserPromptSubmit` hook type confirmed by web search cross-reference with official docs]

**JSON merge requirement:** `--install-skill` MUST read the existing `settings.json` and merge the new hook entry — never overwrite. If `UserPromptSubmit` array already exists, append to it. If `settings.json` doesn't exist, create it with just the hook entry.

### Pattern 3: UserPromptSubmit Hook — stdin JSON and output

**What:** The JSON Claude Code sends to the hook script on stdin, and how the hook responds.

**Verified from:** Multiple official-adjacent sources; the `.prompt` field name confirmed via web search cross-referencing the Claude Code docs hooks reference.

**Stdin JSON structure:**
```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/current/working/directory",
  "prompt": "The full text the user typed and submitted"
}
```

**Hook response — inject additionalContext:**
```json
{
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "~3,200 -> ~480 tokens (85% reduction)\n\n[summary text here]"
  }
}
```

**Hook response — block prompt:**
```json
{
  "decision": "block",
  "reason": "Prompt exceeds threshold; summary injected via additionalContext"
}
```

**Note (D-06):** The decision is locked: hook **replaces prompt with summary + savings line**. The mechanism is: output `additionalContext` via `hookSpecificOutput` with the replacement content. The original prompt is replaced because the hook consumed it and the only content Claude sees is what the hook injects. Per D-06, original text is discarded.

[CITED: https://code.claude.com/docs/en/hooks — UserPromptSubmit output format]
[CITED: https://gist.github.com/FrancisBourre/50dca37124ecc43eaf08328cdcccdb34 — hook input/output schemas]

### Pattern 4: Hook Bash Script Structure

**What:** The bash script that runs on every prompt submission.
**Key constraints:** Zero external deps (D-05); exits 0 if tldt absent (D-08); reads threshold via `tldt --print-threshold` (D-10).

```bash
#!/usr/bin/env bash
# tldt-hook.sh — UserPromptSubmit hook
# Auto-summarizes prompts exceeding the configured token threshold.
# Exit 0 silently if tldt is not in PATH.

set -euo pipefail

# Require tldt in PATH — exit 0 silently if absent (D-08)
if ! command -v tldt >/dev/null 2>&1; then
  exit 0
fi

# Read JSON from stdin (Claude Code provides event as JSON)
INPUT=$(cat)

# Extract prompt text — use jq if available, python3 as fallback
if command -v jq >/dev/null 2>&1; then
  PROMPT=$(printf '%s' "$INPUT" | jq -r '.prompt // empty')
else
  PROMPT=$(printf '%s' "$INPUT" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(d.get('prompt', ''), end='')
" 2>/dev/null || true)
fi

# Empty prompt → no-op
if [ -z "$PROMPT" ]; then
  exit 0
fi

# Token estimate: chars / 4 heuristic (same as tldt's TokenizeSentences, D-10)
CHAR_COUNT=$(printf '%s' "$PROMPT" | wc -c | tr -d ' ')
TOKEN_ESTIMATE=$(( CHAR_COUNT / 4 ))

# Get threshold from tldt config (reads ~/.tldt.toml [hook] threshold, D-10)
THRESHOLD=$(tldt --print-threshold 2>/dev/null || echo "2000")

# Below threshold → pass through silently
if [ "$TOKEN_ESTIMATE" -lt "$THRESHOLD" ]; then
  exit 0
fi

# Summarize — capture stderr (savings line) and stdout (summary)
STATS_FILE=$(mktemp)
SUMMARY=$(printf '%s' "$PROMPT" | tldt --verbose 2>"$STATS_FILE" || true)
SAVINGS=$(cat "$STATS_FILE")
rm -f "$STATS_FILE"

# If summarization failed or returned empty, pass through
if [ -z "$SUMMARY" ]; then
  exit 0
fi

# Build replacement context: savings line first, then summary (D-04, D-06)
REPLACEMENT="${SAVINGS}

${SUMMARY}"

# Output hookSpecificOutput JSON for Claude Code to inject as additionalContext
printf '%s' "$(printf '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":%s}}' \
  "$(printf '%s' "$REPLACEMENT" | python3 -c "import json,sys; print(json.dumps(sys.stdin.read()))")")"
```

**Known issue:** The `.prompt` field may contain raw control characters (unescaped in the JSON stream) — see GitHub issue #53463. Use python3 json parser rather than jq as primary if jq version < 1.6, or add error handling.

[CITED: https://github.com/anthropics/claude-code/issues/53463 — unescaped control chars in prompt field]

### Pattern 5: go:embed for Skills and Hooks

**What:** Embed skill and hook template files into the tldt binary at compile time.
**When to use:** Required for `--install-skill` to work from any PATH location (D-14).

```go
// File: cmd/tldt/embed.go
package main

import "embed"

//go:embed ../../skills
//go:embed ../../hooks
var embeddedFiles embed.FS
```

**Alternative — use a dedicated embed package at repo root:**

```go
// File: internal/installer/embed.go  (simpler path resolution)
package installer

import "embed"

//go:embed skills hooks
var EmbeddedFiles embed.FS
```

**Key go:embed rules verified:**
- `//go:embed dir` embeds all files recursively in the directory (files starting with `.` or `_` are excluded)
- The directive must immediately precede the `var` declaration
- Variable must be `embed.FS`, `string`, or `[]byte`
- Works since Go 1.16; this project uses Go 1.26.2

[CITED: https://pkg.go.dev/embed — official embed package docs]

**Recommended approach:** Place `embed.go` in a new `internal/installer/` package. The installer package owns both the embed and the install logic. This avoids putting embed directives in `main` with complex relative paths.

**Reading embedded files:**
```go
data, err := EmbeddedFiles.ReadFile("skills/tldt/SKILL.md")
// deploy to: filepath.Join(homeDir, ".claude", "skills", "tldt", "SKILL.md")
```

### Pattern 6: settings.json JSON Patching

**What:** Read, merge, and write `~/.claude/settings.json` without losing existing content.

```go
func PatchSettingsJSON(settingsPath string, hookCmd string) error {
    // Read existing or start fresh
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
                "command": hookCmd,
                "timeout": 30,
            },
        },
    }
    
    existing, _ := hooks["UserPromptSubmit"].([]interface{})
    // Idempotency: check if this hook command is already registered
    for _, e := range existing {
        if m, ok := e.(map[string]interface{}); ok {
            for _, h := range m["hooks"].([]interface{}) {
                if hm, ok := h.(map[string]interface{}); ok {
                    if hm["command"] == hookCmd {
                        return nil // already installed
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
    return os.WriteFile(settingsPath, out, 0644)
}
```

**Critical:** Use `json.MarshalIndent` to preserve human-readable formatting. The existing `settings.json` is hand-edited by users.

### Anti-Patterns to Avoid

- **Overwriting settings.json:** Never `os.WriteFile(path, newContent)` without reading first — destroys user config.
- **Using bash heredoc in hook for JSON:** Fragile with special chars in summary. Use python3 for JSON encoding.
- **Skipping jq fallback:** jq is not universally installed; python3 is more portable as fallback.
- **Embedding files from cmd/ with relative paths:** Use `internal/installer/` to keep paths clean.
- **Writing stats to stdout in `--print-threshold`:** Must print ONLY the integer to stdout (same stdout-clean rule as summaries).
- **`--verbose` flag absent in hook script:** tldt only prints token stats to stderr when `--verbose` is set (from main.go line 166); hook MUST pass `--verbose` to capture stats, or read from `--format json`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON parsing in bash | Custom string manipulation | `jq` + python3 fallback | Control chars in `.prompt` will break naive parsing (confirmed bug #53463) |
| Token estimation in hook | Complex tokenizer | `wc -c / 4` heuristic (D-10) | Same heuristic as tldt's own TokenizeSentences — consistent and simple |
| Skill format detection | Per-app format variants | Single SKILL.md (Claude Code format) | Claude Code, Cursor, and OpenCode all use identical SKILL.md format |
| Binary file embedding | Runtime file reads from disk | `go:embed` | Guarantees install works from any PATH location (D-14) |

**Key insight:** The SKILL.md format is identical across Claude Code, Cursor, and OpenCode — write once, install to multiple dirs. No format transformation is needed.

---

## Multi-App Install Targets

**Verified installed apps on this machine:** Claude Code (primary), Cursor, OpenCode.
**Skills directories confirmed:**
- `~/.claude/skills/` — Claude Code global (VERIFIED: 80+ GSD skills present)
- `~/.cursor/skills/` — Cursor global (VERIFIED: GSD skills synced here)
- `~/.config/opencode/skills/` — OpenCode global (VERIFIED: sysadmin skill present)
- `~/.agents/skills/` — Cross-agent/Codex compatible (VERIFIED: present on this machine)

**Standard SKILL.md format:** All four directories use the same SKILL.md frontmatter schema. Write once, copy to all.

**Hook installation:** Only Claude Code supports `~/.claude/settings.json` UserPromptSubmit hooks. Cursor hooks and OpenCode hooks are a different system — Phase 6 only installs the hook to Claude Code settings.json. Cursor and OpenCode get the SKILL.md only.

**Detection logic for `--install-skill`:**

```
Always install:    ~/.claude/skills/tldt/SKILL.md   (Claude Code — primary)
                   ~/.claude/hooks/tldt-hook.sh      (Claude Code hook)
                   ~/.claude/settings.json           (Claude Code — add UserPromptSubmit entry)

Detect and install (if dir exists):
                   ~/.cursor/skills/tldt/SKILL.md    (Cursor — if ~/.cursor/ exists)
                   ~/.config/opencode/skills/tldt/SKILL.md  (OpenCode — if ~/.config/opencode/ exists)
                   ~/.agents/skills/tldt/SKILL.md    (Codex — if ~/.agents/ exists)

--skill-dir <path>:  <path>/tldt/SKILL.md            (explicit override, no hook installation)
```

**GSD reference:** The GSD `sync-skills` workflow (verified at `~/.claude/get-shit-done/workflows/sync-skills.md`) lists 14 supported runtimes: `claude codex copilot cursor windsurf opencode gemini kilo augment trae qwen codebuddy cline antigravity`. Phase 6 covers the four actually installed on this machine; others can be added in v3+.

[VERIFIED: `ls ~/.cursor/skills/`, `ls ~/.config/opencode/skills/`, `ls ~/.agents/skills/` — run directly]

---

## Common Pitfalls

### Pitfall 1: tldt Stats Only Print When `--verbose` Is Set

**What goes wrong:** Hook script pipes text to `tldt` and expects stats on stderr, but gets nothing because `--verbose` is required (main.go line 166).
**Why it happens:** The `--verbose` flag was added in Phase 3 to suppress stats when stdout is piped. The hook uses a pipe.
**How to avoid:** Hook script MUST call `tldt --verbose` to get the `~X -> ~Y tokens (Z% reduction)` line on stderr.
**Warning signs:** Summary appears but savings line is empty or missing.

[VERIFIED: main.go lines 158-169 — `if *verbose && effectiveFormat != "json"`]

### Pitfall 2: JSON Output from Hook Must Be Valid JSON

**What goes wrong:** Summary text contains double quotes, backslashes, or newlines that break the JSON output from the hook script.
**Why it happens:** Naive bash string interpolation into JSON: `{"additionalContext":"$SUMMARY"}` breaks on special chars.
**How to avoid:** Always use python3 to JSON-encode the summary: `python3 -c "import json,sys; print(json.dumps(sys.stdin.read()))"`.
**Warning signs:** Hook outputs malformed JSON; Claude Code ignores the additionalContext.

### Pitfall 3: go:embed Path Must Be Relative to the Package File

**What goes wrong:** `//go:embed ../../skills` fails if the file is in `cmd/tldt/embed.go` because go:embed only accepts paths relative to the package directory, not upward path traversal.
**Why it happens:** go:embed does not support `..` path components.
**How to avoid:** Place `embed.go` in `internal/installer/` — then `skills/` and `hooks/` directories can be at the repo root if the embed directive uses module-relative paths correctly. Actually: go:embed paths must be below the package directory OR use `//go:embed` from the repo root level package. **Best solution:** Create `skills/` and `hooks/` dirs at repo root; create an `embed.go` file at repo root in a new package `package installer` inside `internal/installer/`, but that package is at `internal/installer/` — the embed dirs must be at or below `internal/installer/`. **Simplest correct solution:** put `skills/` and `hooks/` inside `internal/installer/skills/` and `internal/installer/hooks/` and embed with `//go:embed skills hooks`.
**Warning signs:** Compile error: `pattern ../../skills: invalid pattern syntax`.

**Correct pattern — embed dirs as siblings of embed.go:**
```
internal/installer/
├── embed.go          // //go:embed skills hooks
├── installer.go
├── skills/
│   └── tldt/
│       └── SKILL.md
└── hooks/
    └── tldt-hook.sh
```

Alternatively, keep `skills/` and `hooks/` at repo root and use a thin embed shim at repo root level — but the package must be in a Go file at the same directory level. The `internal/installer/` approach with subdirectories is cleanest.

[CITED: https://pkg.go.dev/embed — "A //go:embed directive names one or more files/directories to embed, specified as patterns relative to the directory containing the source file"]

### Pitfall 4: settings.json MarshalIndent Loses Schema URL

**What goes wrong:** Reading and re-marshaling settings.json drops the `$schema` key or reorders keys, confusing tools that validate against the schema.
**Why it happens:** Go's `map[string]interface{}` does not preserve key order; `json.MarshalIndent` outputs alphabetical order.
**How to avoid:** Use `json.RawMessage` to preserve the full existing content and only append to the `hooks.UserPromptSubmit` array. Or accept key reordering — Claude Code does not validate key order.
**Warning signs:** Settings file changes diff shows all keys reordered even when only hook was added.

### Pitfall 5: `wc -c` Counts Bytes, Not Characters

**What goes wrong:** `wc -c` counts bytes; multi-byte UTF-8 characters (e.g., em dashes, CJK) inflate the count vs. true character count.
**Why it happens:** bash `wc -c` is byte count, not char count.
**How to avoid:** This is acceptable — the heuristic is already approximate (chars/4). Byte count slightly over-estimates tokens for Unicode text, which means the hook is conservative (fires slightly earlier than necessary). This is the correct failure mode.
**Warning signs:** None — acceptable approximation per D-10.

### Pitfall 6: UserPromptSubmit Hook Bug in Subdirectories

**What goes wrong:** UserPromptSubmit hooks may not fire when Claude Code is launched from a subdirectory (reported in issue #8810 for Claude Code 2.0.5 on Linux/WSL2).
**Why it happens:** Possible path resolution bug in hook discovery when cwd is not the home directory.
**How to avoid:** Use absolute paths in settings.json hook command (expand `$HOME` at install time to the actual path, e.g., `/Users/gleicon/.claude/hooks/tldt-hook.sh`). The installer MUST write the expanded absolute path, not a `$HOME/...` string.
**Warning signs:** Hook fires from `~` but not from `~/code/project/`.

[CITED: https://github.com/anthropics/claude-code/issues/8810]

---

## Code Examples

### Installing skill file from embedded FS

```go
// Source: https://pkg.go.dev/embed
func installSkillFile(fs embed.FS, srcPath, destPath string) error {
    data, err := fs.ReadFile(srcPath)
    if err != nil {
        return fmt.Errorf("embedded file %q not found: %w", srcPath, err)
    }
    if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
        return err
    }
    return os.WriteFile(destPath, data, 0644)
}
```

### Hook script reading .prompt from stdin JSON (verified bash)

```bash
# Portable JSON extraction — jq primary, python3 fallback
INPUT=$(cat)
if command -v jq >/dev/null 2>&1; then
  PROMPT=$(printf '%s' "$INPUT" | jq -r '.prompt // empty' 2>/dev/null)
else
  PROMPT=$(printf '%s' "$INPUT" | python3 -c \
    "import json,sys; d=json.load(sys.stdin); print(d.get('prompt',''), end='')" 2>/dev/null || true)
fi
```

### --print-threshold implementation in main.go

```go
// Add before flag.Parse() section:
printThreshold := flag.Bool("print-threshold", false, "print configured hook token threshold to stdout and exit")
installSkill := flag.Bool("install-skill", false, "install Claude Code skill file and hook")

// After config load, add early-exit dispatches:
if *printThreshold {
    fmt.Println(cfg.Hook.Threshold)
    os.Exit(0)
}
if *installSkill {
    err := installer.Install(skillDir, target)
    // ...
    os.Exit(0)
}
```

### Config struct extension

```go
// internal/config/config.go — add Hook sub-struct
type HookConfig struct {
    Threshold int `toml:"threshold"`
}

type Config struct {
    Algorithm string     `toml:"algorithm"`
    Sentences int        `toml:"sentences"`
    Format    string     `toml:"format"`
    Level     string     `toml:"level"`
    Hook      HookConfig `toml:"hook"`
}

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

### TOML config example for [hook] section

```toml
# ~/.tldt.toml
algorithm = "lexrank"
sentences = 5

[hook]
threshold = 2000
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Copy skill files manually | `--install-skill` CLI command | Phase 6 (new) | Users run one command to install across all detected apps |
| Hard-coded skill dirs | `go:embed` + runtime detection | Phase 6 (new) | Works from any PATH location, not just repo clone |
| No hook | UserPromptSubmit auto-trigger | Phase 6 (new) | Transparent compression before AI context overflow |

**Deprecated/outdated:**
- Nothing deprecated in this phase — all patterns are additive.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | UserPromptSubmit hook injects `additionalContext` as the text Claude sees, effectively replacing the original prompt | Architecture Patterns — Pattern 3 | If additionalContext is appended (not replacing), original huge prompt still enters context; need to use `decision: block` + additionalContext combo |
| A2 | `tldt --verbose` prints stats line to stderr when stdout is a pipe | Pitfall 1 | If verbose flag behavior changed, hook gets empty savings line; need to test |
| A3 | `jq-1.5` handles the `.prompt` field correctly without crashing on control chars | Pattern 4 — hook script | If `.prompt` has control chars and jq 1.5 crashes, fallback to python3 kicks in — acceptable |

**Notes on A1:** D-06 says "hook replaces prompt with summary + savings line — original text discarded." The mechanism for replacement via UserPromptSubmit hooks is `additionalContext` injection. The exact behavior (whether original is truly discarded vs. appended) should be confirmed via a quick test during Wave 0. If additionalContext merely appends, the hook should return `{"decision": "block"}` on stderr in addition to the additionalContext output, but the Claude Code docs suggest `additionalContext` replaces for UserPromptSubmit. Treat as LOW confidence until tested.

---

## Open Questions

1. **Does UserPromptSubmit additionalContext replace or append to the original prompt?**
   - What we know: Claude Code docs say "stdout is added as context" for UserPromptSubmit; the word "added" suggests append, not replace.
   - What's unclear: Whether `additionalContext` in `hookSpecificOutput` appends or replaces; whether `decision: block` is needed to suppress the original.
   - Recommendation: Wave 0 plan should include a trivial test hook that prints a fixed string and verify whether the original prompt appears alongside it or is suppressed.

2. **Does `--verbose` work correctly when tldt stdout is a pipe (non-TTY)?**
   - What we know: `main.go` line 166: `if *verbose && effectiveFormat != "json"` — the flag is checked, not TTY detection.
   - What's unclear: Whether any TTY check elsewhere suppresses verbose output when piping.
   - Recommendation: Add a unit test in Wave 0: `echo "text" | tldt --verbose 2>&1` — confirm stats line appears.

3. **Should the hook file be at `~/.claude/hooks/tldt-hook.sh` or in a user-controlled location?**
   - What we know: All GSD hooks live in `~/.claude/hooks/` (verified on this machine).
   - What's unclear: Whether `~/.claude/hooks/` is a Claude Code-defined convention or just GSD's preference.
   - Recommendation: Follow GSD convention — install to `~/.claude/hooks/tldt-hook.sh`. Document in README.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.16+ (embed support) | `go:embed` directive | ✓ | go1.26.2 | — |
| `jq` | Hook script JSON parsing | ✓ | jq-1.5 | python3 fallback in hook |
| `python3` | Hook script JSON encoding | ✓ | Available | None needed (jq primary) |
| `~/.claude/` directory | Claude Code skill + hook install | ✓ | — | Create on install |
| `~/.cursor/` directory | Cursor skill install | ✓ | — | Skip if absent |
| `~/.config/opencode/` directory | OpenCode skill install | ✓ | — | Skip if absent |
| `~/.agents/` directory | Codex-compatible install | ✓ | — | Skip if absent |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** `jq` — python3 fallback in hook script.

---

## Validation Architecture

> `nyquist_validation` is explicitly `false` in `.planning/config.json` — this section is skipped.

---

## Security Domain

> Phase 6 has no authentication, session management, or cryptography concerns. Input validation is limited to: hook reads JSON from Claude Code (trusted local process); installer writes to user home directory (user owns it).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes (minimal) | Quote all bash variables; use python3 for JSON encoding |
| V6 Cryptography | no | — |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Hook reads .prompt from Claude Code; malformed JSON could crash hook | Tampering | python3 json.load() with error handling; hook exits 0 on any parse error |
| settings.json patch could corrupt file | Tampering | Read-parse-merge-write pattern; write to temp file then rename |
| Malformed SKILL.md could confuse Claude | Tampering | Template is embedded and static; not user-editable at install time |

---

## Sources

### Primary (HIGH confidence)
- Live `~/.claude/skills/*/SKILL.md` — 80+ files inspected directly; frontmatter schema confirmed
- Live `~/.claude/settings.json` — hooks array format confirmed from PreToolUse/PostToolUse entries
- `/Users/gleicon/code/go/src/github.com/gleicon/tldt/cmd/tldt/main.go` — `--verbose` flag and stats logic confirmed at lines 158-169
- `/Users/gleicon/code/go/src/github.com/gleicon/tldt/internal/config/config.go` — Config struct and TOML extension pattern
- `https://pkg.go.dev/embed` — go:embed directive rules and path constraints
- `~/.claude/get-shit-done/workflows/sync-skills.md` — multi-runtime names and detection pattern

### Secondary (MEDIUM confidence)
- `https://code.claude.com/docs/en/hooks` — UserPromptSubmit hook type confirmed; input/output schema
- `https://agentskill.sh/how-to-install-a-skill` — Claude Code, Cursor, Codex install paths verified against local filesystem
- `https://opencode.ai/docs/skills/` — OpenCode SKILL.md format and `~/.config/opencode/skills/` path
- `https://cursor.com/help/customization/skills` — Cursor `~/.cursor/skills/` path confirmed

### Tertiary (LOW confidence)
- `https://gist.github.com/FrancisBourre/50dca37124ecc43eaf08328cdcccdb34` — hook input/output schema detail
- Multiple web search results corroborating `.prompt` field name in UserPromptSubmit stdin JSON

---

## Metadata

**Confidence breakdown:**
- SKILL.md format: HIGH — verified from 80+ live examples
- settings.json hook format: HIGH — verified from live file; UserPromptSubmit structure confirmed from docs
- go:embed pattern: HIGH — official stdlib docs
- Multi-app install paths: HIGH — verified by running `ls` on all four directories
- UserPromptSubmit stdin JSON schema: MEDIUM — confirmed from multiple sources; `.prompt` field name consistent
- additionalContext replacement behavior: LOW — docs say "added as context"; exact behavior unconfirmed (see A1)

**Research date:** 2026-05-02
**Valid until:** 2026-06-01 (Claude Code hook format stable; check if UserPromptSubmit behavior changes in Claude Code updates)
