---
phase: 06-ai-integration
plan: "03"
subsystem: installer
tags:
  - go-embed
  - installer
  - claude-code
  - skill
  - hook
  - settings-json
dependency_graph:
  requires:
    - "06-01 (HookConfig in config.go)"
    - "06-02 (SKILL.md + tldt-hook.sh templates in internal/installer/)"
  provides:
    - "internal/installer package (Install, PatchSettingsJSON, Options)"
    - "embed.FS with skills/ and hooks/ compiled into binary"
  affects:
    - "cmd/tldt/main.go (Plan 04 wires --install-skill flag to installer.Install)"
tech_stack:
  added:
    - "go:embed directive (stdlib embed package)"
    - "encoding/json for read-merge-write settings.json patching"
    - "atomic rename pattern (WriteFile to .tmp + os.Rename) for settings.json"
  patterns:
    - "TDD: RED (failing tests) -> GREEN (implementation) -> verify"
    - "Idempotency check before append in PatchSettingsJSON"
    - "Optional app detection via os.Stat on known directories"
key_files:
  created:
    - internal/installer/embed.go
    - internal/installer/installer.go
    - internal/installer/installer_test.go
  modified: []
decisions:
  - "go:embed directive placed in dedicated embed.go at internal/installer/ — keeps paths simple (no .. required)"
  - "PatchSettingsJSON uses atomic temp-file-then-rename for crash safety (Pitfall 4 mitigation)"
  - "hookCmd passed as absolute path to PatchSettingsJSON — hook registered with expanded path, not $HOME/... (Pitfall 6 mitigation)"
  - "Optional apps (Cursor, OpenCode, Agents) detected by directory existence; silently skipped if absent"
  - "Only Claude Code gets hookDest/settingsPath — Cursor/OpenCode/Agents receive SKILL.md only"
  - "containsString helper retained in installer.go (unexported, package-internal use only)"
metrics:
  duration: "~20 minutes"
  completed: "2026-05-02T21:27:00Z"
  tasks_completed: 2
  files_created: 3
  tests_added: 9
  tests_total: 233
---

# Phase 6 Plan 03: Installer Package Summary

**One-liner:** Embedded go:embed installer package writing SKILL.md and hook to Claude Code (always) and detected apps, with idempotent atomic settings.json patching.

## What Was Built

The `internal/installer` package delivers the core engine behind `tldt --install-skill`:

- **`embed.go`** — single `//go:embed skills hooks` directive compiling `internal/installer/skills/` and `internal/installer/hooks/` into the binary as `EmbeddedFiles embed.FS`.
- **`installer.go`** — `Install(opts Options) error` orchestrates multi-app installation; `PatchSettingsJSON(path, cmd)` performs read-merge-write with idempotency; `resolveTargets()` always includes Claude Code and conditionally adds Cursor/OpenCode/Agents.
- **`installer_test.go`** — 9 tests covering skill write, deep MkdirAll, hook write + executable bit, settings.json create from empty, merge-preserving-existing-keys, idempotency (exactly once), always-includes-claude, SkillDir override, and optional-app detection.

## Task Execution

### Task 1: embed.go (feat commit 5ea4d15)

Created `internal/installer/embed.go` with `//go:embed skills hooks` immediately before `var EmbeddedFiles embed.FS`. Verified both Wave 1 sibling directories exist (`skills/tldt/SKILL.md` and `hooks/tldt-hook.sh`). `go build ./internal/installer/...` exits 0.

### Task 2: installer.go + installer_test.go (TDD)

**RED** (commit e053d36): `installer_test.go` written with 9 tests referencing undefined symbols. Build fails as expected — confirmed RED phase.

**GREEN** (commit 547d28b): `installer.go` implemented. All 9 tests pass. Full 233-test suite passes — no regressions.

## Deviations from Plan

None — plan executed exactly as written. Implementation matched the plan's code exactly.

## TDD Gate Compliance

- RED gate: `test(06-03)` commit e053d36 — build-fail confirms RED
- GREEN gate: `feat(06-03)` commit 547d28b — all tests pass confirms GREEN
- REFACTOR gate: not needed (implementation was clean on first pass)

## Security / Threat Mitigations Applied

| Threat ID | Mitigation | Status |
|-----------|------------|--------|
| T-06-07 | Read-merge-write + atomic rename in PatchSettingsJSON | Applied |
| T-06-08 | json.Unmarshal error returned to caller; malformed JSON causes Install() to return error | Applied |
| T-06-09 | installHookFile uses mode 0755 | Applied |
| T-06-10 | Idempotency check before append; second call returns nil without writing | Applied — verified by TestPatchSettingsJSON_Idempotent |

## Known Stubs

None — no placeholder values, hardcoded empty returns, or TODO markers in any created files.

## Threat Flags

None — no new network endpoints, auth paths, or file access patterns beyond those described in the plan's threat model. The installer writes only to user home directory paths under explicit install targets.

## Self-Check: PASSED

- `internal/installer/embed.go` — FOUND
- `internal/installer/installer.go` — FOUND
- `internal/installer/installer_test.go` — FOUND
- Commit 5ea4d15 (embed.go) — FOUND
- Commit e053d36 (tests RED) — FOUND
- Commit 547d28b (installer.go GREEN) — FOUND
- `go build ./internal/installer/...` — exits 0
- `go test ./internal/installer/...` — 9 passed
- `go test ./...` — 233 passed (no regressions)
