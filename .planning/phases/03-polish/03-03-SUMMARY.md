---
phase: "03-polish"
plan: "03"
subsystem: "cli"
tags: ["go", "cli", "formatter", "output-formats", "json", "markdown", "tty"]
dependency_graph:
  requires: ["03-01", "03-02"]
  provides: ["--format flag", "formatter dispatch", "json-output", "markdown-output", "tty-json-stats-gate"]
  affects: ["cmd/tldt/main.go"]
tech_stack:
  added: ["github.com/gleicon/tldt/internal/formatter"]
  patterns: ["switch-based format dispatch", "SummaryMeta population from computed stats", "TTY+format combined gate"]
key_files:
  created: []
  modified:
    - "cmd/tldt/main.go"
decisions:
  - "Use float64(tokIn-tokOut) / float64(tokIn+1) for CompressionRatio to guard divide-by-zero when tokIn==0"
  - "Suppress stderr token stats when --format json is active (stats are in JSON payload)"
  - "fmt.Print (not Println) for markdown to avoid double trailing newline since FormatMarkdown already ends with newline"
metrics:
  duration: "~1 min"
  completed_date: "2026-05-02"
  tasks_completed: 1
  files_created: 0
  files_modified: 1
---

# Phase 3 Plan 03: Format Flag Wiring Summary

**One-liner:** Wired `--format text|json|markdown` flag into `cmd/tldt/main.go` dispatching through `internal/formatter` functions with TTY stats suppressed for JSON format.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add --format flag and dispatcher to main.go | 9bbe8de | cmd/tldt/main.go |

## What Was Built

### cmd/tldt/main.go

Three targeted changes applied:

1. **`--format` flag** — `flag.String("format", "text", "output format: text|json|markdown")` added after `--no-cap`. Usage string updated to include `[-format text|json|markdown]`.

2. **Formatter import** — `"github.com/gleicon/tldt/internal/formatter"` added to import block.

3. **Format dispatch block** — Replaced static `fmt.Println(strings.Join(result, "\n"))` output with:
   - `SummaryMeta` struct populated from `charsIn`, `charsOut`, `tokIn`, `tokOut`, `len(result)`, `len(summarizer.TokenizeSentences(text))`
   - `switch *format` dispatching to `formatter.FormatJSON`, `formatter.FormatMarkdown`, or default text path
   - `CompressionRatio` computed as `float64(tokIn-tokOut) / float64(tokIn+1)` (divide-by-zero guard)

4. **TTY stats gate tightened** — Changed `if isTTY` to `if isTTY && *format != "json"` so JSON format carries stats in payload, not on stderr.

## Verification Results

All acceptance criteria verified:

- `go build ./...` passes
- `go test ./... -count=1` passes (57 tests across 3 packages)
- `--format json` produces valid JSON with all 9 OUT-02 fields: `summary`, `algorithm`, `sentences_in`, `sentences_out`, `chars_in`, `chars_out`, `tokens_estimated_in`, `tokens_estimated_out`, `compression_ratio`
- `--format markdown` output starts with `<!-- tldt | algorithm: lexrank | sentences: 5 | compression: 4% -->`
- `--format text` (default) produces one sentence per line with no metadata

## Deviations from Plan

None — plan executed exactly as written. All three changes match plan specification exactly.

## Known Stubs

None — all three format paths are fully wired and return real output from the formatter package.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced. The `--format` flag value is consumed by a `switch` statement with a safe default fallback (text), implementing T-03-06 (unknown format falls to text). The JSON compression stats derive only from character/token counts, not file paths or credentials, consistent with T-03-07 acceptance.

## Self-Check: PASSED

- `cmd/tldt/main.go` exists: FOUND
- Commit 9bbe8de exists: FOUND
- `cmd/tldt/main.go` contains `flag.String("format", "text", "output format: text|json|markdown")`: PRESENT
- `cmd/tldt/main.go` contains `"github.com/gleicon/tldt/internal/formatter"`: PRESENT
- `cmd/tldt/main.go` contains `formatter.SummaryMeta{`: PRESENT
- `cmd/tldt/main.go` contains `formatter.FormatJSON(result, meta)`: PRESENT
- `cmd/tldt/main.go` contains `formatter.FormatMarkdown(result, meta)`: PRESENT
- `cmd/tldt/main.go` contains `case "json":`: PRESENT
- `cmd/tldt/main.go` contains `case "markdown":`: PRESENT
- `cmd/tldt/main.go` contains `isTTY && *format != "json"`: PRESENT
- `go build ./...` exits 0: PASSED
- `go test ./... -count=1` exits 0 (57 tests): PASSED
- JSON output contains all 9 required fields: VERIFIED
- Markdown output starts with HTML comment header: VERIFIED
