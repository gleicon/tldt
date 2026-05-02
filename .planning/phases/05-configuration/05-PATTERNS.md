# Phase 5: Configuration - Pattern Map

**Mapped:** 2026-05-02
**Files analyzed:** 4 (2 new, 1 modified, 1 go.mod update)
**Analogs found:** 4 / 4

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/config/config.go` | utility/config | file-I/O | `internal/fetcher/fetcher.go` | role-match (isolated I/O package, no-error public API) |
| `internal/config/config_test.go` | test | file-I/O | `internal/fetcher/fetcher_test.go` | exact (same package test style, temp files) |
| `cmd/tldt/main.go` | entrypoint | request-response | `cmd/tldt/main.go` (self) | exact (modify in place) |
| `go.mod` / `go.sum` | config | — | `go.mod` (self) | exact (add one `require` entry) |

---

## Pattern Assignments

### `internal/config/config.go` (utility, file-I/O)

**Analog:** `internal/fetcher/fetcher.go`

**Package declaration pattern** (`internal/fetcher/fetcher.go` line 1-4):
```go
// Package fetcher fetches a URL and extracts the main article text content.
//
// TODO: ...
package fetcher
```
Copy the doc-comment-above-package-decl style:
```go
// Package config loads per-user defaults from ~/.tldt.toml and exposes
// named level presets. All errors are absorbed; Load always returns a
// usable Config.
package config
```

**Imports pattern** (`internal/fetcher/fetcher.go` lines 8-18 as reference; config equivalent):
```go
import (
    "os"
    "path/filepath"

    "github.com/BurntSushi/toml"
)
```
Conventions observed: stdlib block first, blank line, then third-party. Single import block, no dot imports, no aliasing unless collision.

**Struct + tags pattern** (`internal/formatter/formatter.go` lines 9-20 as reference for struct-with-tags):
```go
type SummaryMeta struct {
    Algorithm          string
    SentencesIn        int
    ...
}
```
Config equivalent uses `toml` struct tags (analogous to `json` tags in formatter):
```go
type Config struct {
    Algorithm string `toml:"algorithm"`
    Sentences int    `toml:"sentences"`
    Format    string `toml:"format"`
    Level     string `toml:"level"`
}
```

**Default/constructor pattern** (no direct analog — use research pattern):
```go
func DefaultConfig() Config {
    return Config{
        Algorithm: "lexrank",
        Sentences: 5,
        Format:    "text",
        Level:     "",
    }
}
```

**Level presets map** (no analog — new concept):
```go
var LevelPresets = map[string]int{
    "lite":       3,
    "standard":   5,
    "aggressive": 10,
}
```

**Core I/O pattern** (`internal/fetcher/fetcher.go` lines 27-84 as structure reference):
Fetcher absorbs low-level errors and wraps them in `fmt.Errorf`. Config's `Load` goes further: it must NEVER return an error (CFG-03). Model the silent-fallback pattern:
```go
func Load(cfgPath string) Config {
    cfg := DefaultConfig()
    _, err := toml.DecodeFile(cfgPath, &cfg)
    if err != nil {
        // file absent (os.IsNotExist) OR malformed TOML — both are silent (CFG-03).
        // Return a fresh DefaultConfig(), NOT the partially-filled cfg, because
        // BurntSushi/toml is a streaming decoder that may have partially mutated cfg
        // before hitting the parse error.
        return DefaultConfig()
    }
    return cfg
}
```

**Path helper pattern** (`internal/fetcher/fetcher.go` lines 29-35 URL validation as structural reference):
```go
func ConfigPath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".tldt.toml"), nil
}
```
`ConfigPath` is the only function in the package that returns an error — it propagates `os.UserHomeDir` failure. All other functions absorb errors.

---

### `internal/config/config_test.go` (test, file-I/O)

**Analog:** `internal/fetcher/fetcher_test.go`

**Package declaration** (`fetcher_test.go` line 1):
```go
package fetcher
```
Config tests use the same white-box (same-package) style:
```go
package config
```

**Imports pattern** (`fetcher_test.go` lines 3-11):
```go
import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)
```
Config test equivalent (no net; uses os and temp files):
```go
import (
    "os"
    "testing"
)
```

**Temp file pattern** (`cmd/tldt/main_test.go` lines 73-85 `writeTempFile`):
```go
func writeTempFile(t *testing.T, content string) string {
    t.Helper()
    f, err := os.CreateTemp("", "tldt-ref-*.txt")
    if err != nil {
        t.Fatalf("cannot create temp file: %v", err)
    }
    if _, err := f.WriteString(content); err != nil {
        t.Fatalf("cannot write temp file: %v", err)
    }
    f.Close()
    t.Cleanup(func() { os.Remove(f.Name()) })
    return f.Name()
}
```
Config tests use the same `os.CreateTemp` + `t.Cleanup(os.Remove)` pattern inline (no shared helper needed since the package is small):
```go
func TestLoad_MalformedTOML(t *testing.T) {
    f, err := os.CreateTemp("", "bad-*.toml")
    if err != nil {
        t.Fatal(err)
    }
    f.WriteString("algorithm = bad toml [[[")
    f.Close()
    t.Cleanup(func() { os.Remove(f.Name()) })

    cfg := Load(f.Name())
    want := DefaultConfig()
    if cfg != want {
        t.Errorf("malformed TOML: got %+v, want %+v", cfg, want)
    }
}
```

**Error-path test pattern** (`fetcher_test.go` lines 42-55 `TestFetch_404`):
```go
func TestFetch_404(t *testing.T) {
    ...
    _, err := Fetch(ts.URL, testTimeout, testMaxBytes)
    if err == nil {
        t.Error("Fetch: expected error for 404 response, got nil")
    }
    if !strings.Contains(err.Error(), "404") {
        t.Errorf(...)
    }
}
```
Config equivalent tests the no-error contract — `Load` never errors, so tests assert the returned struct equals `DefaultConfig()`:
```go
func TestLoad_MissingFile(t *testing.T) {
    cfg := Load("/nonexistent/path/.tldt.toml")
    want := DefaultConfig()
    if cfg != want {
        t.Errorf("missing file: got %+v, want %+v", cfg, want)
    }
}
```

**Happy-path test pattern** (`fetcher_test.go` lines 15-40 `TestFetch_OK`):
```go
func TestFetch_OK(t *testing.T) {
    ts := httptest.NewServer(...)
    defer ts.Close()
    text, err := Fetch(ts.URL, ...)
    if err != nil {
        t.Fatalf("Fetch: unexpected error: %v", err)
    }
    if strings.TrimSpace(text) == "" {
        t.Error(...)
    }
}
```
Config equivalent:
```go
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

