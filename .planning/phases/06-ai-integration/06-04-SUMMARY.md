---
phase: 06-ai-integration
plan: "04"
subsystem: cli
tags: [go, flag, installer, makefile, cli-flags]

# Dependency graph
requires:
  - phase: 06-03
    provides: internal/installer package with Install(Options) and embedded skill/hook files
  - phase: 06-01
    provides: internal/config Hook.Threshold field (default 2000)

provides:
  - "--print-threshold flag in cmd/tldt/main.go: prints cfg.Hook.Threshold to stdout and exits"
  - "--install-skill flag in cmd/tldt/main.go: calls installer.Install with --skill-dir and --target options"
  - "install-skill Makefile target: builds binary then runs ./tldt --install-skill"

affects: [06-ai-integration, users-of-tldt]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "early-exit dispatch pattern after flag.Visit block — new flags that bypass summarization pipeline"
    - "installer.Install(Options{...}) called from main() CLI entry point"

key-files:
  created: []
  modified:
    - cmd/tldt/main.go
    - Makefile

key-decisions:
  - "Early-exit dispatches placed after flag.Visit() but before effectiveAlgorithm block to mirror existing pattern"
  - "--print-threshold prints bare integer only (no label) so hook script can capture via command substitution"
  - "install-skill Makefile target depends on build so binary is always fresh; uses ./$(BINARY) not installed binary"
  - "--skill-dir maps to installer.Options.SkillDir; --target maps to installer.Options.Target"

patterns-established:
  - "CLI flags that bypass summarization exit before effectiveAlgorithm line — consistent early-exit pattern"
  - "Installer errors print to stderr with 'install-skill:' prefix then exit 1"

requirements-completed: [AI-01, AI-02, AI-03, AI-04]

# Metrics
duration: 15min
completed: 2026-05-02
---

# Phase 6 Plan 04: CLI Flag Wiring Summary

**--print-threshold and --install-skill flags wired into cmd/tldt/main.go with installer.Install dispatch and Makefile install-skill target**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-05-02T21:15:00Z
- **Completed:** 2026-05-02T21:30:35Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added four new flags to cmd/tldt/main.go: --print-threshold, --install-skill, --skill-dir, --target
- Wired --print-threshold to print cfg.Hook.Threshold (default 2000) to stdout and exit 0
- Wired --install-skill to call installer.Install(installer.Options{SkillDir, Target}) with proper error handling
- Added internal/installer import to cmd/tldt/main.go
- Added install-skill target to Makefile with build dependency
- All 233 existing tests pass — no regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Add --print-threshold and --install-skill flags to main.go** - `c435647` (feat)
2. **Task 2: Add install-skill target to Makefile** - `c1d360f` (feat)

## Files Created/Modified

- `cmd/tldt/main.go` - Added installer import, four new flags, early-exit dispatches after flag.Visit block
- `Makefile` - Added install-skill to .PHONY and added install-skill target with build dependency

## Decisions Made

- Early-exit dispatches placed after `flag.Visit()` (so cfg is loaded and flagsSet is populated) but before `effectiveAlgorithm` block — mirrors the existing pattern
- `--print-threshold` prints bare integer with no label so hook script can capture it directly via `$(tldt --print-threshold)`
- `install-skill` Makefile target uses `./$(BINARY)` (locally built binary, not installed) to avoid PATH confusion during development
- `--skill-dir` passed as `installer.Options.SkillDir`; `--target` passed as `installer.Options.Target` — no translation in main.go

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. Users can now run:
- `tldt --print-threshold` to see their configured hook threshold
- `tldt --install-skill` to install the Claude Code skill and hook
- `make install-skill` to build and install in one step

## Next Phase Readiness

Phase 6 AI Integration is now complete. All four requirements (AI-01 through AI-04) are satisfied:
1. `tldt --install-skill` writes SKILL.md and registers the UserPromptSubmit hook
2. `tldt --print-threshold` exposes threshold config for the hook script
3. `make install-skill` provides a one-step build + install target
4. The installer handles claude, cursor, opencode, and agents targets

---
*Phase: 06-ai-integration*
*Completed: 2026-05-02*

## Self-Check: PASSED

- cmd/tldt/main.go: FOUND (modified with new flags)
- Makefile: FOUND (modified with install-skill target)
- Commit c435647: FOUND (Task 1 - main.go flags)
- Commit c1d360f: FOUND (Task 2 - Makefile target)
- /tmp/tldt-test --print-threshold outputs "2000": VERIFIED
- /tmp/tldt-skill-test/tldt/SKILL.md created: VERIFIED
- go test ./... 233 tests pass: VERIFIED
