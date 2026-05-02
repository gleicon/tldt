---
phase: 05-configuration
fixed_at: 2026-05-02T00:00:00Z
review_path: .planning/phases/05-configuration/05-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 05: Code Review Fix Report

**Fixed at:** 2026-05-02T00:00:00Z
**Source review:** .planning/phases/05-configuration/05-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-01: `sentences = 0` (or negative) in config file silently produces empty output

**Files modified:** `internal/config/config.go`, `internal/config/config_test.go`
**Commit:** 9cad5da
**Applied fix:** Added a post-decode guard in `Load()` that resets `cfg.Sentences` to `DefaultConfig().Sentences` (5) when the decoded value is <= 0. Added `TestLoad_ZeroSentences` to `config_test.go` to cover this edge case explicitly.

### WR-02: Invalid `format` value from config file is silently ignored, not validated

**Files modified:** `cmd/tldt/main.go`
**Commit:** 148438a
**Applied fix:** Added a `validFormats` map check after `effectiveFormat` is fully resolved (after both config and flag precedence is applied). Unknown format values now produce a clear error message to stderr and exit 1, covering both CLI flag and config file paths.

### WR-03: `LevelPresets` naming is semantically inverted relative to common convention

**Files modified:** `internal/config/config.go`, `internal/config/config_test.go`, `cmd/tldt/main.go`, `cmd/tldt/main_test.go`, `.planning/REQUIREMENTS.md`
**Commit:** 5384271
**Applied fix:** Reversed the sentence counts so that "aggressive" means most compression (3 sentences) and "lite" means least compression (10 sentences), standard stays at 5. Updated the `LevelPresets` map comment, flag usage string, `flag.String` description, `TestLevelPresets` expected values, `TestMain_LevelLite` (3→10), `TestMain_LevelAggressive` (10→3), `TestMain_LevelOverriddenBySentences` comment, `TestMain_ConfigLevelDefault` (3→10), and REQUIREMENTS.md CFG-04 description.

### WR-04: `flag.Usage` exits with code `1` instead of the POSIX-conventional `2`

**Files modified:** `cmd/tldt/main.go`
**Commit:** d55c610
**Applied fix:** Changed `os.Exit(1)` to `os.Exit(2)` in the custom `flag.Usage` handler, consistent with POSIX convention for usage/argument errors and with Go's `flag` package own behavior on unknown flags.

---

_Fixed: 2026-05-02T00:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
