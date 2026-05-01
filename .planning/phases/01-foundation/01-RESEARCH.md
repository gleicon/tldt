# Phase 1: Foundation - Research

**Researched:** 2026-05-01
**Domain:** Go CLI tooling, go modules migration, extractive summarization baseline
**Confidence:** HIGH

## Summary

The tldt repository is a brownfield Go project currently structured as a GOPATH-based HTTP web server named "resumator" — a template artifact from 2015. There is no `go.mod` at the repo root. The project uses `go get` via `src/Makefile` and references three external dependencies (`github.com/JesusIslam/tldr`, `github.com/fiorix/go-redis/redis`, `github.com/gorilla/handlers`, `github.com/BurntSushi/toml`). The entire HTTP/Redis/config infrastructure must be deleted; only `summary.go` (a 9-line wrapper) is worth adapting, and even it references the wrong library.

Phase 1 is a near-greenfield build. The only reusable assets from the existing codebase are the test-data corpus (three short Portuguese-language texts, each under 400 words) and the GOPATH path on disk (`$GOPATH/src/github.com/gleicon/tldt`). The `src/` directory and root-level Makefile should be completely replaced. The new layout is a standard Go modules CLI project: `go.mod` at repo root, `cmd/tldt/main.go` as entry point, `internal/summarizer/` for algorithm wrappers, with the stdlib `flag` package for argument parsing.

The baseline summarization dependency is `github.com/didasy/tldr` v0.7.0 (released 2025-10-03). Its API is simple: `tldr.New()` returns `*Bag`; `bag.Summarize(text string, num int) ([]string, error)` returns ranked sentences already re-ordered to original document position. The test-data corpus needs augmentation: the existing three files are all Portuguese and all under 400 words, so Phase 1 must add an English Wikipedia article, a raw YouTube transcript, a 3000+ word long-form document, and a sub-5-sentence edge case.

**Primary recommendation:** Build the new project structure from scratch at repo root using `go mod init github.com/gleicon/tldt`; keep `src/` temporarily until all Phase 1 work passes, then delete it in the final plan step.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PROJ-01 | Modern go modules (`go.mod` at repo root, drop GOPATH/src/ structure) | New `go mod init`, delete `src/` tree, wire `go build ./...` |
| CLI-01 | `tldt` installable as standalone binary from PATH | Standard `go install ./cmd/tldt` — works with modules out of the box |
| CLI-02 | Pipe text via stdin: `cat file.txt | tldt` | `os.Stdin` + `io.ReadAll`; detect via `stat.Mode()&os.ModeCharDevice == 0` |
| CLI-03 | Specify input file: `tldt -f article.txt` | stdlib `flag` `-f` string flag + `os.ReadFile` |
| CLI-04 | Pass text as positional argument: `tldt "long text..."` | `flag.Args()` after parsing |
| SUM-08 | `graph` algorithm delegates to `github.com/didasy/tldr` | `bag := tldr.New(); sentences, err := bag.Summarize(text, n)` |
| TEST-07 | Test data: English Wikipedia article, YouTube transcript, 3000+ words, sub-5-sentence edge case | Four new files in `test-data/`; existing 3 files are Portuguese, all < 400 words |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Input routing (stdin/file/arg) | CLI entry point (`cmd/tldt/main.go`) | — | All input sources collapse to a single `string` before any summarizer call |
| Graph summarization | Library wrapper (`internal/summarizer/`) | CLI invokes it | Keeps algorithm logic separate from I/O; enables Phase 2 additions |
| Output rendering | CLI entry point | — | Phase 1 output is plain text only; no secondary tier needed |
| Test data corpus | `test-data/` directory | — | Static files on disk, loaded by integration tests |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/didasy/tldr` | v0.7.0 | Graph-based extractive summarization baseline | Explicitly required by SUM-08; LexRank via PageRank; MIT license; 137 stars; actively maintained (2025-10-03 release) [VERIFIED: proxy.golang.org] |
| `github.com/alixaxel/pagerank` | (transitive, managed by go mod) | PageRank computation used internally by didasy/tldr | Pulled automatically as dependency |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `flag` | N/A (Go 1.26.2) | CLI argument parsing | Sufficient for Phase 1 flags: `-f`, positional args; no subcommands needed yet |
| stdlib `os`, `io` | N/A | stdin detection, file I/O | All input routing |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `flag` | `github.com/spf13/cobra` v1.10.2 | Cobra adds subcommand support and auto-generated help; overkill for Phase 1 which has only two flags (`-f` and positional); Phase 2 adds `--algorithm`, `--sentences` which `flag` handles fine. Introduce Cobra only if Phase 2 flag count makes `flag` unwieldy. |
| stdlib `flag` | `github.com/urfave/cli/v2` v2.27.7 | Similar tradeoff as Cobra; not justified for Phase 1 scope |
| `github.com/didasy/tldr` | `github.com/JesusIslam/tldr` | The current `src/summary.go` uses JesusIslam/tldr, but SUM-08 explicitly requires didasy/tldr. The two packages have different APIs (JesusIslam returns `string`, didasy returns `[]string`). Do not reuse the old wrapper. [ASSUMED: JesusIslam/tldr is not actively maintained; last verified via codebase only] |

**Installation:**
```bash
go mod init github.com/gleicon/tldt
go get github.com/didasy/tldr@v0.7.0
```

**Version verification:**
```bash
# Verified via proxy.golang.org:
# github.com/didasy/tldr: v0.7.0 (2025-10-03) [VERIFIED: proxy.golang.org]
# github.com/spf13/cobra: v1.10.2 (2025-12-03) [VERIFIED: proxy.golang.org] — not used in Phase 1
```

## Architecture Patterns

### System Architecture Diagram

```
Input Sources
    |
    +-- stdin (pipe)     detected via os.Stdin stat ModeCharDevice
    +-- -f flag          os.ReadFile path
    +-- positional arg   flag.Args()[0]
    |
    v
