---
phase: 08-network-hardening
plan: "02"
subsystem: hook
tags: [security, hook-defense, injection-detection, owasp-llm01, sec-13, sec-16]
dependency_graph:
  requires:
    - fetcher.ErrSSRFBlocked (08-01)
  provides:
    - hook.security-flags (--sanitize --detect-injection --verbose)
    - hook.output-guard (--detect-injection --sentences 999 on summary)
    - hook.labeled-additionalContext
  affects:
    - internal/installer/hooks/tldt-hook.sh
tech_stack:
  added: []
  patterns:
    - "stderr splitting: grep 'WARNING' / grep -v 'WARNING' on single STDERR_FILE"
    - "output guard: mktemp + tldt --detect-injection --sentences 999 + GUARD_FILE"
    - "labeled additionalContext sections with conditional emission"
key_files:
  created: []
  modified:
    - internal/installer/hooks/tldt-hook.sh
decisions:
  - "Use unanchored grep 'WARNING' (not grep '^WARNING') — tldt emits 'injection-detect: WARNING —' which starts with 'injection-detect:', not 'WARNING'"
  - "Output guard uses --sentences 999 to prevent re-summarization; stdout discarded; only stderr WARNING lines extracted"
  - "All sections are advisory-only — summary always emitted even when warnings present"
  - "GUARD_FILE mktemp pattern mirrors STDERR_FILE pattern for consistency"
metrics:
  duration: "~5 minutes"
  completed: "2026-05-03T12:13:07Z"
  tasks_completed: 1
  tasks_total: 1
  files_modified: 1
  tests_added: 0
  total_tests: 316
---

# Phase 08 Plan 02: Hook Defense with Injection Detection and Output Guard Summary

Hook updated to invoke tldt with `--sanitize --detect-injection --verbose` by default, split stderr into WARNING lines and token stats, and re-check the summary via an output guard before emitting to Claude's additionalContext.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Update hook with security flags, stderr splitting, and output guard | 6f2de99 | internal/installer/hooks/tldt-hook.sh |

## What Was Built

**Task 1 — tldt-hook.sh changes:**

- Replaced `tldt --verbose` with `tldt --sanitize --detect-injection --verbose` (SEC-13)
- Replaced single `STATS_FILE` pattern with `STDERR_FILE` capturing all stderr
- Added stderr splitting: `WARNINGS=$(grep 'WARNING' "$STDERR_FILE" || true)` and `SAVINGS=$(grep -v 'WARNING' "$STDERR_FILE" || true)`
- Added output guard: creates `GUARD_FILE`, runs `tldt --detect-injection --sentences 999` on the summary, captures `SUMMARY_WARNINGS` from stderr (SEC-16)
- Replaced single `REPLACEMENT` string with labeled section builder:
  - `[Token savings]` — always present (token stats from --verbose)
  - `[Security warnings - input]` — conditional, only when WARNINGS non-empty
  - `[Security warnings - summary]` — conditional, only when SUMMARY_WARNINGS non-empty
  - `[Summary]` — always present (the actual extracted summary)
- Python3 JSON output block unchanged — encodes `$REPLACEMENT` which now has labeled structure

## Deviations from Plan

None — plan executed exactly as written. The unanchored `grep 'WARNING'` pattern was already specified in the plan (Pitfall 4 from RESEARCH.md was pre-identified and documented in the plan itself).

## Known Stubs

None — all security flags, stderr splitting, output guard, and labeled sections are fully wired.

## Threat Surface Scan

No new network endpoints or auth paths introduced. All changes are within the hook script that was already handling user prompts. The threat model items T-08-05 (Tampering via hook prompt input) and T-08-06 (Tampering via hook summary output) are both mitigated by this plan:

- T-08-05: mitigated via `--sanitize --detect-injection --verbose` on initial prompt
- T-08-06: mitigated via output guard `--detect-injection --sentences 999` on summary

## Self-Check: PASSED

- `internal/installer/hooks/tldt-hook.sh` — exists, bash syntax valid
- `bash -n internal/installer/hooks/tldt-hook.sh` — exits 0
- `go build ./...` — succeeds (embed.go still finds the hook file via //go:embed)
- Commit 6f2de99 — exists in git log
- `--sanitize --detect-injection --verbose` present: 1 occurrence
- `--detect-injection --sentences 999` present: 1 occurrence
- `grep 'WARNING'` (unanchored, excluding grep -v): 2 occurrences (STDERR_FILE + GUARD_FILE)
- `grep -v 'WARNING'` present: 1 occurrence (SAVINGS extraction)
- `GUARD_FILE` references: 4 (mktemp + pipe + grep + rm)
- `[Token savings]`, `[Security warnings - input]`, `[Security warnings - summary]`, `[Summary]`: each 1 occurrence
- No anchored `grep '^WARNING'` found
