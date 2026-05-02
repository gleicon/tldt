---
phase: 05-configuration
reviewed: 2026-05-02T00:00:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - internal/config/config.go
  - internal/config/config_test.go
  - cmd/tldt/main.go
  - cmd/tldt/main_test.go
findings:
  critical: 0
  warning: 4
  info: 3
  total: 7
status: issues_found
---

# Phase 05: Code Review Report

**Reviewed:** 2026-05-02T00:00:00Z
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

This phase adds TOML config file loading (`~/.tldt.toml`) and named level presets (`lite`, `standard`, `aggressive`) to the CLI. The config package is small and clean. The main wiring in `cmd/tldt/main.go` correctly uses `flag.Visit` to detect explicitly-set flags and applies a documented precedence order (config → level preset → explicit flags). Test coverage for the config package is thorough.

Four warnings were found: one is a correctness gap (no validation of `sentences` values from the config file), one is a silent-degradation issue (unvalidated `format` from config), one is a naming-semantic mismatch in `LevelPresets`, and one is a non-standard exit code in `flag.Usage`. Three info items cover minor quality gaps.

## Warnings

### WR-01: `sentences = 0` (or negative) in config file silently produces empty output

**File:** `internal/config/config.go:41-47`, `cmd/tldt/main.go:51`

**Issue:** `Load()` stores whatever integer the TOML file contains directly into `cfg.Sentences`. The Go TOML decoder writes the literal value `0` (or any negative integer) because it is a valid TOML integer distinct from "key absent". `DefaultConfig()` is called first and sets `Sentences = 5`, but `toml.DecodeFile` then overwrites it with the file value. There is no validation in `Load()` or in `main.go` before `effectiveSentences` reaches `Summarize()`. When `n = 0`, `selectTopN` returns an empty slice, `strings.Join` produces `""`, and `fmt.Println` emits a bare newline — silent empty output with exit 0.

**Fix:**
```go
// In internal/config/config.go, after DecodeFile succeeds:
func Load(cfgPath string) Config {
    cfg := DefaultConfig()
    _, err := toml.DecodeFile(cfgPath, &cfg)
    if err != nil {
        return DefaultConfig()
    }
    // Guard: zero/negative sentences in config file falls back to default
    if cfg.Sentences <= 0 {
        cfg.Sentences = DefaultConfig().Sentences
    }
    return cfg
}
```

Or in `cmd/tldt/main.go` after resolving `effectiveSentences`:
```go
if effectiveSentences <= 0 {
    fmt.Fprintln(os.Stderr, "sentences must be >= 1")
    os.Exit(1)
}
```

---

### WR-02: Invalid `format` value from config file is silently ignored, not validated

**File:** `cmd/tldt/main.go:187`

**Issue:** The `switch effectiveFormat` block has a `default:` case that falls through to plain-text output for any unrecognised format string. If a user sets `format = "xml"` in `~/.tldt.toml`, the tool silently produces text output without any warning. This is consistent with how the CLI `--format` flag behaves for unknown values (same silent fallback), but config-file values are invisible to the user and harder to debug. The related `--level` flag validates its value and exits with an error; the `--format` flag has no equivalent guard.

**Fix:** Add a validation step that covers both the CLI flag path and the config file path. The cleanest approach is a shared validator:
```go
// After resolving effectiveFormat, before consuming it:
validFormats := map[string]bool{"text": true, "json": true, "markdown": true}
if !validFormats[effectiveFormat] {
    fmt.Fprintf(os.Stderr, "unknown --format %q: valid values are text, json, markdown\n", effectiveFormat)
    os.Exit(1)
}
```

---

### WR-03: `LevelPresets` naming is semantically inverted relative to common convention

**File:** `internal/config/config.go:32-36`

**Issue:** `"aggressive"` maps to `10` sentences — the *most* output — while `"lite"` maps to `3` sentences. In text compression tooling, "aggressive" compression universally means *more* compression, i.e., *fewer* output sentences. A user choosing `--level aggressive` will likely expect the most compressed output (fewest sentences), not the least compressed. CFG-04 in REQUIREMENTS.md explicitly defines the mapping as `lite=3, standard=5, aggressive=10`, so this is a spec-level semantic choice, but the choice is counterintuitive and likely to cause user confusion.

