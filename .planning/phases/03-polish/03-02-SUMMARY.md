---
phase: "03-polish"
plan: "02"
subsystem: "formatter"
tags: ["go", "formatter", "json", "markdown", "output-formats", "tdd"]
dependency_graph:
  requires: []
  provides: ["internal/formatter.FormatText", "internal/formatter.FormatJSON", "internal/formatter.FormatMarkdown", "internal/formatter.SummaryMeta", "internal/formatter.JSONOutput"]
  affects: ["cmd/tldt/main.go"]
tech_stack:
  added: []
  patterns: ["encoding/json.MarshalIndent", "strings.Builder blockquote construction", "TDD RED/GREEN cycle"]
key_files:
  created:
    - "internal/formatter/formatter.go"
    - "internal/formatter/formatter_test.go"
  modified: []
decisions:
  - "Blank blockquote separator line is '>' (no trailing space) — test updated to allow bare '>' as valid separator"
  - "nil sentences slice normalized to []string{} before marshalling to ensure JSON array (not null)"
  - "CompressionRatio multiplied by 100 and cast to int for % display in Markdown header"
metrics:
  duration: "~8 min"
  completed_date: "2026-05-02"
  tasks_completed: 1
  files_created: 2
  files_modified: 0
---

# Phase 3 Plan 02: Formatter Package Summary

**One-liner:** New `internal/formatter` package with `FormatText`, `FormatJSON`, `FormatMarkdown` functions using `encoding/json.MarshalIndent` and `strings.Builder` for all three OUT-0x output format requirements.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 RED | Add failing formatter tests | 65a7056 | internal/formatter/formatter_test.go |
| 1 GREEN | Implement formatter package | 765ddfe | internal/formatter/formatter.go, formatter_test.go |

## What Was Built

### internal/formatter/formatter.go

Exports three pure functions and two types:

- `SummaryMeta` struct — metadata populated by main.go (algorithm, counts, ratio)
- `JSONOutput` struct — 9-field struct with exact JSON tags matching OUT-02 spec
- `FormatText(sentences []string) string` — newline-joined, pipe-safe, nil-safe
- `FormatJSON(sentences []string, meta SummaryMeta) (string, error)` — indented JSON via `json.MarshalIndent`
- `FormatMarkdown(sentences []string, meta SummaryMeta) string` — HTML comment header + `> ` prefixed sentences

### internal/formatter/formatter_test.go

8 unit tests covering all behavior items from the plan:
- `TestFormatText_MultiSentence`, `TestFormatText_Empty`
- `TestFormatJSON_ValidJSON`, `TestFormatJSON_RequiredFields`, `TestFormatJSON_NilSummaryBecomesArray`
- `TestFormatMarkdown_Header`, `TestFormatMarkdown_BlockquotePrefix`, `TestFormatMarkdown_BlankLineBetweenSentences`

## Verification

```
go test ./internal/formatter/ -v -count=1  -> 8 passed
go build ./...                             -> success
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed TestFormatMarkdown_BlockquotePrefix test to allow bare `>` separator**
- **Found during:** GREEN phase (test run)
- **Issue:** Test checked all non-empty lines start with `> ` (space), but blank blockquote separator is `>` without trailing space — causing one test failure
- **Fix:** Updated test condition to also allow `line == ">"` as valid (blank blockquote separator per markdown spec)
- **Files modified:** `internal/formatter/formatter_test.go`
- **Commit:** 765ddfe

## TDD Gate Compliance

- RED gate commit: `65a7056` — `test(03-02): add failing tests for formatter package`
- GREEN gate commit: `765ddfe` — `feat(03-02): implement internal/formatter package`
- REFACTOR: Not required — implementation is clean

## Known Stubs

None — all three format functions are fully implemented and return real output.

## Threat Flags

No new threat surface introduced. The formatter is a pure rendering layer with no I/O, no network access, and no file access. Input trust boundaries documented in plan threat model (T-03-04, T-03-05) are handled by `encoding/json` escaping and upstream sentence cap in main.go.

## Self-Check

- [x] `internal/formatter/formatter.go` exists
- [x] `internal/formatter/formatter_test.go` exists
- [x] RED commit `65a7056` exists
- [x] GREEN commit `765ddfe` exists
- [x] `go test ./internal/formatter/ -count=1` passes (8 tests)
- [x] `go build ./...` succeeds

## Self-Check: PASSED
