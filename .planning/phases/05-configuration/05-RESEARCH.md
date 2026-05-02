# Phase 5: Configuration - Research

**Researched:** 2026-05-02
**Domain:** Go TOML config file loading, flag override precedence, named compression presets
**Confidence:** HIGH

## Summary

Phase 5 adds two capabilities to tldt: (1) a persistent config file at `~/.tldt.toml` that sets default values for `algorithm`, `sentences`, `format`, and `level`; and (2) a `--level` flag with three named presets (lite=3, standard=5, aggressive=10 sentences).

The project uses Go's standard `flag` package (not cobra/pflag). The key mechanism for CLI-overrides-config (CFG-02) is `flag.Visit`, which iterates only over flags the user explicitly provided on the command line — allowing precise detection of whether `--sentences` was set. This was verified by experiment in this session. The TOML library `github.com/BurntSushi/toml` v1.6.0 is the clear standard choice: it is the most widely used Go TOML library, its API matches the `encoding/json` pattern already familiar from the codebase, and `os.IsNotExist` correctly identifies a missing config file from `toml.DecodeFile` — enabling the silent fallback required by CFG-03.

The config loading belongs in a new `internal/config` package (consistent with `internal/fetcher`, `internal/formatter` patterns). Integration tests for the config-aware binary use the `HOME` environment variable override to redirect config lookup to a temp directory — this keeps tests hermetic without touching the real `~/.tldt.toml`.

**Primary recommendation:** Add `github.com/BurntSushi/toml@v1.6.0`, create `internal/config`, implement `flag.Visit` for override detection, use `HOME` env override in integration tests.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Config file loading | internal/config pkg | cmd/tldt/main.go (caller) | Isolates I/O and defaults, testable independently |
| Level preset resolution | internal/config pkg | — | Preset-to-sentence mapping is config domain logic |
| Flag override detection | cmd/tldt/main.go | — | flag.Visit is only meaningful at CLI entry point |
| Default values | internal/config.DefaultConfig | — | Centralizes all built-in defaults |
| Config file path resolution | internal/config pkg | — | Uses os.UserHomeDir; isolated for testability |

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CFG-01 | User can create ~/.tldt.toml with default values for algorithm, sentences, format, and level flags | toml.DecodeFile into Config struct; struct fields match these four keys |
| CFG-02 | CLI flags always override values from ~/.tldt.toml | flag.Visit detects explicitly-set flags; override applied post-parse |
| CFG-03 | Missing or malformed ~/.tldt.toml is not an error — defaults apply silently | os.IsNotExist on DecodeFile error for missing; catch all errors and return defaults |
| CFG-04 | User can run `tldt --level lite` (3), `--level standard` (5), or `--level aggressive` (10) | levelPresets map; resolved before flag override check |
| CFG-05 | --level can be set as default in ~/.tldt.toml; explicit --sentences N overrides it | Config.Level field; flag.Visit checks "sentences" key to detect explicit override |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/BurntSushi/toml | v1.6.0 | Parse ~/.tldt.toml into Config struct | Most widely used Go TOML library; encoding/json-like API; TOML v1.1.0 compliant |

No other new dependencies. The `flag` package and `os` package are stdlib and already used in `cmd/tldt/main.go`.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| github.com/BurntSushi/toml | github.com/pelletier/go-toml/v2 (v2.3.1) | pelletier is more actively maintained but adds complexity (strict mode, etc.); BurntSushi is simpler and sufficient for a small config struct |
| github.com/BurntSushi/toml | encoding/json with .tldt.json | TOML is specified in the requirements; JSON would violate CFG-01 |

**Installation:**
```bash
go get github.com/BurntSushi/toml@v1.6.0
```

**Version verification:** `curl -s https://proxy.golang.org/github.com/!burnt!sushi/toml/@latest` returned `"Version":"v1.6.0","Time":"2025-12-18T12:15:22Z"` [VERIFIED: Go module proxy].

## Architecture Patterns

### System Architecture Diagram

