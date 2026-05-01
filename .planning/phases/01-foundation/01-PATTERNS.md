# Phase 1: Foundation - Pattern Map

**Mapped:** 2026-05-01
**Files analyzed:** 6 new/modified files
**Analogs found:** 1 partial / 6 (near-greenfield build; old src/ code provides negative patterns only)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `go.mod` | config | — | none (first module file) | none |
| `cmd/tldt/main.go` | utility/entry | request-response | `src/main.go` | negative-only (wrong pattern; shows what to avoid) |
| `internal/summarizer/graph.go` | service | transform | `src/summary.go` | partial (same concept, incompatible API) |
| `Makefile` (replacement) | config | — | `src/Makefile` | negative-only |
| `test-data/wikipedia_en.txt` | test fixture | — | `test-data/body.txt` | role-match (same role, different language) |
| `test-data/youtube_transcript.txt` | test fixture | — | `test-data/body.txt` | role-match |
| `test-data/longform_3000.txt` | test fixture | — | `test-data/body3.txt` | role-match |
| `test-data/edge_short.txt` | test fixture | — | none (no short-text fixture exists) | none |

## Pattern Assignments

### `go.mod` (config)

**Analog:** none — first module file in this repo.

**Core pattern** (from RESEARCH.md, verified against go.dev/ref/mod):
```
module github.com/gleicon/tldt

go 1.26.2

require github.com/didasy/tldr v0.7.0
```

**How to generate:**
```bash
cd /Users/gleicon/code/go/src/github.com/gleicon/tldt
go mod init github.com/gleicon/tldt
go get github.com/didasy/tldr@v0.7.0
go mod tidy
```

**Note:** `go.sum` is generated automatically by `go mod tidy`. Do not hand-write it.

---

### `cmd/tldt/main.go` (entry point, request-response)

**Closest analog:** `src/main.go` — provides negative reference only. The old file uses `flag` correctly but wires a Redis-backed HTTP server. Copy the `flag` skeleton; discard everything else.

**What to reuse from `src/main.go` (lines 6-16, 23-29):**
- `flag.String(...)` / `flag.Parse()` / `flag.Usage` pattern — structurally correct.
- `fmt.Fprintln(os.Stderr, err)` + `os.Exit(1)` error exit pattern.

**What NOT to reuse from `src/main.go`:**
- All Redis, HTTP server, signal/SIGHUP log rotation, config file loading — delete entirely.
- `ioutil.ReadAll` from `src/handlers.go` line 44 — deprecated since Go 1.16; use `io.ReadAll`.

**Imports pattern to use:**
```go
import (
    "flag"
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/gleicon/tldt/internal/summarizer"
)
```

**Flag parsing pattern** (modeled from `src/main.go` lines 23-28, adapted):
```go
func main() {
    filePath := flag.String("f", "", "input file path")
    flag.Usage = func() {
        fmt.Fprintln(os.Stderr, "Usage: tldt [-f file] [text...]")
        flag.PrintDefaults()
        os.Exit(1)
    }
    flag.Parse()
    // ...
}
```

**Input resolution pattern** (from RESEARCH.md Pattern 1, verified against pkg.go.dev/os):
```go
func resolveInput(args []string, filePath string) (string, error) {
    // 1. stdin pipe: non-TTY stdin
    stat, _ := os.Stdin.Stat()
    if (stat.Mode() & os.ModeCharDevice) == 0 {
        data, err := io.ReadAll(os.Stdin)
        return string(data), err
    }
    // 2. -f flag
    if filePath != "" {
        data, err := os.ReadFile(filePath)
        return string(data), err
    }
    // 3. positional argument
    if len(args) > 0 {
        return strings.Join(args, " "), nil
    }
    return "", fmt.Errorf("no input: provide text via stdin, -f file, or positional argument")
}
```

**Error handling pattern** (from `src/main.go` lines 33-35 and handlers.go line 34, adapted):
```go
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```

**Output pattern:**
```go
fmt.Println(strings.Join(sentences, " "))
```

---

### `internal/summarizer/graph.go` (service, transform)

**Closest analog:** `src/summary.go` (lines 1-8) — same concept (wrap a tldr library), incompatible API.

**What to reuse from `src/summary.go`:**
- The structural idea: one exported function `Summarize(...)` wraps the library's `New()` + `Summarize()` call.
- Keeping it as a thin wrapper with no state — do not add caching or shared `*Bag`.

**What NOT to reuse from `src/summary.go`:**
- Import `github.com/JesusIslam/tldr` — incompatible API; returns `string` not `[]string`. Replace with `github.com/didasy/tldr`.
- Function signature `Summarize(sentences int, body string) (string, error)` — argument order and return type differ from the new API.

**New package declaration and imports:**
```go
package summarizer

import "github.com/didasy/tldr"
```

**Core wrapper pattern** (from RESEARCH.md Pattern 2, verified against didasy/tldr v0.7.0 raw source):
```go
// Summarize returns up to n sentences from text using the graph/LexRank algorithm.
// Sentences are returned in original document order, not score order.
// Note: tldr.Bag is not thread-safe; do not share across goroutines.
func Summarize(text string, n int) ([]string, error) {
    bag := tldr.New()
    return bag.Summarize(text, n)
}
```

