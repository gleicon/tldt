---
phase: 06-ai-integration
plan: "01"
subsystem: config
tags: [go, toml, config, hook, threshold, BurntSushi/toml]

# Dependency graph
requires:
  - phase: 05-configuration
    provides: Config struct, TOML loading, DefaultConfig(), Load() with guards
provides:
  - HookConfig struct with Threshold int field
  - Config.Hook field (toml:"hook" section)
  - DefaultConfig().Hook.Threshold = 2000
  - Load() threshold guard (zero/negative falls back to 2000)
affects: [06-02, 06-03, 06-04, main.go --print-threshold wiring]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "HookConfig sub-struct pattern for TOML sub-sections (extends Phase 5 Config pattern)"
    - "Guard pattern: zero/negative int fields fall back to DefaultConfig() value"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go

key-decisions:
  - "HookConfig placed before Config struct declaration for readability (declaration order mirrors usage)"
  - "Threshold guard uses same pattern as Sentences guard: cfg.Hook.Threshold <= 0 falls back to DefaultConfig().Hook.Threshold"
  - "Default threshold 2000 (D-11, D-12): matches ROADMAP.md success criteria and ~8KB text boundary"

patterns-established:
  - "Sub-struct TOML section: add struct above Config, embed as field with toml:\"section\" tag"
  - "Defensive guard: after TOML decode, reset any zero/negative int to DefaultConfig value"

requirements-completed:
  - AI-03
  - AI-04

# Metrics
duration: 2min
completed: "2026-05-02"
---

# Phase 06 Plan 01: Config HookConfig Extension Summary

**HookConfig struct with Threshold int added to Config via toml:"hook" section; DefaultConfig returns 2000; Load() guards zero/negative threshold back to 2000**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-05-02T21:19:22Z
- **Completed:** 2026-05-02T21:21:00Z
- **Tasks:** 1 (TDD: RED + GREEN commits)
- **Files modified:** 2

## Accomplishments

- Added `HookConfig` struct with `Threshold int` (`toml:"threshold"`) before `Config` struct in `config.go`
- Extended `Config` struct with `Hook HookConfig` field (`toml:"hook"`)
- Updated `DefaultConfig()` to include `Hook: HookConfig{Threshold: 2000}` (D-11, D-12)
- Added threshold guard in `Load()` after sentences guard — zero/negative threshold falls back to 2000 (T-06-02)
- Added `TestHookConfig` function in `config_test.go` covering all 4 behavior cases
- 224 total tests pass (222 pre-phase-6 + 2 new); `go build ./...` succeeds

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Failing TestHookConfig** - `03fa090` (test)
2. **Task 1 GREEN: HookConfig implementation** - `ac483ad` (feat)

_TDD task: test commit (RED) then implementation commit (GREEN)_

## Files Created/Modified

- `internal/config/config.go` — Added `HookConfig` struct, `Hook HookConfig` field in `Config`, default `Threshold: 2000` in `DefaultConfig()`, threshold guard in `Load()`
- `internal/config/config_test.go` — Added `TestHookConfig` covering: default=2000, load 1500 from TOML, guard zero→2000, guard negative→2000

## Decisions Made

- Placed `HookConfig` struct immediately before `Config` struct (plan specification; mirrors TOML nesting visually)
- Used identical guard pattern as `Sentences`: `if cfg.Hook.Threshold <= 0 { cfg.Hook.Threshold = DefaultConfig().Hook.Threshold }` — consistent with existing code style
- No refactor pass needed — implementation is minimal and clean

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes introduced. The TOML threat mitigations (T-06-01, T-06-02) are both satisfied:
- T-06-01: parse error returns `DefaultConfig()` (pre-existing)
- T-06-02: threshold guard ensures zero/negative values fall back to 2000 (implemented in this plan)

## Known Stubs

None — `HookConfig.Threshold` is a real config value with a meaningful default. It will be consumed by `--print-threshold` in plan 06-04 (main.go wiring).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `Config.Hook.Threshold` is available for `--print-threshold` wiring in `main.go` (plan 06-04)
- Plans 06-02 (skill/hook templates) and 06-03 (installer package) do not depend on this field but proceed in parallel (wave 1)
- `cfg.Hook.Threshold` accessed as `cfg.Hook.Threshold` after `config.Load()` — no additional changes needed in consumer code

## Self-Check

- [x] `internal/config/config.go` exists and contains `HookConfig`, `Threshold: 2000`, threshold guard
- [x] `internal/config/config_test.go` exists and contains `TestHookConfig`
- [x] Commit `03fa090` exists (RED)
- [x] Commit `ac483ad` exists (GREEN)
- [x] `go test ./internal/config/... -run TestHookConfig` exits 0
- [x] `go test ./...` exits 0 (224 tests)
- [x] `go build ./...` succeeds

## Self-Check: PASSED

---
*Phase: 06-ai-integration*
*Completed: 2026-05-02*