```
tldt binary invocation
        |
        v
  flag.Parse()          <-- CLI flags parsed, stored as *T pointers
        |
        v
  loadConfig()          <-- internal/config.Load(cfgPath)
  [internal/config]         reads ~/.tldt.toml (or returns defaults)
        |
  toml.DecodeFile()
     /        \
  found        not found / malformed
    |               |
  Config{...}   DefaultConfig()   (CFG-03: silent fallback)
        |
        v
  resolveEffectiveParams()   <-- in main.go
    - start with config values
    - if Level set (config or flag), apply preset (CFG-04, CFG-05)
    - flag.Visit: for each explicitly-set flag, override config value (CFG-02)
        |
        v
  summarizer.New(algorithm).Summarize(text, sentences)
        |
        v
  formatter.Format*(result, meta)  ->  stdout
```

### Recommended Project Structure
```
internal/
├── config/
│   ├── config.go        # Config struct, Load(), DefaultConfig(), LevelPresets
│   └── config_test.go   # unit tests for Load, preset resolution, defaults
internal/fetcher/        # existing (unchanged)
internal/formatter/      # existing (unchanged)
internal/summarizer/     # existing (unchanged)
cmd/tldt/
├── main.go              # wire config + flag.Visit override logic
└── main_test.go         # integration tests (HOME env override for config)
```

### Pattern 1: Config Struct and LoadConfig

```go
// Source: verified against github.com/BurntSushi/toml v1.6.0 docs
package config

import (
    "os"
    "path/filepath"

    "github.com/BurntSushi/toml"
)

// Config holds the persisted defaults from ~/.tldt.toml.
// All fields are optional in the file; zero values mean "not set".
type Config struct {
    Algorithm string `toml:"algorithm"`
    Sentences int    `toml:"sentences"`
    Format    string `toml:"format"`
    Level     string `toml:"level"`
}

// DefaultConfig returns the built-in defaults that apply when no
// config file exists or a field is absent from the file.
func DefaultConfig() Config {
    return Config{
        Algorithm: "lexrank",
        Sentences: 5,
        Format:    "text",
        Level:     "",
    }
}

// LevelPresets maps --level names to sentence counts (CFG-04).
var LevelPresets = map[string]int{
    "lite":       3,
    "standard":   5,
    "aggressive": 10,
}

// Load reads cfgPath (typically ~/.tldt.toml) into a Config struct.
// If the file does not exist or is malformed, Load silently returns
// defaults (CFG-03). A non-nil error is never returned.
func Load(cfgPath string) Config {
    cfg := DefaultConfig()
    _, err := toml.DecodeFile(cfgPath, &cfg)
    if err != nil {
        // os.IsNotExist: file absent — expected (CFG-03)
        // other errors: malformed TOML — also silent fallback (CFG-03)
        return DefaultConfig()
    }
    return cfg
}

// ConfigPath returns the standard path for the user's config file.
func ConfigPath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".tldt.toml"), nil
}
```

### Pattern 2: Flag Override Detection with flag.Visit

```go
// Source: verified against Go stdlib flag package (go doc flag.Visit)
// In cmd/tldt/main.go after flag.Parse():

// Step 1: load config (defaults or ~/.tldt.toml values)
cfgPath, _ := config.ConfigPath()
cfg := config.Load(cfgPath)

// Step 2: collect which flags the user explicitly set
flagsSet := make(map[string]bool)
flag.Visit(func(f *flag.Flag) { flagsSet[f.Name] = true })

// Step 3: resolve effective parameters
effectiveAlgorithm := cfg.Algorithm
effectiveSentences  := cfg.Sentences
effectiveFormat      := cfg.Format
effectiveLevel       := cfg.Level

// --level flag overrides config level (both go through same preset resolution)
if flagsSet["level"] {
    effectiveLevel = *levelFlag
}

// Resolve level -> sentences (CFG-04, CFG-05)
if effectiveLevel != "" {
    if n, ok := config.LevelPresets[effectiveLevel]; ok {
        effectiveSentences = n
    }
}

// Explicit --sentences always wins (CFG-02, CFG-05)
if flagsSet["sentences"] {
    effectiveSentences = *sentencesFlag
}
// Same pattern for algorithm, format
if flagsSet["algorithm"] { effectiveAlgorithm = *algorithmFlag }
if flagsSet["format"]    { effectiveFormat    = *formatFlag    }
```

### Pattern 3: Integration Test with HOME Override

