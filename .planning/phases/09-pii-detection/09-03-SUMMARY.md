---
phase: "09-pii-detection"
plan: "03"
subsystem: "main/hooks"
tags: ["pii", "security", "cli-flags", "hook", "sec-15"]
dependency_graph:
  requires: ["09-01"]
  provides: ["--detect-pii flag", "--sanitize-pii flag", "hook guard --detect-pii"]
  affects: ["cmd/tldt/main.go", "internal/installer/hooks/tldt-hook.sh", "cmd/tldt/main_test.go"]
tech_stack:
  added: []
  patterns: ["advisory detection flag (mirrors --detect-injection pattern)", "pre-summarization redaction (mirrors --sanitize pattern)", "TDD RED/GREEN"]
key_files:
  created: []
  modified:
    - "cmd/tldt/main.go"
    - "internal/installer/hooks/tldt-hook.sh"
    - "cmd/tldt/main_test.go"
decisions:
  - "detect-pii block runs after sanitize-pii on already-redacted text so double-reporting cannot occur when both flags are set"
  - "Hook guard addition is one-flag edit; existing grep 'WARNING' already captures pii-detect: WARNING lines without changes"
  - "Integration tests use existing run() helper (pre-built binary) for consistency with test infrastructure rather than exec.Command('go','run',...)"
metrics:
  duration: "~8 minutes"
  completed: "2026-05-03T17:21:00Z"
  tasks_completed: 2
  files_modified: 3
---

# Phase 9 Plan 03: PII Flag Wiring Summary

**One-liner:** `--detect-pii` advisory stderr scanner and `--sanitize-pii` pre-summarization redactor wired in main.go; hook guard extended to surface PII warnings alongside injection warnings.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Add failing PII flag integration tests | 39c5cef | cmd/tldt/main_test.go |
| 1 (GREEN) | Wire --detect-pii and --sanitize-pii flags to main.go | 2fc3751 | cmd/tldt/main.go |
| 2 | Extend hook output guard with --detect-pii | 5fef518 | internal/installer/hooks/tldt-hook.sh |

## What Was Built

**`cmd/tldt/main.go`** — three targeted edits:

1. Two flag declarations after `injectionThreshold`:
   - `detectPII := flag.Bool("detect-pii", ...)` — advisory PII/secret scanner
   - `sanitizePII := flag.Bool("sanitize-pii", ...)` — pre-summarization redactor

2. `--sanitize-pii` block inserted after the `--sanitize` block:
   - Calls `detector.SanitizePII(text)`, overwrites `text` with redacted form
   - Always reports `pii-detect: N redaction(s) applied` to stderr
   - Original text is not stored (T-09-03-04 mitigation)

3. `--detect-pii` block inserted after the `--sanitize-pii` block:
   - Calls `detector.DetectPII(text)` on the (possibly already-redacted) text
   - Reports `pii-detect: no findings` or per-finding `WARNING — [type] excerpt (line N)` to stderr
   - Never modifies text or blocks summarization
   - Excerpts truncated to 12 chars + "..." (T-09-03-01 mitigation)

4. `flag.Usage` help string updated to include `[--detect-pii] [--sanitize-pii]`

**`internal/installer/hooks/tldt-hook.sh`** — one-line edit:

- Guard invocation extended: `tldt --detect-injection --detect-pii --sentences 999`
- Existing `grep 'WARNING'` on the next line automatically captures `pii-detect: WARNING` lines (both injection and PII WARNING prefixes contain the word WARNING)

**`cmd/tldt/main_test.go`** — four integration tests added:

- `TestDetectPIIFlag`: email input produces WARNING on stderr, summary on stdout, no pii-detect on stdout
- `TestDetectPIIFlagCleanInput`: clean input produces "no findings" on stderr
- `TestSanitizePIIFlag`: email input redacted from stdout summary; stderr reports "redaction(s) applied"
- `TestSanitizePIIFlagStdoutOnly`: pii-detect output never appears on stdout

## TDD Gate Compliance

- RED gate: commit `39c5cef` — `test(09-03): add failing PII flag integration tests (RED)`
- GREEN gate: commit `2fc3751` — `feat(09-03): wire --detect-pii and --sanitize-pii flags to main.go (GREEN)`
- REFACTOR gate: not needed — implementation was clean on first pass

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Pattern] Adapted test style to match existing test infrastructure**
- **Found during:** Task 2 test writing
- **Issue:** Plan specified tests using `exec.Command("go", "run", "./cmd/tldt", ...)` which is inconsistent with the existing `run(t, stdin, args...)` helper in main_test.go that uses a pre-built `-cover` binary
- **Fix:** Tests written using `run(t, ...)` helper for consistency, coverage collection, and faster test execution
- **Files modified:** cmd/tldt/main_test.go
- **Commit:** 39c5cef

## Threat Model Coverage

| Threat ID | Status | Implementation |
|-----------|--------|----------------|
| T-09-03-01 | Mitigated | Excerpts truncated to 12 chars + "..." in DetectPII (implemented in 09-01); main.go uses Finding.Excerpt directly |
| T-09-03-02 | Accepted | Redaction count on stderr reveals PII presence, not content — intentional advisory behavior |
| T-09-03-03 | Accepted | Hook guard never modifies output; only adds WARNING context to additionalContext |
| T-09-03-04 | Mitigated | `text = redacted` overwrites original; DetectPII runs on already-redacted text when both flags set |

## Known Stubs

None — all flags are fully wired with real detector calls.

## Self-Check: PASSED

- `cmd/tldt/main.go` contains `detect-pii` (3 matches >= 2 required) and `sanitize-pii` (5 matches >= 2 required)
- `grep 'SanitizePII\|DetectPII' cmd/tldt/main.go` returns 2 matches
- `grep '--detect-pii' internal/installer/hooks/tldt-hook.sh` returns the guard line
- Commits `39c5cef`, `2fc3751`, `5fef518` present in git log
- `go build ./...` exits 0
- `go test ./...` exits 0 with 344 tests passing (up from 340 pre-plan)
