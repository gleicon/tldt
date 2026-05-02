---
phase: 06-ai-integration
plan: 02
subsystem: installer
tags: [bash, claude-code, skill, hook, go-embed]

requires:
  - phase: 05-config
    provides: "~/.tldt.toml config system with [hook] threshold support (tldt --print-threshold)"
provides:
  - "internal/installer/skills/tldt/SKILL.md — Claude Code /tldt skill template"
  - "internal/installer/hooks/tldt-hook.sh — UserPromptSubmit hook bash script"
affects:
  - 06-03
  - "go:embed directive in Plan 03 embed.go"

tech-stack:
  added: []
  patterns:
    - "SKILL.md: YAML frontmatter with allowed-tools: [Bash] + prose body instructing Claude to run Bash tool"
    - "Hook bash: set -euo pipefail; jq-primary/python3-fallback JSON parsing; python3 json.dumps for output encoding"
    - "Hook bash: tldt --print-threshold for threshold read; tmpfile pattern for stderr capture"

key-files:
  created:
    - internal/installer/skills/tldt/SKILL.md
    - internal/installer/hooks/tldt-hook.sh
  modified: []

key-decisions:
  - "SKILL.md uses allowed-tools: [Bash] because skill instructs Claude to invoke the Bash tool, not run bash directly"
  - "Hook uses jq as primary JSON parser with python3 fallback to handle unescaped control chars in .prompt field (Pitfall 3)"
  - "Hook uses python3 json.dumps for JSON output encoding — bash string interpolation breaks on special chars (Pitfall 2)"
  - "Hook uses --verbose flag on tldt call — stats only printed to stderr when --verbose is set (Pitfall 1)"
  - "Hook exits 0 silently when tldt absent from PATH so user session never breaks (D-08)"
  - "Both files placed under internal/installer/ so go:embed in Plan 03 can reference them as sibling directories"

patterns-established:
  - "Pattern 1: SKILL.md format — YAML frontmatter (name, description, argument-hint, allowed-tools) + prose body with bash code block"
  - "Pattern 2: Hook bash stderr capture — mktemp + redirect 2>file + cat + rm -f pattern avoids mixing stderr into summary"
  - "Pattern 3: Hook JSON output — python3 json.dumps always, never bash string interpolation"

requirements-completed: [AI-01, AI-02, AI-03, AI-04]

duration: 12min
completed: 2026-05-02
---

# Phase 06 Plan 02: AI Integration — Installer Templates Summary

**SKILL.md and tldt-hook.sh installer templates created: Claude Code /tldt slash command skill and UserPromptSubmit auto-summarize hook with jq/python3 fallback JSON parsing and threshold-gated summarization**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-05-02T21:09:00Z
- **Completed:** 2026-05-02T21:21:11Z
- **Tasks:** 2 of 2
- **Files created:** 2

## Accomplishments

- Created `internal/installer/skills/tldt/SKILL.md` — a valid Claude Code skill with YAML frontmatter, `allowed-tools: [Bash]`, and body that instructs Claude to run `echo "$ARGUMENTS" | tldt --verbose 2>&1`
- Created `internal/installer/hooks/tldt-hook.sh` — a bash hook script (mode 0755) that fires on UserPromptSubmit, checks token count against configured threshold, summarizes via tldt, and outputs hookSpecificOutput JSON
- Both files placed under `internal/installer/` so Plan 03's `go:embed` directive can reference them at compile time

## Task Commits

Each task was committed atomically:

1. **Task 1: Create SKILL.md template inside internal/installer/skills/tldt/** - `80b023d` (feat)
2. **Task 2: Create tldt-hook.sh template inside internal/installer/hooks/** - `d8777c0` (feat)

## Files Created/Modified

- `internal/installer/skills/tldt/SKILL.md` — Claude Code skill template; YAML frontmatter (name, description, argument-hint, allowed-tools: [Bash]) + prose instructing Claude to run tldt via Bash tool with --verbose and 2>&1
- `internal/installer/hooks/tldt-hook.sh` — UserPromptSubmit hook bash script; graceful no-op if tldt absent; threshold read via `tldt --print-threshold`; jq primary + python3 fallback JSON parsing; stderr capture via tmpfile; python3 json.dumps output encoding

## Decisions Made

- Both files live under `internal/installer/` (not repo root) so Plan 03's `go:embed` directive can reference them — this is the go:embed sibling constraint (Pitfall 3 from RESEARCH.md).
- Hook uses `|| true` guards on jq and tldt calls so `set -euo pipefail` never breaks the user's Claude Code session.
- Hook's threshold fallback is `2000` when `tldt --print-threshold` fails, matching D-11.

## Deviations from Plan

None — plan executed exactly as written. Both files match the exact content specified in the plan and PATTERNS.md.

## Issues Encountered

None.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced beyond what the plan's threat model documents. T-06-03 through T-06-06 mitigations are implemented as specified:
- T-06-03: jq `// empty` + python3 fallback handles control chars in .prompt
- T-06-04: python3 json.dumps used for all JSON output
- T-06-05: `|| true` guards + empty SUMMARY check ensure hook never blocks session
- T-06-06: mktemp + immediate rm -f; no persistence

## Known Stubs

None — these are template files, not wired Go code. They will be embedded and written to disk by the installer code in Plan 03.

## Next Phase Readiness

- Plan 03 can now compile `go:embed` directives that reference `internal/installer/skills/*` and `internal/installer/hooks/*`
- The `--install-skill` installer (Plan 03) will write these templates to `~/.claude/skills/tldt/SKILL.md` and `~/.claude/hooks/tldt-hook.sh`
- No blockers for Plan 03 or Plan 04

## Self-Check

Verifying created files exist and commits are present:

- `internal/installer/skills/tldt/SKILL.md`: FOUND
- `internal/installer/hooks/tldt-hook.sh`: FOUND (executable)
- Commit `80b023d`: FOUND (feat(06-02): SKILL.md)
- Commit `d8777c0`: FOUND (feat(06-02): tldt-hook.sh)

## Self-Check: PASSED

---
*Phase: 06-ai-integration*
*Completed: 2026-05-02*