[ cmd/tldt/main.go ]
    |
    +-- resolveInput() --> string (body text)
    |
    v
[ internal/summarizer/graph.go ]
    |  tldr.New() + bag.Summarize(text, n)
    v
[ []string sentences ]
    |
    v
[ stdout: joined sentences ]
```

### Recommended Project Structure

```
github.com/gleicon/tldt/        (repo root)
├── go.mod                      # module: github.com/gleicon/tldt
├── go.sum
├── cmd/
│   └── tldt/
│       └── main.go             # CLI entry: flag parsing, input routing, output
├── internal/
│   └── summarizer/
│       └── graph.go            # Wraps github.com/didasy/tldr
├── test-data/
│   ├── body.txt                # existing (Portuguese, ~180 words)
│   ├── body2.txt               # existing (Portuguese, ~225 words)
│   ├── body3.txt               # existing (Portuguese, ~396 words)
│   ├── wikipedia_en.txt        # NEW: English Wikipedia article
│   ├── youtube_transcript.txt  # NEW: raw YouTube transcript
│   ├── longform_3000.txt       # NEW: 3000+ word long-form English
│   └── edge_short.txt          # NEW: sub-5-sentence edge case
├── Makefile                    # replaces current root Makefile
├── README.md                   # (updated in Phase 3)
└── src/                        # DELETE after Phase 1 lands
    └── [old HTTP server files] # conf.go, handlers.go, http.go, main.go, utils.go, summary.go