---

### `cmd/tldt/main.go` (entrypoint, request-response — modify in place)

**Analog:** `cmd/tldt/main.go` (self — extend the existing structure)

**Existing flag block** (lines 20-36 — add `--level` here):
```go
filePath := flag.String("f", "", "input file path")
urlFlag  := flag.String("url", "", "URL of a webpage to fetch and summarize")
algorithm := flag.String("algorithm", "lexrank", "algorithm: lexrank|textrank|graph|ensemble")
sentences := flag.Int("sentences", 5, "number of output sentences")
// ... existing flags ...
flag.Parse()
```
Add `--level` immediately after `sentences`:
```go
level := flag.String("level", "", "named preset: lite (3)|standard (5)|aggressive (10)")
```
Update `flag.Usage` to mention `--level`.

**Config integration insertion point** (after `flag.Parse()` on line 36, before `resolveInputBytes` on line 38):
```go
// Load config file — silent fallback to defaults on any error (CFG-03).
cfgPath, _ := config.ConfigPath()
cfg := config.Load(cfgPath)

// Detect which flags the user explicitly provided (CFG-02).
// flag.Visit (NOT flag.VisitAll) visits only explicitly-set flags.
flagsSet := make(map[string]bool)
flag.Visit(func(f *flag.Flag) { flagsSet[f.Name] = true })

// Resolve effective parameters: config -> level preset -> explicit flags.
effectiveAlgorithm := cfg.Algorithm
effectiveSentences  := cfg.Sentences
effectiveFormat      := cfg.Format
effectiveLevel       := cfg.Level

// --level flag overrides config level (CFG-04).
if flagsSet["level"] {
    effectiveLevel = *level
}
// Validate --level value if set.
if effectiveLevel != "" {
    if n, ok := config.LevelPresets[effectiveLevel]; ok {
        effectiveSentences = n
    } else {
        fmt.Fprintf(os.Stderr, "unknown --level %q: valid values are lite, standard, aggressive\n", effectiveLevel)
        os.Exit(1)
    }
}
// Explicit --sentences always wins over level preset (CFG-05).
if flagsSet["sentences"] { effectiveSentences = *sentences }
if flagsSet["algorithm"] { effectiveAlgorithm = *algorithm }
if flagsSet["format"]    { effectiveFormat    = *format    }
```

