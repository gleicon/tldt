# Phase 3: Polish - Research

**Researched:** 2026-05-02
**Domain:** Go CLI hardening — TTY detection, structured output formats, input validation, O(n^2) safety, README
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CLI-05 | When stdout is piped, output contains ONLY summary text (no metadata, no decoration) | TTY detection via `os.Stdout.Stat()` + `os.ModeCharDevice` — stdlib, no new deps |
| CLI-06 | When stdout is a TTY, output includes compression stats to stderr | Same TTY check; emit `~N -> ~M tokens (P% reduction)` only when `isTerminal == true` |
| CLI-07 | Empty or whitespace-only input exits 0 with no output (pipe-safe) | `strings.TrimSpace(text) == ""` check after `resolveInput`; skip all output including stderr stats |
| CLI-08 | Binary/non-text input detected and rejected with error to stderr and exit non-zero | `unicode/utf8.ValidString()` + NUL byte check (`bytes.IndexByte(data, 0) >= 0`); stdlib only |
| OUT-01 | Default output is plain text (pipe-safe) | Already works; `--format` flag defaults to `"text"` |
| OUT-02 | `--format json` outputs structured JSON | `encoding/json.MarshalIndent()` on an output struct; stdlib only |
| OUT-03 | `--format markdown` wraps summary in markdown blockquote with metadata header | Pure string formatting with `>` prefix; no library needed |
| PROJ-02 | Updated README | Write README.md from scratch; old file is a stale web-server template |
| PROJ-03 | Sentence cap at 2000 (default) with `--no-cap` flag | `TokenizeSentences()` is exported; cap in `main.go` before calling `Summarize()` |
| PROJ-04 | Build via `go build ./...`, test via `go test ./...` | Already passes; research confirms no new build tooling needed |
</phase_requirements>

---

## Summary

Phase 3 is a hardening and polish phase on top of a fully working Phase 2 binary. All core algorithms, flags, and test infrastructure are in place. The work is concentrated in `cmd/tldt/main.go` (new flag, TTY guard, format dispatch, sentence cap), a new output formatter module, and `README.md`.

The three biggest additions are: (1) TTY-aware stats gate — token compression stats on stderr only when stdout is a terminal; (2) structured output formats JSON and Markdown via a `--format` flag; and (3) the 2000-sentence cap for O(n²) safety with a `--no-cap` escape hatch.

All required functionality uses Go stdlib (`os`, `unicode/utf8`, `bytes`, `encoding/json`, `strings`, `fmt`). No new dependencies are needed. The existing `golang.org/x/term` package was briefly evaluated and **not recommended** — the stdlib `os.ModeCharDevice` check already used in `resolveInput()` provides equivalent TTY detection with zero new deps.

**Primary recommendation:** Implement all changes in `cmd/tldt/main.go` and a new `internal/formatter/formatter.go` file. Keep the summarizer package untouched except for adding the 2000-sentence cap inside each `Summarize()` method OR applying it in `main.go` using the exported `TokenizeSentences()`. The `main.go`-side cap is preferred (see Architecture section).

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| TTY detection | CLI (`cmd/tldt/main.go`) | — | stdout is a property of the process, not the algorithm |
| Stats emission | CLI (`cmd/tldt/main.go`) | — | gated by TTY flag; no stats in pipe mode |
| Empty/whitespace guard | CLI (`cmd/tldt/main.go`) | — | input validation before calling summarizer |
| Binary input rejection | CLI (`cmd/tldt/main.go`) | — | validate raw bytes before string conversion |
| JSON/Markdown formatting | `internal/formatter/` | CLI calls it | separates rendering concern from CLI wiring |
| Sentence cap (PROJ-03) | CLI (`cmd/tldt/main.go`) | summarizer package | cap in `main.go` keeps summarizer interface stable |
| README | repo root | — | documentation artifact |

---

## Standard Stack