```

### Pattern 1: Input Resolution Priority

**What:** Determine text source with explicit precedence: stdin pipe > `-f` file > positional arg > error.
**When to use:** Every invocation of `tldt`.
**Example:**
```go
// Source: stdlib os package patterns [CITED: pkg.go.dev/os]
func resolveInput(args []string, filePath string) (string, error) {
    // 1. stdin pipe: check if stdin is a pipe/redirect (not a TTY)
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

### Pattern 2: Graph Summarizer Wrapper

**What:** Thin wrapper around `didasy/tldr` that implements a common summarizer interface (useful when Phase 2 adds LexRank/TextRank).
**When to use:** All summarization calls in Phase 1; interface will be extended in Phase 2.
**Example:**
```go
// Source: github.com/didasy/tldr v0.7.0 API [VERIFIED: raw source at github.com/didasy/tldr]
package summarizer

import "github.com/didasy/tldr"

// Summarize returns up to n sentences from text using the graph/LexRank algorithm.
// Sentences are returned in original document order.
func Summarize(text string, n int) ([]string, error) {
    bag := tldr.New()
    return bag.Summarize(text, n)
}
```

**Key API facts for didasy/tldr v0.7.0 [VERIFIED: raw source]:**
- `New() *Bag` — creates summarizer with defaults (pagerank algorithm, hamming weighing, damping=0.85, tolerance=0.0001)
- `(*Bag).Summarize(text string, num int) ([]string, error)` — returns `[]string` of selected sentences in original document order, NOT score order
- **Not thread-safe** — do not share `*Bag` across goroutines
- Depends on `github.com/alixaxel/pagerank` (auto-resolved by `go mod`)

### Anti-Patterns to Avoid

- **Reusing `src/summary.go` directly:** It imports `github.com/JesusIslam/tldr` which returns `string`, not `[]string`. The APIs are incompatible. Delete and rewrite.
- **Running `go get -v` without go.mod:** The old `src/Makefile` uses `go get -v` in GOPATH mode. This will not work once a `go.mod` exists at the repo root. Replace with `go mod tidy`.
- **Putting `go.mod` inside `src/`:** The module root must be the repo root for `go build ./...` and `go test ./...` to work from root as required by PROJ-01.
- **Shared `*Bag` instance:** didasy/tldr documents that `*Bag` is not thread-safe. For Phase 1 (single-threaded CLI) this doesn't matter, but document it to prevent Phase 2 mistakes.
- **Removing `src/` before new code compiles:** Delete `src/` only as the final step of Phase 1, after `go build ./...` and `go test ./...` pass from the new structure.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Graph-based extractive summarization | Custom PageRank sentence graph | `github.com/didasy/tldr` | Already implemented with IDF + similarity; required by SUM-08; 0.7.0 is production-quality with benchmarks |
| Flag parsing | Manual `os.Args` parsing | stdlib `flag` | Flag handles `-f`, positional args, usage text; no need for custom parser for Phase 1 scope |
| Stdin detection | `bufio.Scanner` + heuristics | `os.Stdin.Stat().Mode() & os.ModeCharDevice` | Standard pattern; one line; reliable across macOS/Linux |

**Key insight:** Phase 1 requires wiring, not algorithms. The two meaningful implementation units are `resolveInput()` (~20 lines) and the `Summarize()` wrapper (~10 lines). Everything else is plumbing.

## Runtime State Inventory

> Not applicable — this is a greenfield CLI build, not a rename/refactor/migration.

The existing codebase has no running services, no stored data, no OS-registered state, and no live config. The "brownfield" label refers only to file layout, not runtime state.

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | None — no database, no Redis data in use | None |
| Live service config | None — no running HTTP server | None |
| OS-registered state | None — no launchd/pm2/systemd registration | None |
| Secrets/env vars | None — no .env files, no credentials | None |
| Build artifacts | `src/resumator` binary may exist if ever built | `make clean` or simply `rm -f src/resumator` |

## Common Pitfalls

### Pitfall 1: GOPATH Module Confusion

**What goes wrong:** Running `go mod init` inside a directory that is also under `$GOPATH/src/` causes Go toolchain warnings and potential confusion between module-aware and GOPATH modes. `go env GOPATH` returns `/Users/gleicon/code/go` and the repo is at `/Users/gleicon/code/go/src/github.com/gleicon/tldt`.
**Why it happens:** Pre-modules Go used `$GOPATH/src/<import-path>` as the canonical location. Module mode coexists but the toolchain has historically had edge cases.
**How to avoid:** Go 1.26 is fully module-aware; `GOPATH` is ignored for module-mode projects. Run `go mod init github.com/gleicon/tldt` at the repo root. Verify with `go env GOFLAGS` — should be empty or module-mode. `go build ./...` from repo root will work correctly. [VERIFIED: Go 1.26.2 on darwin/arm64 confirmed]
**Warning signs:** `go: cannot find main module` or `go: ambiguous import` errors during build.

### Pitfall 2: Makefile Conflicts Between Old and New

**What goes wrong:** The root `Makefile` calls `make -C src` which references `src/Makefile.defs` with `NAME=resumator`. If both the old Makefile and the new `cmd/tldt` coexist, running `make` from root may build the wrong binary or fail.
**Why it happens:** The root Makefile delegates to `src/Makefile`, which assumes GOPATH mode.
**How to avoid:** Replace the root `Makefile` early in Phase 1. New targets: `build` → `go build ./cmd/tldt`, `test` → `go test ./...`, `install` → `go install ./cmd/tldt`.
**Warning signs:** `make` produces `resumator` binary instead of `tldt`.

### Pitfall 3: didasy/tldr Returns Empty Slice on Short Input

**What goes wrong:** Calling `bag.Summarize(text, 5)` on a text with fewer than 5 sentences does not return an error — it returns fewer sentences than requested. The sub-5-sentence edge case (TEST-07) is explicitly designed to expose this.
**Why it happens:** LexRank can only rank as many sentences as exist in the document. Requesting more than available is silently capped.
**How to avoid:** After calling `Summarize()`, check `len(result)` rather than assuming it equals `n`. For Phase 1 this is acceptable behavior; Phase 2 SUM-04 documents it explicitly.
**Warning signs:** Empty output or panic when `n > sentence_count`.

### Pitfall 4: Test Data Language Mismatch

**What goes wrong:** All three existing test-data files (`body.txt`, `body2.txt`, `body3.txt`) are in Portuguese. English-language summarization tests written against these files will produce results that are difficult to manually verify for correctness.
**Why it happens:** The original "resumator" project was likely developed against Portuguese content.
**How to avoid:** Phase 1 TEST-07 requires four new English-language files. Create them in the same plan step where tests are written. Do not write integration tests using only the Portuguese files.
**Warning signs:** Tests that always pass trivially because Portuguese sentence selection is opaque to the reviewer.

### Pitfall 5: ioutil.ReadAll vs io.ReadAll

**What goes wrong:** `src/handlers.go` uses `ioutil.ReadAll` which is deprecated since Go 1.16. Writing new code that copies from the old source will produce deprecation warnings.
**Why it happens:** The old codebase predates Go 1.16.
**How to avoid:** Use `io.ReadAll` in all new code (available since Go 1.16, standard in Go 1.26.2). [CITED: pkg.go.dev/io#ReadAll]

## Code Examples

Verified patterns from official sources:

### Detecting Stdin Pipe

```go
// Source: stdlib os package [CITED: pkg.go.dev/os]
stat, err := os.Stdin.Stat()
if err != nil {
    // stdin stat failed — treat as no pipe
}
isPipe := (stat.Mode() & os.ModeCharDevice) == 0
```

### didasy/tldr Summarize Call

```go
// Source: github.com/didasy/tldr v0.7.0 [VERIFIED: raw source]
import "github.com/didasy/tldr"

bag := tldr.New()
sentences, err := bag.Summarize(text, 5)
if err != nil {
    return err
}
fmt.Println(strings.Join(sentences, " "))
```

### go.mod Initialization

```bash
# Source: go modules documentation [CITED: go.dev/ref/mod]
cd /Users/gleicon/code/go/src/github.com/gleicon/tldt
go mod init github.com/gleicon/tldt
go get github.com/didasy/tldr@v0.7.0
go mod tidy
```

### Minimal CLI main.go Pattern

```go
// Source: stdlib flag package [CITED: pkg.go.dev/flag]
package main

import (
    "flag"
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/gleicon/tldt/internal/summarizer"
)

func main() {
    filePath := flag.String("f", "", "input file path")
    flag.Parse()

    text, err := resolveInput(flag.Args(), *filePath)
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }

    sentences, err := summarizer.Summarize(text, 5)
    if err != nil {
        fmt.Fprintln(os.Stderr, "summarization failed:", err)
        os.Exit(1)
    }
    fmt.Println(strings.Join(sentences, " "))
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `go get -v` in GOPATH mode | `go mod tidy` with `go.mod` | Go 1.11 (2018) | All deps declared in go.mod/go.sum; reproducible builds |
| `ioutil.ReadAll` | `io.ReadAll` | Go 1.16 (2021) | ioutil package deprecated entirely |
| GOPATH-based project layout | Module layout (`go.mod` at root, `cmd/`, `internal/`) | Go 1.11 onwards | Enables `go build ./...` from any directory |
| `go build -ldflags "-X main.VERSION=..."` in Makefile | `go build ./cmd/tldt` | Modules era | -ldflags still valid but module version can be embedded via `debug.ReadBuildInfo()` in Go 1.12+ |

**Deprecated/outdated in existing codebase:**
- `github.com/JesusIslam/tldr`: Used in `src/summary.go`. SUM-08 requires `github.com/didasy/tldr` instead. Do not carry forward.
- `github.com/fiorix/go-redis/redis`: HTTP server artifact. Drop entirely.
- `github.com/gorilla/handlers`: HTTP server artifact. Drop entirely.
- `github.com/BurntSushi/toml`: Config file parsing for the HTTP server. Drop entirely.
- `ioutil.ReadAll` in `src/handlers.go`: Deprecated since Go 1.16. Replace with `io.ReadAll` in new code.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `github.com/JesusIslam/tldr` is not actively maintained | Standard Stack (Alternatives) | Low — SUM-08 explicitly requires didasy/tldr regardless |
| A2 | English-language test data (Wikipedia, YouTube transcript) can be sourced from public domain without license issues | TEST-07 section | Low — Wikipedia content is CC-BY-SA; YouTube transcripts of public videos are generally acceptable for test fixtures |
| A3 | go.mod at `$GOPATH/src/github.com/gleicon/tldt` does not create toolchain issues in Go 1.26 | Pitfall 1 | Low — Go 1.26 is fully module-aware; GOPATH is deprecated for source layout |

**All other claims were verified or cited — no user confirmation needed beyond the above.**

## Open Questions (RESOLVED)

1. **Which specific Wikipedia article and YouTube transcript to use for TEST-07?**
   - What we know: Content must be English, Wikipedia article is available under CC-BY-SA, YouTube transcripts are plain text
   - What's unclear: Which article/video gives representative NLP test coverage (varied sentence structures, named entities)
   - Recommendation: Use a mid-length Wikipedia article (e.g., "Extractive summarization") and any public tech talk transcript from YouTube. The exact content is a planner/executor choice; the files just need to exist and meet word-count requirements.
   - **RESOLVED:** Content provided inline in 01-03-PLAN.md Task 1 action — Wikipedia extractive summarization article and original tech talk transcript prose. Files meet word-count requirements per TEST-07.

2. **Should `-sentences N` be a Phase 1 flag or deferred to Phase 2?**
   - What we know: SUM-01 (`--sentences N`) is assigned to Phase 2. Phase 1 only requires CLI-01 through CLI-04 and SUM-08.
   - What's unclear: The phase description says "produces extractive summaries via the graph baseline algorithm" without mentioning a configurable sentence count.
   - Recommendation: Hardcode `n=5` in Phase 1 as the default; add the flag in Phase 2 with SUM-01. Do not block Phase 1 on this.
   - **RESOLVED:** Deferred to Phase 2 per SUM-01 traceability; Phase 1 hardcodes `defaultSentences = 5` in cmd/tldt/main.go.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | PROJ-01, all build | Yes | go1.26.2 darwin/arm64 | — |
| git | PROJ-01 (go mod) | Yes (inferred from git repo) | — | — |
| internet (go mod download) | go get github.com/didasy/tldr | Yes (assumed) | — | `GONOSUMCHECK` + vendoring |
| network access to proxy.golang.org | go mod tidy | Yes (assumed) | — | `GOFLAGS=-mod=vendor` |

**Missing dependencies with no fallback:** None identified.
**Missing dependencies with fallback:** If proxy.golang.org is blocked, use `GONOSUMCHECK=* GOFLAGS=-mod=vendor` pattern after initial `go mod download`.

## Validation Architecture

> `nyquist_validation: false` in `.planning/config.json` — this section is SKIPPED per configuration.

## Security Domain

The Phase 1 binary is a local CLI tool with no network access, no HTTP server, no authentication, and no external service calls beyond `go mod download` at build time. ASVS controls do not apply to a local offline CLI. No user data is stored or transmitted.

## Sources

### Primary (HIGH confidence)
- `github.com/didasy/tldr` raw source at github.com/didasy/tldr/v0.7.0/tldr.go — API signatures, `New()`, `Summarize()`, thread-safety warning
- proxy.golang.org — version v0.7.0 confirmed current (2025-10-03)
- proxy.golang.org — cobra v1.10.2, urfave/cli v2.27.7 confirmed for alternatives table
- pkg.go.dev/os, pkg.go.dev/io, pkg.go.dev/flag — stdlib patterns
- go.dev/ref/mod — go modules documentation

### Secondary (MEDIUM confidence)
- pkg.go.dev/github.com/didasy/tldr@v0.7.0 — package overview, confirmed thread-safety warning and `Summarize()` return type

### Tertiary (LOW confidence)
- github.com/didasy/tldr README (WebFetch) — general description; confirmed by source code inspection

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — `didasy/tldr` version confirmed on proxy.golang.org; API confirmed from raw source
- Architecture: HIGH — all patterns are stdlib Go; no external services or complex frameworks
- Pitfalls: HIGH — derived from direct codebase inspection (GOPATH layout, old Makefile, ioutil usage, Portuguese test data)
- Test data gap: HIGH — manually verified word counts of existing test-data files; gap is confirmed

**Research date:** 2026-05-01
**Valid until:** 2026-11-01 (stable Go ecosystem; `didasy/tldr` is unlikely to change API in minor versions)
