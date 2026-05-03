---
phase: 08-network-hardening
plan: "04"
subsystem: pkg/tldt
tags: [library, embeddable, api, ssrf, security]
dependency_graph:
  requires:
    - fetcher.ErrSSRFBlocked (from 08-01)
    - fetcher.ErrRedirectLimit (from 08-01)
    - internal/summarizer.New + Summarizer interface
    - internal/detector.Analyze + Report + Finding
    - internal/sanitizer.SanitizeAll + ReportInvisibles + InvisibleReport
    - internal/fetcher.Fetch
  provides:
    - pkg/tldt.Summarize
    - pkg/tldt.Detect
    - pkg/tldt.Sanitize
    - pkg/tldt.Fetch
    - pkg/tldt.Pipeline
    - pkg/tldt.ErrSSRFBlocked (re-export)
    - pkg/tldt.ErrRedirectLimit (re-export)
  affects:
    - pkg/tldt/tldt.go
    - pkg/tldt/tldt_test.go
tech_stack:
  added: []
  patterns:
    - "Plain struct options (not functional options) — consistent with codebase convention"
    - "Delegation pattern: pkg/tldt wraps internal/ with no duplicated business logic"
    - "Re-export sentinel errors for caller errors.Is() checking"
    - "Zero-value option defaults applied via applySummarizeDefaults helper"
    - "Pipeline order: sanitize -> detect -> summarize matches cmd/tldt/main.go"
key_files:
  created:
    - pkg/tldt/tldt.go
    - pkg/tldt/tldt_test.go
decisions:
  - "Plain struct options over functional options — matches existing codebase pattern"
  - "Detection is advisory-only in Pipeline — always returns summary; callers check Warnings field (SEC-07)"
  - "Re-export ErrSSRFBlocked and ErrRedirectLimit as pkg-level vars (not type aliases) so callers can use errors.Is()"
  - "SanitizeReport.Invisibles exposes raw []sanitizer.InvisibleReport for callers who need details"
metrics:
  duration: "~2 minutes"
  completed: "2026-05-03T12:13:54Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 0
  tests_added: 16
  total_tests: 332
---

# Phase 08 Plan 04: Embeddable Go Library API Summary

Public Go library `pkg/tldt` wrapping all internal packages behind five clean, stateless exported functions (Summarize, Detect, Sanitize, Fetch, Pipeline) with plain struct options, re-exported sentinel errors, and 16 integration tests.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create pkg/tldt/tldt.go with exported API | 28032dd | pkg/tldt/tldt.go |
| 2 | Create pkg/tldt/tldt_test.go with integration tests | e938639 | pkg/tldt/tldt_test.go |

## What Was Built

**Task 1 -- pkg/tldt/tldt.go:**
- Package declaration `package tldt` at `pkg/tldt/tldt.go`
- Option types: `SummarizeOptions`, `DetectOptions`, `FetchOptions`, `PipelineOptions` (plain structs per D-12)
- Result types: `Result`, `DetectResult`, `SanitizeReport`, `PipelineResult`
- Re-exported sentinel errors: `ErrSSRFBlocked = fetcher.ErrSSRFBlocked`, `ErrRedirectLimit = fetcher.ErrRedirectLimit`
- `Summarize(text, SummarizeOptions) (Result, error)` -- delegates to summarizer.New(algo).Summarize()
- `Detect(text, DetectOptions) (DetectResult, error)` -- delegates to detector.Analyze()
- `Sanitize(text) (string, SanitizeReport, error)` -- delegates to sanitizer.ReportInvisibles + SanitizeAll
- `Fetch(url, FetchOptions) (string, error)` -- delegates to fetcher.Fetch() with SSRF protection from 08-01
- `Pipeline(text, PipelineOptions) (PipelineResult, error)` -- sanitize -> detect -> summarize flow
- Zero-value defaults: Algorithm defaults to "lexrank", Sentences to 5, Timeout to 30s, MaxBytes to 5MB

**Task 2 -- pkg/tldt/tldt_test.go:**
- 16 integration-style tests exercising the full public API
- Summarize: basic, zero-value defaults, all 4 algorithms (lexrank/textrank/graph/ensemble), invalid algorithm error
- Detect: clean text (Suspicious=false), injection text (Suspicious=true, Warnings non-empty)
- Sanitize: clean text unchanged, zero-width space removal confirmed
- Pipeline: full flow with sanitize, with injection content (advisory-only), without sanitize (Redactions=0)
- Sentinel errors: ErrSSRFBlocked and ErrRedirectLimit non-nil verification

## Deviations from Plan

None -- plan executed exactly as written. All acceptance criteria met on first attempt.

## Known Stubs

None -- all five exported functions are fully wired to internal packages. No placeholders.

## Threat Surface Scan

No new network endpoints introduced. `pkg/tldt.Fetch()` delegates entirely to `internal/fetcher.Fetch()` which has SSRF blocking from 08-01 (T-08-10 mitigated). T-08-09 addressed: pkg/tldt/ imports only from internal/; no import cycles with cmd/ (enforced by Go compiler internal/ package protection). T-08-11 accepted: Pipeline detection is advisory-only per SEC-07.

## Self-Check: PASSED

- `pkg/tldt/tldt.go` -- exists, `go build ./pkg/tldt/...` succeeds
- `pkg/tldt/tldt_test.go` -- exists, 16/16 tests pass
- Commit 28032dd -- Task 1 in git log
- Commit e938639 -- Task 2 in git log
- `go test ./...` -- 332 tests passing (316 existing + 16 new), 0 failures
- All 5 exported functions present: Summarize, Detect, Sanitize, Fetch, Pipeline
- ErrSSRFBlocked and ErrRedirectLimit re-exported
- No import from cmd/ (no import cycle)