### Core (all stdlib — no new imports needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os` | stdlib | Stdout TTY detection via `Stat()` + `ModeCharDevice` | Already used in `resolveInput()`; zero deps |
| `unicode/utf8` | stdlib | `utf8.ValidString()` for binary detection | Correct UTF-8 validity check |
| `bytes` | stdlib | NUL byte check: `bytes.IndexByte(data, 0)` | Catches binary files that pass as valid UTF-8 |
| `encoding/json` | stdlib | `json.MarshalIndent()` for structured output | Standard Go JSON serialization |
| `strings` | stdlib | `strings.TrimSpace()` for whitespace check; `>` prefix for markdown | Already imported |
| `fmt` | stdlib | All formatting | Already imported |

### No New Dependencies

`golang.org/x/term` was evaluated:
- Provides `term.IsTerminal(int(os.Stdout.Fd()))` — cleaner API
- But adds a module dependency; the existing `os.ModeCharDevice` pattern already works [VERIFIED: codebase inspection]
- **Decision: stay stdlib-only.** `os.ModeCharDevice` approach is already proven in `resolveInput()` on line 147 of `main.go`. [VERIFIED: codebase]

**Version verification:** `go.mod` currently has: `go 1.26.2`, `require github.com/didasy/tldr v0.7.0`. No changes to `go.mod` required. [VERIFIED: codebase]

---

## Architecture Patterns

### System Architecture Diagram

```
stdin / -f flag / positional arg
        |
        v
[resolveInput()] --> raw bytes
        |
        v
[validateInput()]  <-- NEW: whitespace check + binary check
   |         |
empty     binary
exit 0    exit 1 (stderr error)
   |
   v
[sentence cap]  <-- NEW: if len > 2000 and !noCap, truncate
   |
   v
[summarizer.New(algo).Summarize(text, n)]
        |
        v
[formatOutput(result, format, metadata)]  <-- NEW: text | json | markdown
        |
        v
   stdout (summary text ONLY)
        |
   [TTY gate]  <-- NEW: check os.Stdout
        |
        v (only when stdout is terminal)
   stderr (token compression stats)
```

### Recommended Project Structure

```
cmd/tldt/
  main.go           # CLI wiring (modified: TTY gate, --format, --no-cap flags, input validation)
internal/
  formatter/
    formatter.go    # NEW: formatText / formatJSON / formatMarkdown
    formatter_test.go  # NEW: unit tests for each format
  summarizer/       # UNCHANGED from Phase 2
```

### Pattern 1: TTY Detection for Stdout

**What:** Check if `os.Stdout` is connected to a terminal (TTY) or redirected to a file/pipe.
**When to use:** To decide whether to emit token stats to stderr.

```go
// Source: stdlib os package — same pattern already used in resolveInput() line 147
func stdoutIsTerminal() bool {
    stat, err := os.Stdout.Stat()
    if err != nil {
        return false
    }
    return (stat.Mode() & os.ModeCharDevice) != 0
}
```

[VERIFIED: existing code in `cmd/tldt/main.go` line 147 uses identical pattern for stdin]

### Pattern 2: Binary Input Detection

**What:** Reject input that is not valid UTF-8 text, or that contains NUL bytes (binary files).
**When to use:** After reading the raw input bytes, before summarizing.

```go
// Source: stdlib unicode/utf8 and bytes packages
func validateText(data []byte) error {
    if bytes.IndexByte(data, 0) >= 0 {
        return fmt.Errorf("binary input detected: NUL byte found")
    }
    if !utf8.Valid(data) {
        return fmt.Errorf("binary input detected: invalid UTF-8 encoding")
    }
    return nil
}
```

[VERIFIED: `unicode/utf8.Valid()` is stdlib; `bytes.IndexByte()` is stdlib]

Note: `resolveInput()` currently returns a `string`. To validate before string conversion, the internal read should expose `[]byte` for validation before converting. The cleanest approach: validate in `resolveInput()` before returning, converting `[]byte` to `string` only after validation passes.