**Key API facts for `github.com/didasy/tldr` v0.7.0:**
- `tldr.New()` returns `*Bag` with defaults: pagerank algorithm, hamming weighing, damping=0.85, tolerance=0.0001.
- `(*Bag).Summarize(text string, num int) ([]string, error)` — returns `[]string` (not `string`).
- Returns fewer than `num` sentences when the document has fewer sentences than requested — not an error.
- `*Bag` is not thread-safe.

---

### `Makefile` (replacement)

**Closest analog:** root `Makefile` and `src/Makefile` — both delegate to the old GOPATH build. Replace entirely.

**What NOT to reuse:**
- `include src/Makefile.defs` (line 6 root Makefile) — GOPATH artifact.
- `make -C src` delegation — wrong build system.
- `NAME=resumator` — wrong binary name.

**New Makefile pattern:**
```makefile
.PHONY: build test install clean

build:
	go build ./cmd/tldt

test:
	go test ./...

install:
	go install ./cmd/tldt

clean:
	rm -f tldt
```

---

### `test-data/wikipedia_en.txt`, `test-data/youtube_transcript.txt`, `test-data/longform_3000.txt` (test fixtures)

**Closest analog:** `test-data/body.txt`, `test-data/body2.txt`, `test-data/body3.txt` — existing Portuguese fixtures.

**What to reuse:** file naming convention (plain `.txt`, no subdirectory), plain UTF-8 text, no metadata headers.

**What NOT to reuse:** Portuguese language content — all new fixtures must be English.

**Size requirements from RESEARCH.md TEST-07:**
- `wikipedia_en.txt`: English Wikipedia article (any mid-length article; "Extractive summarization" is a suitable candidate).
- `youtube_transcript.txt`: Raw YouTube transcript of any public tech talk — no timestamps, plain sentences.
- `longform_3000.txt`: Minimum 3000 words of English prose.
- `edge_short.txt`: Sub-5-sentence English text (e.g., 2-3 sentences) to exercise the silent-cap behavior of `didasy/tldr`.

**Pitfall to avoid (from RESEARCH.md Pitfall 3):** `bag.Summarize(text, 5)` on a 3-sentence input returns 3 sentences without error. Tests must check `len(result)` not assume `len(result) == n`.

---

## Shared Patterns

### Error handling
**Source:** `src/main.go` lines 33-35 and `src/handlers.go` lines 34-35 (adapted, not copied verbatim)
**Apply to:** `cmd/tldt/main.go` — all error exit points
```go
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```

### Stdin I/O
**Source:** stdlib `io` and `os` packages (pkg.go.dev/io#ReadAll, pkg.go.dev/os)
**Apply to:** `cmd/tldt/main.go` — `resolveInput()` function
**Critical:** Use `io.ReadAll` (Go 1.16+), NOT `ioutil.ReadAll` (deprecated). `src/handlers.go` line 44 uses the deprecated form — do not copy it.

### Flag parsing
**Source:** `src/main.go` lines 23-28 (structural skeleton reusable; flag names and wiring are not)
**Apply to:** `cmd/tldt/main.go` — `main()` function
```go
filePath := flag.String("f", "", "input file path")
flag.Parse()
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `go.mod` | config | — | First module file; no prior modules project in this repo |
| `test-data/edge_short.txt` | test fixture | — | No short-text (sub-5-sentence) fixture exists; must be authored |

---

## Negative Patterns (What to Avoid)

These patterns exist in `src/` and must NOT be carried forward:

| Anti-Pattern | Source Location | Correct Alternative |
|---|---|---|
| `import "github.com/JesusIslam/tldr"` | `src/summary.go` line 3 | `import "github.com/didasy/tldr"` |
| `ioutil.ReadAll(...)` | `src/handlers.go` line 44 | `io.ReadAll(...)` |
| `import "github.com/fiorix/go-redis/redis"` | `src/main.go` line 16 | Drop entirely — no Redis in Phase 1 |
| `import "github.com/BurntSushi/toml"` | `src/conf.go` line 10 | Drop entirely — no config file in Phase 1 |
| `import "github.com/gorilla/handlers"` | `src/http.go` (inferred) | Drop entirely — no HTTP server in Phase 1 |
| `go get -v` in GOPATH mode | `src/Makefile` | `go mod tidy` |
| `make -C src` delegation | root `Makefile` line 13 | `go build ./cmd/tldt` |
| Shared `*Bag` instance | — (not in src, but a risk) | Create new `tldr.New()` per call |

---

## Metadata

**Analog search scope:** `/Users/gleicon/code/go/src/github.com/gleicon/tldt/src/` (all .go files), root Makefile, src/Makefile
**Files scanned:** `src/conf.go`, `src/handlers.go`, `src/http.go`, `src/main.go`, `src/summary.go`, `src/utils.go`, root `Makefile`, `src/Makefile`
**Pattern extraction date:** 2026-05-01
**Assessment:** Near-greenfield build. The existing `src/` directory provides structural reference and negative patterns only. All meaningful implementation patterns come from stdlib documentation and the `didasy/tldr` v0.7.0 API (both verified in RESEARCH.md). The planner should use RESEARCH.md code examples as primary reference for `cmd/tldt/main.go` and `internal/summarizer/graph.go`.