```go
// Source: verified by running the HOME override pattern in this session
// In cmd/tldt/main_test.go:

func TestMain_ConfigFileDefaults(t *testing.T) {
    // Create temp dir to act as HOME
    tmpHome := t.TempDir()
    cfgContent := "algorithm = \"ensemble\"\nsentences = 7\n"
    if err := os.WriteFile(filepath.Join(tmpHome, ".tldt.toml"), []byte(cfgContent), 0644); err != nil {
        t.Fatal(err)
    }

    // Override HOME so the binary uses the temp config
    oldHome := os.Getenv("HOME")
    t.Setenv("HOME", tmpHome) // t.Setenv restores automatically
    _ = oldHome

    stdout, _, ok := run(t, shortText)  // no --sentences flag
    if !ok {
        t.Fatal("config defaults: binary exited non-zero")
    }
    // Verify 7 sentences were produced (ensemble algorithm, 7 sentences from config)
    lines := strings.Split(strings.TrimSpace(stdout), "\n")
    if len(lines) != 7 {
        t.Errorf("config defaults: want 7 sentences, got %d", len(lines))
    }
}
```

Note: The existing `run()` helper passes `cmd.Env = append(os.Environ(), ...)`. To inject `HOME`, either use `t.Setenv("HOME", tmpHome)` before calling `run()` (which modifies `os.Environ()` that `run()` reads), or extend `run()` to accept extra env vars. The `t.Setenv` approach is cleaner since it auto-restores.

### Pattern 4: Unit Tests for internal/config

Unit tests for `internal/config` do NOT need the binary — test `Load()` directly:

```go
// Source: standard Go testing pattern
func TestLoad_MissingFile(t *testing.T) {
    cfg := Load("/nonexistent/path/.tldt.toml")
    want := DefaultConfig()
    if cfg != want {
        t.Errorf("missing file: got %+v, want %+v", cfg, want)
    }
}

func TestLoad_MalformedTOML(t *testing.T) {
    f, _ := os.CreateTemp("", "bad-*.toml")
    f.WriteString("algorithm = bad toml [[[")
    f.Close()
    t.Cleanup(func() { os.Remove(f.Name()) })
    cfg := Load(f.Name())
    want := DefaultConfig()
    if cfg != want {
        t.Errorf("malformed TOML: got %+v, want %+v", cfg, want)
    }
}

func TestLoad_ValidConfig(t *testing.T) {
    f, _ := os.CreateTemp("", "valid-*.toml")
    f.WriteString("algorithm = \"textrank\"\nsentences = 7\n")
    f.Close()
    t.Cleanup(func() { os.Remove(f.Name()) })
    cfg := Load(f.Name())
    if cfg.Algorithm != "textrank" || cfg.Sentences != 7 {
        t.Errorf("valid config: got %+v", cfg)
    }
}
```

### Anti-Patterns to Avoid

- **Reading config inside `resolveInputBytes`:** Config loading is separate from input resolution. Do not mix them.
- **Returning error from `Load()`:** CFG-03 requires silent fallback. `Load()` must never return an error — absorb all errors and return defaults.
- **Using `flag.VisitAll` instead of `flag.Visit`:** `VisitAll` visits ALL flags including unset ones. Only `flag.Visit` visits explicitly-set flags.
- **Deriving sentences from level after the `flag.Visit` check:** Level resolution must happen before the `--sentences` override check, not after. Otherwise `--sentences` cannot override a `--level` setting.
- **Calling `flag.Visit` before `flag.Parse()`:** `flag.Visit` only knows about set flags after `flag.Parse()` has run.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TOML parsing | Custom parser, regex key=value reader | github.com/BurntSushi/toml | TOML has multiline strings, arrays, datetime; hand-rolling misses edge cases |
| Config file location | Custom path construction | `os.UserHomeDir()` + `filepath.Join` | Handles $HOME, Windows %USERPROFILE%, Plan 9 $home correctly |

**Key insight:** The TOML spec has enough edge cases (inline tables, datetime, escape sequences) that hand-rolling is always wrong. Even for simple `key = "value"` configs, use the library — it handles unknown keys, type mismatches, and encoding gracefully.

## Common Pitfalls

### Pitfall 1: flag.VisitAll vs flag.Visit Confusion
**What goes wrong:** Using `flag.VisitAll` to detect user-set flags — it visits ALL flags including those using defaults, so every flag appears "set".
**Why it happens:** Both functions have similar names.
**How to avoid:** Always use `flag.Visit` (no "All"). Verified: `flag.Visit` only visits flags the user provided on the command line.
**Warning signs:** Config values are always overridden by flag defaults.