**Import addition** (lines 3-17 — add config import):
```go
import (
    "bytes"
    "flag"
    "fmt"
    "io"
    "os"
    "strconv"
    "strings"
    "time"
    "unicode/utf8"

    "github.com/gleicon/tldt/internal/config"   // <-- add this line
    "github.com/gleicon/tldt/internal/fetcher"
    "github.com/gleicon/tldt/internal/formatter"
    "github.com/gleicon/tldt/internal/summarizer"
)
```

**Downstream variable usage** — after the override block, replace all `*algorithm`, `*sentences`, `*format` dereferences with `effectiveAlgorithm`, `effectiveSentences`, `effectiveFormat`. The existing call sites are:
- `summarizer.New(*algorithm)` → `summarizer.New(effectiveAlgorithm)` (line 57)
- `s.Summarize(text, *sentences)` → `s.Summarize(text, effectiveSentences)` (lines 69, 89, 91)
- `meta := formatter.SummaryMeta{Algorithm: *algorithm, ...}` → use effective vars (lines 124-133)
- `switch *format {` → `switch effectiveFormat {` (line 135)
- The `*verbose` and `*format != "json"` check on line 118 → `effectiveFormat != "json"`

**Error handling pattern** (existing — `fmt.Fprintln(os.Stderr, err); os.Exit(1)` pattern at lines 39-41, 44-46, 58-60):
```go
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```
Reuse this exact pattern for the `--level` validation error (use `fmt.Fprintf(os.Stderr, ...)` for formatted message, then `os.Exit(1)`).

---

### `cmd/tldt/main_test.go` (integration tests for config — extend in place)

**Analog:** `cmd/tldt/main_test.go` (self — add new test functions)

**`run()` helper** (lines 58-70 — reuse unchanged):
```go
func run(t *testing.T, stdin string, args ...string) (stdout, stderr string, ok bool) {
    t.Helper()
    cmd := exec.Command(binaryPath, args...)
    cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
    if stdin != "" {
        cmd.Stdin = strings.NewReader(stdin)
    }
    var outBuf, errBuf strings.Builder
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf
    err := cmd.Run()
    return outBuf.String(), errBuf.String(), err == nil
}
```

**HOME override pattern for config tests** (use `t.Setenv` before `run()`):
`cmd.Env = append(os.Environ(), ...)` in `run()` snapshots `os.Environ()` at call time. `t.Setenv("HOME", tmpHome)` modifies `os.Environ()` and is auto-restored after the test. Since tests are NOT parallel (no `t.Parallel()` calls anywhere), this is safe.
```go
func TestMain_ConfigFileDefaults(t *testing.T) {
    tmpHome := t.TempDir()
    tomlContent := "algorithm = \"ensemble\"\nsentences = 7\n"
    if err := os.WriteFile(filepath.Join(tmpHome, ".tldt.toml"), []byte(tomlContent), 0644); err != nil {
        t.Fatal(err)
    }
    t.Setenv("HOME", tmpHome) // auto-restored; propagates into run() via os.Environ()

    stdout, _, ok := run(t, shortText) // shortText = existing test constant
    if !ok {
        t.Fatal("config defaults: binary exited non-zero")
    }
    lines := strings.Split(strings.TrimSpace(stdout), "\n")
    if len(lines) != 7 {
        t.Errorf("config defaults: want 7 sentences, got %d", len(lines))
    }
}
```