### Pattern 3: Structured JSON Output (OUT-02)

**What:** Marshal a fixed struct to JSON.
**Required fields:** `summary` ([]string), `algorithm` (string), `sentences_in` (int), `sentences_out` (int), `chars_in` (int), `chars_out` (int), `tokens_estimated_in` (int), `tokens_estimated_out` (int), `compression_ratio` (float64).

```go
// Source: encoding/json stdlib
type JSONOutput struct {
    Summary             []string `json:"summary"`
    Algorithm           string   `json:"algorithm"`
    SentencesIn         int      `json:"sentences_in"`
    SentencesOut        int      `json:"sentences_out"`
    CharsIn             int      `json:"chars_in"`
    CharsOut            int      `json:"chars_out"`
    TokensEstimatedIn   int      `json:"tokens_estimated_in"`
    TokensEstimatedOut  int      `json:"tokens_estimated_out"`
    CompressionRatio    float64  `json:"compression_ratio"`
}

out, err := json.MarshalIndent(JSONOutput{...}, "", "  ")
```

[VERIFIED: `encoding/json` is stdlib; field tags are standard Go JSON convention]

When `--format json`, token stats MUST NOT be emitted to stderr even in TTY mode. The stats are in the JSON payload. [ASSUMED: inferred from requirement spec; not explicitly stated]

### Pattern 4: Markdown Blockquote Output (OUT-03)

**What:** Wrap each summary sentence with `> ` prefix; prepend a metadata comment header.
**Format:**

```
<!-- tldt | algorithm: lexrank | sentences: 5 | compression: 89% -->
> First selected sentence.
>
> Second selected sentence.
```

[ASSUMED: exact header format not specified in requirements — the above is a reasonable interpretation of "metadata header". Planner should confirm or standardize this format.]

```go
// Pure string formatting, no library needed
func formatMarkdown(sentences []string, algo string, compression int) string {
    var b strings.Builder
    fmt.Fprintf(&b, "<!-- tldt | algorithm: %s | sentences: %d | compression: %d%% -->\n",
        algo, len(sentences), compression)
    for i, s := range sentences {
        if i > 0 {
            b.WriteString(">\n")
        }
        fmt.Fprintf(&b, "> %s\n", s)
    }
    return b.String()
}
```

### Pattern 5: Sentence Cap (PROJ-03)

**What:** Limit input to 2000 sentences before calling `Summarize()` to prevent O(n²) hang on huge inputs.
**Where:** In `main.go`, using the already-exported `summarizer.TokenizeSentences()`.
**Why `main.go` vs inside summarizer:** Keeps the `Summarizer` interface stable; `--no-cap` is a CLI-level decision.

```go
// Source: internal/summarizer/tokenizer.go (exported)
const defaultSentenceCap = 2000

func capInput(text string, cap int) string {
    sentences := summarizer.TokenizeSentences(text)
    if len(sentences) <= cap {
        return text
    }
    return strings.Join(sentences[:cap], " ")
}
```

[VERIFIED: `TokenizeSentences` is exported — `func TokenizeSentences(text string) []string` in tokenizer.go]

Note: Rejoining with `" "` is acceptable because the algorithms call `TokenizeSentences()` again internally. The rejoined string will re-tokenize identically.

### Pattern 6: Stats Format Fix (CLI-06)

Current code emits: `tokens: 30 -> 30 (0% reduction)` [VERIFIED: codebase line 81]
Required format per SUCCESS-02: `~12,400 -> ~1,380 tokens (89% reduction)`

Changes needed:
1. Add `~` prefix to both token counts in the format string
2. Move `"tokens"` to appear after the arrow and counts

```go
// Change from:
fmt.Fprintf(os.Stderr, "tokens: %s -> %s (%d%% reduction)\n", ...)
// Change to:
fmt.Fprintf(os.Stderr, "~%s -> ~%s tokens (%d%% reduction)\n", ...)
```

