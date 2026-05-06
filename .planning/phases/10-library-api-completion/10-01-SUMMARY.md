---
phase: 10-library-api-completion
plan: 01
subsystem: library-api
status: completed
dependencies: []
requires:
  - LIB-01
  - LIB-02
  - LIB-03
provides:
  - PIIFinding type
  - DetectPII function
  - SanitizePII function
key-files:
  created: []
  modified:
    - pkg/tldt/tldt.go
    - pkg/tldt/tldt_test.go
decisions:
  - Field mapping uses f.Sentence for PIIFinding.Line (detector.Finding has no Line field)
  - Nil return for empty findings (not empty slice) for backward compatibility
  - Excerpts truncated to 12 chars + "..." at detector level, preserved in wrapper
tech-stack:
  added: []
  patterns:
    - Type wrapper pattern for public API exposure of internal types
    - Helper converter functions to map internal to public types
    - Delegation pattern for function wrappers
metrics:
  duration: "12m"
  completed_at: "2026-01-06T12:00:00Z"
  tasks_completed: 2
  test_count: 4
---

# Phase 10 Plan 01: PIIFinding Type and Wrapper Functions Summary

## One-Liner
Added public PIIFinding type with DetectPII and SanitizePII wrapper functions to pkg/tldt library API, enabling PII detection without internal package access.

## What Was Delivered

### PIIFinding Type
Exported struct with three fields:
- `Pattern string` - PII type name ("email", "api-key", "jwt", "credit-card")
- `Excerpt string` - Truncated preview (first 12 chars + "...")
- `Line int` - 1-based line number in source text

### Wrapper Functions
- `DetectPII(text string) []PIIFinding` - Scans text, returns findings, text unchanged
- `SanitizePII(text string) (string, []PIIFinding)` - Redacts PII with `[REDACTED:<type>]` placeholders, returns redacted text and findings

### Helper Functions
- `toPublicPIIFinding(f detector.Finding) PIIFinding` - Single finding converter
- `toPublicPIIFindings(findings []detector.Finding) []PIIFinding` - Slice converter, returns nil for empty input

## Deviations from Plan

None - plan executed exactly as written. TDD cycle preserved:
1. RED: 4 failing tests committed (undefined errors)
2. GREEN: Implementation added, all tests pass

## TDD Gate Compliance

| Gate | Status | Commit |
|------|--------|--------|
| RED (test) | ✅ | test(10-01): add failing tests |
| GREEN (feat) | ✅ | feat(10-01): implement PIIFinding |
| REFACTOR | N/A | No cleanup needed |

## Key Decisions

1. **Field mapping**: `detector.Finding.Sentence` → `PIIFinding.Line`
   - The internal Finding struct uses `Sentence` for the 1-based line number
   - Critical to use correct field name to avoid nil pointer or zero-value bugs

2. **Nil vs empty slice**: `toPublicPIIFindings` returns `nil` for empty input
   - Matches internal detector behavior
   - More idiomatic Go (nil slice is falsy in `len()` checks)

3. **Excerpt truncation**: Happens at detector level, wrapper preserves
   - No additional truncation in public API
   - Privacy-preserving by design

## Test Coverage

| Test | Purpose |
|------|---------|
| TestDetectPII_NoFindings | Clean text returns empty findings |
| TestDetectPII_EmailFound | Email detection populates Pattern, Line, Excerpt |
| TestSanitizePII_CleanText | Clean text unchanged, nil findings |
| TestSanitizePII_EmailRedacted | Email redacted to `[REDACTED:email]`, findings populated |

All 357 tests pass (4 new + 353 existing).

## Threat Model Implementation

| Threat ID | Disposition | Implementation |
|-----------|-------------|----------------|
| T-10-01 | mitigate | Excerpt truncation preserved from detector (12 chars + "...") |
| T-10-02 | accept | Caller responsible for logging/storage of redacted text |
| T-10-03 | accept | PII patterns tested in internal/detector (Phase 9, 344 tests) |

## Verification

```bash
$ go test ./pkg/tldt/... -v -run "TestDetectPII|TestSanitizePII"
=== RUN   TestDetectPII_NoFindings
--- PASS: TestDetectPII_NoFindings (0.00s)
=== RUN   TestDetectPII_EmailFound
--- PASS: TestDetectPII_EmailFound (0.00s)
=== RUN   TestSanitizePII_CleanText
--- PASS: TestSanitizePII_CleanText (0.00s)
=== RUN   TestSanitizePII_EmailRedacted
--- PASS: TestSanitizePII_EmailRedacted (0.00s)
PASS
```

## Self-Check: PASSED

- [x] PIIFinding type exists with correct field signatures
- [x] DetectPII function exported and functional
- [x] SanitizePII function exported and functional
- [x] 4 new tests pass
- [x] 357 total tests pass with no regressions
- [x] RED and GREEN commits present in git log

## Commits

| Hash | Type | Message |
|------|------|---------|
| `HEAD~1` | test | test(10-01): add failing tests for DetectPII and SanitizePII |
| `HEAD` | feat | feat(10-01): implement PIIFinding type, DetectPII, and SanitizePII |