**`writeTempFile` helper** (lines 73-85 — reuse for writing config TOML in tests where `t.TempDir()` is inconvenient):
```go
func writeTempFile(t *testing.T, content string) string { ... }
```

---

### `go.mod` / `go.sum` (dependency update)

**Analog:** `go.mod` (self)

**Existing `require` block** (lines 5-8):
```go
require (
    github.com/didasy/tldr v0.7.0
    github.com/go-shiori/go-readability v0.0.0-20251205110129-5db1dc9836f0
)
```
Add `github.com/BurntSushi/toml` to the direct `require` block via `go get`:
```bash
go get github.com/BurntSushi/toml@v1.6.0
```
This updates both `go.mod` and `go.sum` automatically. Do not edit `go.mod` by hand.

---

## Shared Patterns

### Error handling (non-fatal stderr + os.Exit(1))
**Source:** `cmd/tldt/main.go` lines 39-41, 44-46, 58-60
**Apply to:** All new error paths in modified `main.go` (config path failure, invalid --level)
```go
fmt.Fprintln(os.Stderr, err)
os.Exit(1)
```
For formatted messages (e.g., invalid --level):
```go
fmt.Fprintf(os.Stderr, "unknown --level %q: valid values are lite, standard, aggressive\n", effectiveLevel)
os.Exit(1)
```

### Temp file creation in tests
**Source:** `cmd/tldt/main_test.go` lines 73-85
**Apply to:** `internal/config/config_test.go` (all tests that need a TOML file on disk)
```go
f, err := os.CreateTemp("", "pattern-*.toml")
// ...write content...
f.Close()
t.Cleanup(func() { os.Remove(f.Name()) })
```

### Package-level doc comment
**Source:** `internal/fetcher/fetcher.go` lines 1-4
**Apply to:** `internal/config/config.go`
```go
// Package config loads per-user defaults from ~/.tldt.toml and exposes
// named level presets. All errors are absorbed; Load always returns a
// usable Config.
package config
```

### Same-package (white-box) test files
**Source:** `internal/fetcher/fetcher_test.go` line 1 (`package fetcher`)
**Apply to:** `internal/config/config_test.go`
```go
package config
```

### Struct tags for external encoding
**Source:** `internal/formatter/formatter.go` lines 23-34 (json tags on JSONOutput)
**Apply to:** `internal/config/config.go` Config struct (toml tags instead of json tags)

---

## No Analog Found

All four files have close analogs. No files require falling back to RESEARCH.md patterns exclusively — though the `flag.Visit` override block has no direct analog (it is new logic with no existing equivalent in the codebase) and must be copied verbatim from RESEARCH.md Pattern 2.

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| — | — | — | — |

---

## Anti-Patterns (from RESEARCH.md — enforced via patterns above)

| Anti-Pattern | Correct Pattern | Enforced By |
|---|---|---|
| `flag.VisitAll` to detect user-set flags | `flag.Visit` (only set flags) | Pattern Assignment for main.go |
| Modify `*sentencesFlag` directly with config value | Use separate `effectiveSentences` variable | Override block pattern above |
| `return cfg` after `toml.DecodeFile` error | `return DefaultConfig()` (fresh call) | Load() pattern above |
| Calling `flag.Visit` before `flag.Parse()` | `flag.Visit` after `flag.Parse()` | Insertion point in main.go pattern |
| Level resolution after `--sentences` override check | Level resolution before `--sentences` check | Override block ordering above |

---

## Metadata

**Analog search scope:** `internal/fetcher/`, `internal/formatter/`, `internal/summarizer/`, `cmd/tldt/`
**Files scanned:** 20 Go source files + go.mod
**Pattern extraction date:** 2026-05-02