### Pitfall 2: Loading Config After flag.Parse but Overriding Before Visit
**What goes wrong:** Applying config values over flag defaults before `flag.Visit` determines which flags were set. If you do `*sentencesFlag = cfg.Sentences` before Visit, Visit can no longer distinguish user-set from config-set.
**Why it happens:** Temptation to modify flag values directly.
**How to avoid:** Never modify the raw flag pointers. Use separate `effectiveX` variables. Load config → build effective params → override with Visit results.

### Pitfall 3: DecodeFile Partial Success on Malformed TOML
**What goes wrong:** When TOML is malformed, `toml.DecodeFile` may have partially filled the struct before hitting the error. If you use the partially-filled struct, you get undefined config values.
**Why it happens:** BurntSushi/toml is a streaming decoder — it may apply some keys before detecting a syntax error.
**How to avoid:** Start with `cfg := DefaultConfig()` before `DecodeFile`. On error, return `DefaultConfig()` (a fresh call), not the partially-filled `cfg`.
**Verified:** Confirmed by experiment — always `return DefaultConfig()` on any error path.

### Pitfall 4: Test Pollution of Real ~/.tldt.toml
**What goes wrong:** Integration tests that run the binary pick up the developer's actual `~/.tldt.toml`, making tests environment-dependent.
**Why it happens:** `run()` inherits `os.Environ()` which includes the real `$HOME`.
**How to avoid:** In tests that exercise config loading, call `t.Setenv("HOME", t.TempDir())` before `run()`. `t.Setenv` is automatically restored after the test. Confirmed this pattern works with `os.UserHomeDir()` on macOS.

### Pitfall 5: Invalid --level Values
**What goes wrong:** User passes `--level maximum` — not in the preset map. Silent fallback to built-in default (5 sentences) is confusing.
**How to avoid:** Validate `--level` value against `LevelPresets` keys after `flag.Parse()`; print error to stderr and exit non-zero if unrecognized. The three valid values are: `lite`, `standard`, `aggressive`.

### Pitfall 6: Sentences Zero-Value Ambiguity
**What goes wrong:** TOML struct field `Sentences int` has zero value 0. If user writes `sentences = 0` in config, it is indistinguishable from "not set" — `Load()` would return the built-in default 5 (because 0 is falsy in the current design).
**How to avoid:** Document that `sentences = 0` in the config file is treated as "not set" (falls back to built-in default 5). This is acceptable per the requirements. Alternative: use `*int` pointer field — but adds complexity. Keep `int` with documented behavior.

## Code Examples

### Minimal Valid ~/.tldt.toml
```toml
# ~/.tldt.toml — tldt configuration
algorithm = "ensemble"
sentences = 7
format    = "text"
level     = "standard"
```

### Level Preset Table (Canonical)
```
lite       -> 3 sentences
standard   -> 5 sentences
aggressive -> 10 sentences
```

### Flag Override Precedence (Resolved Order)
```
1. Built-in defaults (algorithm=lexrank, sentences=5, format=text)
2. ~/.tldt.toml values (any key present overrides built-in default)
3. --level preset (overrides sentences from config or default)
4. --sentences N (always wins if explicitly provided; overrides level-derived count)
5. Other flags (--algorithm, --format) if explicitly provided override config
```

## Runtime State Inventory

Not applicable — this is a new feature addition, not a rename/refactor/migration phase.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Building package | Yes | go1.26.2 darwin/arm64 | — |
| github.com/BurntSushi/toml | Config parsing | Not yet in go.mod | v1.6.0 (latest as of 2025-12-18) | — (no fallback; must be added) |
| os.UserHomeDir | Config path | Yes (stdlib) | Go 1.26 | — |
| flag.Visit | Override detection | Yes (stdlib) | Go 1.26 | — |

**Missing dependencies with no fallback:**
- `github.com/BurntSushi/toml` — must be added via `go get`. Not a blocker; it's a public module with no authentication required.

**Missing dependencies with fallback:**
- None.

## Validation Architecture

`workflow.nyquist_validation` is explicitly `false` in `.planning/config.json` — this section is skipped.