[VERIFIED: current format in main.go line 81; required format from ROADMAP.md Phase 3 success criteria]

### Anti-Patterns to Avoid

- **Checking stdin for TTY instead of stdout:** CLI-05/06 are about stdout pipe detection. Stdin TTY detection (already done in `resolveInput`) is a separate concern.
- **Adding golang.org/x/term dependency:** Unnecessary when `os.ModeCharDevice` stdlib approach already works and is used in the codebase.
- **Modifying the `Summarizer` interface to accept a cap parameter:** Would require changes to LexRank, TextRank, and Graph structs. Keep the interface stable; apply cap at the CLI layer.
- **Emitting token stats to stderr unconditionally:** Phase 2 did this as a design decision (D-09), but Phase 3 gates it to TTY mode only. The `--format json` case should suppress stderr stats entirely (they're in the JSON).
- **Binary detection via file extension or MIME:** NUL byte + `utf8.Valid()` is the reliable programmatic approach for stdin input that has no filename.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON serialization | Custom string builder for JSON | `encoding/json.MarshalIndent()` | Edge cases: string escaping, unicode, nil slices |
| UTF-8 validation | Byte-range checks | `unicode/utf8.Valid([]byte)` | Handles multi-byte sequences, surrogate pairs correctly |
| TTY detection | Platform-specific syscall wrappers | `os.File.Stat()` + `os.ModeCharDevice` | Cross-platform, already in codebase |

**Key insight:** Phase 3 is about wiring, not new algorithms. Every capability is achievable with stdlib. The risk is not in complexity but in edge cases (empty JSON fields, whitespace-only NUL-free binary, TTY check on non-standard stdout).

---

## Common Pitfalls

### Pitfall 1: TTY Check on Wrong File Descriptor
**What goes wrong:** Checking `os.Stdin.Stat()` instead of `os.Stdout.Stat()` for the pipe-safety gate.
**Why it happens:** The existing `resolveInput()` checks stdin; it's easy to copy the wrong check.
**How to avoid:** `stdoutIsTerminal()` must call `os.Stdout.Stat()`, not `os.Stdin.Stat()`.
**Warning signs:** `tldt -f file.txt | wc -l` still shows token stats in output.

### Pitfall 2: Token Stats on Empty Input
**What goes wrong:** Current code (Phase 2) emits `tokens: 0 -> 0 (0% reduction)` to stderr even for empty input. CLI-07 requires no output at all for empty/whitespace input.
**How to avoid:** Early return after whitespace check, before the token stats computation.
**Warning signs:** `echo "" | tldt 2>&1 | wc -c` returns non-zero.

### Pitfall 3: `fmt.Println` Adds Extra Newline
**What goes wrong:** `fmt.Println(strings.Join(result, "\n"))` adds a trailing newline. This is fine for plain text, but for JSON output `fmt.Println` wraps a `json.MarshalIndent` result and adds `\n` twice (once from `MarshalIndent` not adding one, and once from `Println`). Test with `tldt --format json | python3 -m json.tool` to verify.
**How to avoid:** Use `fmt.Print(string(out) + "\n")` for JSON, or ensure `MarshalIndent` + `Println` produces valid parseable JSON.

### Pitfall 4: Binary Detection False Positives
**What goes wrong:** Valid UTF-8 text with no NUL bytes but unusual characters (emoji, CJK) gets rejected.
**Why it happens:** Over-aggressive binary detection heuristic.
**How to avoid:** Use only two checks: (1) NUL byte presence, (2) `utf8.Valid()`. Do not check byte value ranges beyond what these two cover.

### Pitfall 5: Sentence Cap Joins Lose Sentence Boundaries
**What goes wrong:** Joining capped sentences with `" "` causes the tokenizer's regex to see no sentence boundary between the last sentence of one group and first of the next (if they collide).
**Why it happens:** The tokenizer regex `[.!?]["''"]?\s+[A-Z"'"]` looks for punctuation + whitespace + capital — which is preserved when joining with `" "`.
**How to avoid:** Use `" "` as the joiner (space). The sentence-ending punctuation on each sentence fragment ensures the regex boundary survives. [VERIFIED: tokenizer.go regex pattern inspected]

### Pitfall 6: JSON Output Emits Stats to Stderr
**What goes wrong:** `--format json` still prints `~N -> ~M tokens (89% reduction)` to stderr in TTY mode, polluting stderr for scripts that capture both stdout and stderr.
**How to avoid:** Gate stderr stats with `isTTY && format == "text"` condition.

---

## Code Examples

### Full TTY Gate Pattern
```go
// Source: derived from existing resolveInput() pattern in cmd/tldt/main.go line 147
isTTY := func() bool {
    stat, err := os.Stdout.Stat()
    return err == nil && (stat.Mode()&os.ModeCharDevice) != 0
}()

// Only emit token stats when running interactively AND not in JSON format
if isTTY && *format == "text" {
    fmt.Fprintf(os.Stderr, "~%s -> ~%s tokens (%d%% reduction)\n",
        formatTokens(tokIn), formatTokens(tokOut), reduction)
}
```

### Binary + Whitespace Validation
```go
// Source: stdlib bytes and unicode/utf8
func validateInput(data []byte) (string, error) {
    if bytes.IndexByte(data, 0) >= 0 {
        return "", fmt.Errorf("binary input: NUL byte found")
    }
    if !utf8.Valid(data) {
        return "", fmt.Errorf("binary input: invalid UTF-8")
    }
    text := string(data)
    if strings.TrimSpace(text) == "" {
        return "", nil // signal empty — caller exits 0
    }
    return text, nil
}
```

### Sentence Cap
```go
// Source: exported TokenizeSentences from internal/summarizer/tokenizer.go
const defaultSentenceCap = 2000

func applySentenceCap(text string, cap int) string {
    sents := summarizer.TokenizeSentences(text)
    if len(sents) <= cap {
        return text
    }
    return strings.Join(sents[:cap], " ")
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Token stats always on stderr | Token stats only on TTY stderr | Phase 3 | Pipe-safe: `cat f | tldt | wc -l` works cleanly |
| No format flag | `--format text\|json\|markdown` | Phase 3 | Structured output for scripting |
| No input validation | UTF-8 + NUL byte check | Phase 3 | Rejects binary piped by mistake |
| No sentence cap | Cap at 2000, `--no-cap` escape | Phase 3 | O(n²) safety for large docs |

**Deprecated/outdated from Phase 2:**
- Token stats format `"tokens: N -> M (P% reduction)"` → changes to `"~N -> ~M tokens (P% reduction)"` per SUCCESS-02

---

## Gap Analysis: What Exists vs What's Needed

[VERIFIED: codebase inspection]

| Requirement | Exists? | Gap |
|-------------|---------|-----|
| Token stats on stderr | Yes (unconditional) | Must gate to TTY mode only |
| Stats format `~N -> ~M tokens` | No — current: `tokens: N -> M` | Fix format string in main.go |
| Empty input exits 0 silently | Partial — exits 0 but prints stderr stats | Suppress stderr stats for empty |
| Binary detection | No | Add `validateInput()` |
| `--format` flag | No | Add flag + dispatch |
| JSON output | No | Add formatter |
| Markdown output | No | Add formatter |
| Sentence cap | No | Add `--no-cap` flag + `applySentenceCap()` |
| README | Stale (old web server template) | Full rewrite |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `--format json` should suppress stderr stats even in TTY mode | Architecture, Pitfall 6 | Minor UX inconsistency; easy to change |
| A2 | Markdown metadata header format is `<!-- tldt \| algorithm: X \| sentences: N \| compression: P% -->` | Pattern 4 | If planner/user wants different format, adjust formatter.go only |
| A3 | Cap logic belongs in `main.go` (not inside summarizer methods) | Architecture, Pattern 5 | If wrong, would require changes to summarizer interface — more disruptive |
| A4 | Joining capped sentences with `" "` produces correct re-tokenization | Pitfall 5 | Could cause last-sentence boundary merge; test with edge case |

---

## Open Questions

1. **Markdown header format (A2)**
   - What we know: OUT-03 says "metadata header" but doesn't specify the exact format
   - What's unclear: HTML comment vs a table vs a code fence header; which metadata fields to include
   - Recommendation: Use `<!-- tldt | algorithm: X | sentences: N | compression: P% -->` — it's invisible when rendered and informative when read as source

2. **JSON stats suppression with TTY (A1)**
   - What we know: JSON output already contains `tokens_estimated_in`, `tokens_estimated_out`, `compression_ratio`
   - What's unclear: should stderr also get stats when `--format json` and TTY?
   - Recommendation: No stderr stats for JSON format — the stats are in the payload; double-reporting is noise

3. **Sentence cap placement (A3)**
   - What we know: `TokenizeSentences` is exported; cap in main.go keeps summarizer interface stable
   - What's unclear: whether Graph summarizer (didasy/tldr) also needs to respect the cap
   - Recommendation: Apply cap before calling ANY summarizer (including graph) since it applies to input size

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + test | Yes | 1.26.2 darwin/arm64 | — |
| `encoding/json` | OUT-02 | Yes (stdlib) | — | — |
| `unicode/utf8` | CLI-08 | Yes (stdlib) | — | — |
| `bytes` | CLI-08 | Yes (stdlib) | — | — |
| `os` | CLI-05/06 | Yes (stdlib) | — | — |

[VERIFIED: `go version go1.26.2 darwin/arm64`; `go build ./... && go test ./...` pass with 49 tests]

**Missing dependencies with no fallback:** None.

---

## Validation Architecture

> `workflow.nyquist_validation` is `false` in `.planning/config.json` — this section is skipped per config.

---

## Security Domain

Phase 3 introduces input handling paths (binary detection, UTF-8 validation). Applicable ASVS categories are narrow for a local CLI tool.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes | `utf8.Valid()` + NUL byte check; `strings.TrimSpace()` |
| V2 Authentication | no | Local CLI, no auth |
| V3 Session Management | no | Stateless CLI |
| V4 Access Control | no | No ACL |
| V6 Cryptography | no | No crypto |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malicious binary piped to stdin | Tampering | NUL byte + utf8.Valid() check; exit non-zero with stderr error |
| Extremely large input (DoS via O(n²)) | DoS | 2000-sentence cap; --no-cap is opt-in |
| Path traversal via -f flag | Tampering | `os.ReadFile()` uses OS-level path resolution; no additional sanitization needed for local CLI |

---

## Sources

### Primary (HIGH confidence)
- Codebase: `cmd/tldt/main.go` — verified current implementation state, existing TTY pattern, token stats format [VERIFIED]
- Codebase: `internal/summarizer/tokenizer.go` — verified `TokenizeSentences` is exported [VERIFIED]
- Go stdlib docs: `os`, `unicode/utf8`, `bytes`, `encoding/json` — all available in Go 1.26.2 [VERIFIED: go doc]
- `.planning/REQUIREMENTS.md` — source of truth for all requirement IDs and descriptions [VERIFIED]
- `.planning/ROADMAP.md` — Phase 3 success criteria including exact stats format [VERIFIED]

### Secondary (MEDIUM confidence)
- `.planning/phases/02-algorithms/02-RESEARCH.md` — Phase 2 design decisions (D-09: stats always to stderr); Phase 3 overrides D-09 with TTY gate

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib, already used in codebase
- Architecture: HIGH — direct code inspection of existing implementation
- Pitfalls: HIGH — derived from verified code state and requirement gap analysis

**Research date:** 2026-05-02
**Valid until:** 2026-08-02 (Go stdlib is stable; 90-day window appropriate)
