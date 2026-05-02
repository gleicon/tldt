---
phase: 03-polish
plan: 01
subsystem: cli
tags: [cli, input-validation, tty-detection, pipe-safety, sentence-cap]
dependency_graph:
  requires: []
  provides: [pipe-safe-output, binary-input-rejection, sentence-cap, tty-gated-stats]
  affects: [cmd/tldt/main.go]
tech_stack:
  added: [bytes, unicode/utf8]
  patterns: [TTY detection via os.ModeCharDevice, NUL byte binary detection, O(n^2) sentence cap]
key_files:
  created: []
  modified:
    - cmd/tldt/main.go
decisions:
  - "Use resolveInputBytes + validateInput pipeline instead of modifying resolveInput to preserve existing function for test compatibility"
  - "Gate token stats with isTTY := stdoutIsTerminal() so piped output is stats-free"
  - "Apply sentence cap at CLI layer (not inside summarizer) to keep Summarizer interface stable"
metrics:
  duration: ~10 min
  completed: 2026-05-02T10:50:42Z
  tasks_completed: 2
  tasks_total: 2
---

# Phase 3 Plan 01: CLI Hardening — Pipe Safety and Input Validation Summary

TTY-gated stats, binary input rejection, empty input silent exit, and O(n^2) sentence cap with --no-cap override implemented in cmd/tldt/main.go using only stdlib.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add stdoutIsTerminal, validateInput, applySentenceCap helpers | cfdf43c | cmd/tldt/main.go |
| 2 | Wire TTY gate, input validation, sentence cap, --no-cap into main() | ef3f5a4 | cmd/tldt/main.go |

## What Was Built

### cmd/tldt/main.go

Five targeted changes applied to main() and three new helper functions added:

1. **`stdoutIsTerminal()`** — Checks `os.Stdout.Stat()` with `os.ModeCharDevice` to detect TTY vs pipe/redirect.

2. **`validateInput(data []byte) (string, bool, error)`** — Rejects binary input (NUL byte via `bytes.IndexByte`, invalid UTF-8 via `utf8.Valid`), returns `isEmpty=true` for whitespace-only input.

3. **`applySentenceCap(text string, cap int) string`** — Caps input to N sentences using `summarizer.TokenizeSentences()`, rejoining with space to preserve tokenizer-recognizable sentence boundaries.

4. **`resolveInputBytes()`** — Mirrors `resolveInput()` but returns `[]byte` for pre-validation before string conversion.

5. **`--no-cap` flag** — Added to bypass the 2000-sentence cap for users who opt into O(n^2) processing.

6. **TTY gate on stats** — `fmt.Fprintf(os.Stderr, "~%s -> ~%s tokens (%d%% reduction)\n", ...)` now only fires when `isTTY == true`.

7. **Stats format fix** — Changed from `"tokens: %s -> %s (%d%% reduction)"` to `"~%s -> ~%s tokens (%d%% reduction)"` per CLI-06 / ROADMAP Phase 3 success criteria.

## Verification Results

All plan success criteria verified:

- `go build ./...` and `go test ./... -count=1` pass (49 tests)
- Empty/whitespace stdin: exits 0, zero bytes on stdout and stderr
- Binary stdin (NUL byte): exits 1, error message on stderr, nothing on stdout
- Piped stdout: token stats absent from all output streams
- `--no-cap` flag accepted without error and visible in `--help`
- Input validated before summarizer call; sentence cap applied when `!noCap`

## Deviations from Plan

None — plan executed exactly as written. All helpers and wiring match the plan specification.

## Known Stubs

None — all functionality is fully wired.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced beyond what the plan's threat model covers. The `validateInput()` function implements T-03-01 mitigation (binary input rejection) and `applySentenceCap()` implements T-03-02 mitigation (DoS via O(n^2)). Both threat mitigations are now active in the execution path.

## Self-Check: PASSED

- cmd/tldt/main.go exists with all required functions: FOUND
- Commit cfdf43c exists: FOUND
- Commit ef3f5a4 exists: FOUND
- go build ./... passes: PASSED
- go test ./... -count=1 passes: 49 tests PASSED
- echo "" | tldt outputs 0 bytes: VERIFIED
- printf '\x00binary' | tldt exits 1: VERIFIED
- Piped stdout stats count = 0: VERIFIED