## Security Domain

This phase introduces no new network calls, no user authentication, and no cryptographic operations. The only new I/O is reading `~/.tldt.toml` from the local filesystem.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | Yes (limited) | Validate --level against known preset keys; reject unknown values with non-zero exit |
| V1 Architecture | Yes (minimal) | Config file is read-only at startup; no writes to filesystem |

**TOML file path traversal:** `os.UserHomeDir()` + `filepath.Join` produces an absolute path. The user controls their own `~/.tldt.toml`. No traversal risk for a single-user CLI tool.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| INI / custom key=value | TOML | 2013+ | TOML is now the dominant Go config format for CLI tools |
| cobra/viper for config | Direct flag + BurntSushi/toml | — | This project uses stdlib `flag`; no need to pull cobra just for config |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `t.Setenv("HOME", ...)` propagates into `run()` subprocess via `os.Environ()` | Pattern 3 | Tests would not be hermetic; use explicit env injection in run() instead | 

Note on A1: The `run()` helper calls `cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)`. `t.Setenv` modifies `os.Environ()` for the duration of the test. Since subprocess env is snapshotted at `exec.Command` time, this should work — but if test parallelism were enabled, concurrent modification of `os.Environ()` would be unsafe. Current tests are NOT parallel (no `t.Parallel()` calls), so this is safe. [ASSUMED — not verified by inspecting Go subprocess env snapshotting, but consistent with standard Go testing practice]

## Open Questions

1. **Sentences=0 in config**
   - What we know: TOML `int` zero-value is 0, which is indistinguishable from "not set" in the current `Config` struct design.
   - What's unclear: Should `sentences = 0` be rejected as invalid, or silently treated as "not set"?
   - Recommendation: Treat `sentences = 0` (or any negative value) as "not set" — fall through to built-in default. Document this in --help output. This avoids pointer fields which add complexity.

2. **Unknown keys in ~/.tldt.toml**
   - What we know: `toml.DecodeFile` with a plain struct silently ignores unknown TOML keys (they are not errors).
   - What's unclear: Should unknown keys generate a warning to stderr?
   - Recommendation: Silently ignore (CFG-03 says missing/malformed is not an error; unknown keys are even less severe). No warning. Use `MetaData.Undecoded()` in a future `--check-config` command (already deferred per ROADMAP).

## Sources

### Primary (HIGH confidence)
- `github.com/BurntSushi/toml` — `go doc` output verified in session against v1.6.0; `DecodeFile`, `MetaData.Undecoded()`, struct tag behavior confirmed [VERIFIED: go doc in temp module]
- Go stdlib `flag` package — `go doc flag.Visit`, `go doc flag.Flag` confirmed behavior; `flag.Visit` visits only explicitly-set flags [VERIFIED: go doc + live experiment]
- Go module proxy — `curl https://proxy.golang.org/github.com/!burnt!sushi/toml/@latest` returned v1.6.0, published 2025-12-18 [VERIFIED: Go module proxy]
- `os.IsNotExist` on `toml.DecodeFile` error — confirmed returns true for missing file via live experiment [VERIFIED: live experiment in session]
- `flag.Visit` detects only user-set flags — confirmed via live experiment with `-sentences 7` [VERIFIED: live experiment in session]
- HOME env override propagation — confirmed via live experiment that `os.UserHomeDir()` uses `$HOME` env var [VERIFIED: live experiment in session]
- Partial-fill behavior on malformed TOML — confirmed that `return DefaultConfig()` (fresh call) is safer than using partially-filled struct [VERIFIED: live experiment in session]

### Secondary (MEDIUM confidence)
- `t.Setenv()` auto-restore behavior — documented in Go testing package; standard practice for env isolation in tests [CITED: https://pkg.go.dev/testing#T.Setenv]

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — BurntSushi/toml v1.6.0 verified via module proxy; `go doc` API confirmed
- Architecture: HIGH — flag.Visit and HOME override patterns verified by experiment
- Pitfalls: HIGH — all pitfalls verified by experiment or confirmed by API inspection
- Test patterns: HIGH — consistent with existing codebase patterns (os.MkdirTemp, writeTempFile)

**Research date:** 2026-05-02
**Valid until:** 2026-06-02 (stable libraries; BurntSushi/toml is low-churn)
