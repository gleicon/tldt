# Phase 6: AI Integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-02
**Phase:** 06-AI Integration
**Areas discussed:** Skill file format, Hook trigger arch, Threshold config, Install UX

---

## Skill File Format

| Option | Description | Selected |
|--------|-------------|----------|
| stdin pipe | `echo "$text" \| tldt` — existing pipe-safe design | ✓ |
| Positional arg | `tldt "$text"` — breaks on large inputs (shell arg limit) | |
| Temp file | Write to temp, `tldt -f /tmp/tldt-input` — adds complexity | |

**User's choice:** stdin pipe

| Option | Description | Selected |
|--------|-------------|----------|
| /tldt slash command | User selects text, runs /tldt — familiar, inline summary | ✓ |
| Keybinding only | Hotkey, no slash command — less discoverable | |
| Both slash + keybinding | More files to maintain | |

**User's choice:** /tldt slash command

| Option | Description | Selected |
|--------|-------------|----------|
| No flags — use config defaults | tldt reads ~/.tldt.toml; skill stays minimal | ✓ |
| Hardcode --format markdown | Always markdown in conversation | |
| Expose key flags in skill | /tldt --sentences 3 — adds parsing complexity | |

**User's choice:** No flags — use config defaults

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — capture stderr, show it | Redirect stderr, include savings line before summary | ✓ |
| No — summary only | Clean output, no metadata | |

**User's choice:** Yes — capture stderr and show token savings line

---

## Hook Trigger Arch

| Option | Description | Selected |
|--------|-------------|----------|
| Bash | Zero dependencies, runs anywhere Go is installed | ✓ |
| Python | More readable but adds dependency | |
| Go (compiled) | Second binary (tldt-hook) — separate build artifact | |

**User's choice:** Bash

| Option | Description | Selected |
|--------|-------------|----------|
| Replace prompt with summary + savings line | Original text discarded | ✓ |
| Prepend summary, keep original | Both go to Claude — saves nothing | |
| Summarize and ask Claude to confirm | Adds friction | |

**User's choice:** Replace prompt with summary + savings line

| Option | Description | Selected |
|--------|-------------|----------|
| stdin | Claude Code pipes prompt to hook via stdin | ✓ |
| Environment variable | $CLAUDE_PROMPT — breaks on long prompts | |
| Temp file path in env | More complex but handles any size | |

**User's choice:** stdin

| Option | Description | Selected |
|--------|-------------|----------|
| Pass through silently — no-op | Exit 0, prompt unchanged — session never breaks | ✓ |
| Warn to stderr, pass through | Install hint printed to stderr | |
| Hard fail | Breaks Claude Code prompt flow | |

**User's choice:** Pass through silently (no-op) when tldt not in PATH

---

## Threshold Config

| Option | Description | Selected |
|--------|-------------|----------|
| ~/.tldt.toml [hook] section | threshold = 2000 under [hook] table | ✓ |
| Env var TLDT_THRESHOLD | Simple but config scattered across two places | |
| Hardcoded in hook script | User edits script directly | |

**User's choice:** ~/.tldt.toml [hook] section

| Option | Description | Selected |
|--------|-------------|----------|
| tldt --print-threshold flag | Hook calls: THRESH=$(tldt --print-threshold) | ✓ |
| Grep the TOML directly | Brittle if TOML structure changes | |
| TLDT_THRESHOLD env overrides TOML | No TOML parsing in bash | |

**User's choice:** tldt --print-threshold flag

| Option | Description | Selected |
|--------|-------------|----------|
| 2000 tokens | ~8KB text, matches ROADMAP.md success criteria | ✓ |
| 1000 tokens | More aggressive, could surprise users | |
| 4000 tokens | Conservative, less intrusive | |

**User's choice:** 2000 tokens default

---

## Install UX

| Option | Description | Selected |
|--------|-------------|----------|
| make install-skill | Makefile target copies files | |
| tldt --install-skill flag | Self-contained binary writes files; wired to make install-skill | ✓ |
| README manual steps | Zero code changes, error-prone | |

**User's choice:** `tldt --install-skill` (self-contained binary), wired as `make install-skill`
**Notes:** User wants the binary to do the install, with Makefile as the convenience wrapper.

| Option | Description | Selected |
|--------|-------------|----------|
| skills/ and hooks/ at repo root | Discoverable, separate from Go source | ✓ |
| cmd/tldt/embed/ | Embedded in binary via go:embed | |
| contrib/ | Less discoverable | |

**User's choice:** skills/ and hooks/ at repo root (for inspection); embedded in binary via go:embed for distribution

| Option | Description | Selected |
|--------|-------------|----------|
| go:embed in binary | Install works from any PATH location | ✓ |
| Read from repo at install time | Breaks with go install | |

**User's choice:** go:embed

| Option | Description | Selected |
|--------|-------------|----------|
| ~/.claude/skills/ + ~/.claude/settings.json | Default Claude Code paths | ✓ |
| --skill-dir configurable | Accept custom target dir | ✓ |
| Also research other coding apps | OpenCode, Cursor, Copilot, Zed, etc. | ✓ |

**User's choice:** Options 1 + 3 combined: default to ~/.claude/*, support --skill-dir, AND research + support other coding app targets (OpenCode etc). Reference how GSD handles multi-target skill installation.

---

## Claude's Discretion

- Hook bash token counting implementation (wc -c / 4)
- Exact SKILL.md YAML frontmatter schema (researcher determines from Claude Code docs)
- JSON structure for hook registration in ~/.claude/settings.json
- Multi-app target detection logic

## Deferred Ideas

- MCP server mode — already deferred to v3+ (pre-existing)
- Clipboard auto-read — already deferred to v3+ (pre-existing)
- URL authentication headers — already deferred to v3+
- TOML validation/lint command — already deferred to v3+