**Fix (requires spec alignment):** Rename the levels so that the most-compressing preset has the most-compressive name, e.g.:
```go
var LevelPresets = map[string]int{
    "brief":     3,   // formerly "lite"
    "standard":  5,
    "detailed":  10,  // formerly "aggressive"
}
```
Or reverse the sentence counts so `aggressive` means fewer:
```go
var LevelPresets = map[string]int{
    "lite":       10,
    "standard":   5,
    "aggressive": 3,
}
```
This requires a coordinated update to REQUIREMENTS.md, the `flag.Usage` string, and all tests.

---

### WR-04: `flag.Usage` exits with code `1` instead of the POSIX-conventional `2`

**File:** `cmd/tldt/main.go:36`

**Issue:** The custom `flag.Usage` function calls `os.Exit(1)` when usage is printed. POSIX convention (and Go's standard library `flag` package default behavior) uses exit code `2` for usage/argument errors. Shell scripts and CI pipelines often distinguish `exit 1` (runtime error) from `exit 2` (usage error). Additionally, when `flag.Parse()` fails due to an unknown flag, the `flag` package itself calls `os.Exit(2)` via `os.Exit(flag.ContinueOnError)` default — creating an inconsistency where `-unknownflag` exits 2 but the custom usage handler exits 1.

**Fix:**
```go
flag.Usage = func() {
    fmt.Fprintln(os.Stderr, "Usage: tldt ...")
    flag.PrintDefaults()
    os.Exit(2) // conventional exit code for usage errors
}
```

---

## Info

### IN-01: `ConfigPath()` error is silently discarded in `main.go`

**File:** `cmd/tldt/main.go:41`

**Issue:** `cfgPath, _ := config.ConfigPath()` discards the error from `os.UserHomeDir()`. When this fails (e.g., in a container with no home directory), `cfgPath` is `""`. `config.Load("")` then calls `toml.DecodeFile("", ...)` which fails and returns `DefaultConfig()`. The end result is correct (defaults are used), but the silent discard obscures a potentially unexpected environment condition. A debug-level log or `--verbose` note would help users diagnose "why is my config not loading".

**Fix:** No code change required for correctness since `Load("")` safely returns defaults. Consider adding a `--verbose` note:
```go
cfgPath, cfgErr := config.ConfigPath()
cfg := config.Load(cfgPath)
if cfgErr != nil && *verbose {
    fmt.Fprintf(os.Stderr, "note: could not resolve config path: %v; using defaults\n", cfgErr)
}
```

---

### IN-02: No test exercises `sentences = 0` in a TOML config file

**File:** `internal/config/config_test.go`

**Issue:** `TestLoad_PartialConfig` verifies that an *absent* `sentences` key retains the default of `5`. There is no test for `sentences = 0` or `sentences = -1` explicitly present in the file, which is the edge case that exposes WR-01. The existing test suite would pass even with the bug described in WR-01 active.

**Fix:** Add a test case:
```go
func TestLoad_ZeroSentences(t *testing.T) {
    f, _ := os.CreateTemp("", "tldt-test-zero-*.toml")
    f.WriteString("sentences = 0\n")
    f.Close()
    t.Cleanup(func() { os.Remove(f.Name()) })
    cfg := Load(f.Name())
    if cfg.Sentences <= 0 {
        t.Errorf("Load(sentences=0): Sentences = %d, want > 0", cfg.Sentences)
    }
}
```

---

### IN-03: `flag.PrintDefaults()` in `flag.Usage` is called after the manual usage string — output is redundant

**File:** `cmd/tldt/main.go:33-37`

**Issue:** The custom `flag.Usage` prints a full manual synopsis line on line 33, then calls `flag.PrintDefaults()` on line 35. This produces two different representations of the same flags: the hand-maintained synopsis (which can drift from actual flags) and the auto-generated defaults. If a new flag is added and the synopsis is not updated, users see inconsistent help output.

**Fix:** Either remove the hand-maintained synopsis and rely solely on `flag.PrintDefaults()`, or remove the `flag.PrintDefaults()` call and maintain the synopsis manually. The former is lower maintenance risk.

---

_Reviewed: 2026-05-02T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
