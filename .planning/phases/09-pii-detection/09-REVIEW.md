---
phase: 09-pii-detection
reviewed: 2026-05-03T00:00:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - internal/detector/detector.go
  - internal/detector/detector_test.go
  - cmd/tldt/main.go
  - cmd/tldt/main_test.go
  - internal/installer/hooks/tldt-hook.sh
findings:
  critical: 1
  warning: 5
  info: 3
  total: 9
status: issues_found
---

# Phase 09: Code Review Report

**Reviewed:** 2026-05-03
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

This phase introduces PII detection and sanitization (`DetectPII`, `SanitizePII`, `CategoryPII`, `piiPatterns`) wired to two new CLI flags (`--detect-pii`, `--sanitize-pii`) and integrated into the hook script output-guard. The core patterns and CLI wiring are structurally sound. However, one critical test defect makes a negative assertion permanently vacuous, three warnings cover correctness or UX traps in the implementation, and three info items flag style inconsistencies.

---

## Critical Issues

### CR-01: Negative test for `sk-short` API key is a vacuous no-op — real match would not be caught

**File:** `internal/detector/detector_test.go:583-588`

**Issue:** The test that asserts short `sk-*` tokens are NOT matched contains an always-true tautology that makes the assertion fire on every finding rather than filtering to the intended case:

```go
for _, f := range findings {
    if f.Pattern == "api-key" && strings.Contains("sk-short", "sk-short") {
        t.Errorf(...)
    }
}
```

`strings.Contains("sk-short", "sk-short")` is a string literal compared against itself — it is always `true`. The condition collapses to `f.Pattern == "api-key"`. Because `"sk-short"` is only 8 chars after the prefix (regex requires 20+), it does not currently match and `findings` is empty, so the loop never executes and the test passes vacuously. If the regex were relaxed to a lower minimum length, this test would still pass vacuously because the loop body fires correctly — but the intent of the `strings.Contains` guard is completely lost. Any reviewer relying on this test to validate the length guard is misled.

**Fix:** Remove the tautological `strings.Contains` and assert directly that findings is empty:

```go
findings := DetectPII("sk-short")
if len(findings) > 0 {
    t.Errorf("DetectPII(sk-short): expected no api-key match for short token, got %v", findings)
}
```

---

## Warnings

### WR-01: `SanitizePII` applies sequential regex replacement — second pass runs on already-redacted text

**File:** `internal/detector/detector.go:457-468`

**Issue:** `SanitizePII` iterates `piiPatterns` and applies each replacement in sequence against the accumulating `redacted` string. Because earlier replacements produce `[REDACTED:api-key]`, `[REDACTED:jwt]`, etc., later patterns run against those strings. The email regex (`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`) requires `@` so it cannot match replacement text. The JWT regex (`[A-Za-z0-9_\-]{10,}\.`) requires a literal `.` after 10+ word chars — `[REDACTED:api-key]` starts with `[` which is not in `[A-Za-z0-9_\-]`, so no match. The credit-card regex requires `\d`. All current patterns appear safe, but the architecture is fragile: any future pattern added to `piiPatterns` that can match `[REDACTED:X]` text would cause a double-replacement, corrupting the output without any error signal.

Additionally, the `findings` slice returned was collected from the original `text` by `DetectPII`, but the replacements are applied to `redacted` in a separate loop. If a pattern's replacement is itself matched by a later pattern, the returned `findings` count underreports redactions actually applied.

**Fix:** Apply all replacements in a single pass using a combined regex with a dispatch table, or collect replacement positions from `findings` and substitute by offset range to avoid cascade. At minimum, add a test that verifies `SanitizePII` output contains no `[REDACTED:X]` strings that are themselves matched by any `piiPattern`:

```go
// After SanitizePII, verify no replacement token is re-matched
for _, p := range piiPatterns {
    if p.re.MatchString(redacted) {
        // cascade detected
    }
}
```

---

### WR-02: `--detect-pii` silently reports "no findings" when used with `--sanitize-pii`

**File:** `cmd/tldt/main.go:147-167`

**Issue:** When both `--sanitize-pii` and `--detect-pii` are set, `--sanitize-pii` runs first (line 147) and replaces all PII in `text`. Then `--detect-pii` runs on the already-redacted string (line 157) and reports "no findings" to stderr. Users who pass both flags expecting to see a detection report before redaction will only see `pii-detect: no findings`, which is silently wrong — PII was present and was redacted, but the detection output says nothing was found.

The comment on line 155 acknowledges this: "detection post-redaction is safe." It is safe, but it is misleading UX that could cause an operator to conclude their input was clean when it was not.

**Fix:** Run detection before sanitization, or capture findings from `SanitizePII` (which already returns them) and use those findings for the `--detect-pii` report. Simplest fix:

```go
// Collect PII findings before any redaction so --detect-pii always reports the original state.
var piiFindingsForReport []detector.Finding
if *detectPII || *sanitizePII {
    piiFindingsForReport = detector.DetectPII(text)
}

if *sanitizePII {
    redacted, _ := detector.SanitizePII(text)
    fmt.Fprintf(os.Stderr, "pii-detect: %d redaction(s) applied\n", len(piiFindingsForReport))
    text = redacted
}

if *detectPII {
    // use piiFindingsForReport — always from original text
    ...
}
```

---

### WR-03: `Finding.Sentence` field semantics are overloaded — PII uses it as a 1-based line number

**File:** `internal/detector/detector.go:429-449`

**Issue:** The `Finding` struct documents `Sentence` as "index into sentence list; -1 if not sentence-scoped." Every other detector that uses `Sentence` sets it to a 0-based sentence index or -1. `DetectPII` sets `Sentence: lineIdx + 1` — a 1-based line number from `strings.Split(text, "\n")`. This is a different semantic: it is a line number, not a sentence index.

Callers iterating `report.Findings` and using `f.Sentence` to index `sentences[]` from a `TokenizeSentences` call will get an off-by-one error or an out-of-bounds panic. The stderr output in `main.go` uses it as a display value (`"line %d"`, line 164) which is correctly labeled, but any programmatic consumer of the `Finding` type will be surprised by the mixed semantics.

**Fix:** Either add a `Line int` field to `Finding` for line-scoped detectors, or document explicitly that PII findings use `Sentence` as a 1-based line number and always set `Sentence = -1` for PII findings (using the `Offset` field for position instead). Whichever approach is chosen, the struct-level doc comment must be updated.

---

### WR-04: Hook script swallows all tldt errors silently with `|| true`

**File:** `internal/installer/hooks/tldt-hook.sh:47`

**Issue:**

```bash
SUMMARY=$(printf '%s' "$PROMPT" | tldt --sanitize --detect-injection --verbose 2>"$STDERR_FILE" || true)
```

The `|| true` makes the subshell always succeed. If tldt exits 1 (binary input, OOM, invalid input), `$SUMMARY` is empty and the script hits the guard at line 53 (`if [ -z "$SUMMARY" ]; then exit 0; fi`) and passes through the full unmodified prompt silently. The user gets no indication that summarization failed. This is a design tradeoff (documented as "D-05 spirit"), but the failure mode is invisible: the hook appears to succeed while doing nothing.

**Fix:** At minimum, capture the tldt exit code and emit a warning to stderr when it is non-zero, before falling through:

```bash
SUMMARY=$(printf '%s' "$PROMPT" | tldt --sanitize --detect-injection --verbose 2>"$STDERR_FILE") || {
    printf '%s\n' "tldt-hook: summarization failed (exit $?), passing through" >&2
}
```

---

### WR-05: Credit card regex produces false positives on long digit strings (phone numbers, ISBNs, tracking numbers)

**File:** `internal/detector/detector.go:418-420`

**Issue:** The credit card regex `\b(?:\d[ \-]?){12,15}\d\b` matches any 13–16 consecutive digit sequence with optional single spaces or hyphens between digits. This will match:

- US phone numbers with area code in context: `+1 800 555-1234` (if adjacent to another number)
- ISBN-13 numbers: `978-0-306-40615-7` (13 digits total)
- Long tracking numbers (FedEx: 12–22 digits, UPS: 18 digits)
- Unix timestamps concatenated: `16781234001234` (14 digits)

The Luhn algorithm would eliminate virtually all false positives from random digit strings (only ~10% of random numbers pass Luhn), but it is not applied here.

There are no false-positive guard tests for credit card numbers (the test at line 631 only checks a 5-digit number — nowhere near the 13-digit match threshold).

**Fix:** Apply the Luhn check before emitting a credit card finding. If Luhn is too expensive for a pattern-matching library, at minimum add explicit false-positive test cases for ISBNs, phone numbers, and tracking numbers.

---

## Info

### IN-01: `CategoryPII` declared outside the existing `const` block — style inconsistency

**File:** `internal/detector/detector.go:377-378`

**Issue:** `CategoryPattern`, `CategoryEncoding`, and `CategoryOutlier` are declared together in one `const (...)` block (lines 27-30). `CategoryPII` is declared as a separate `const` on line 378, outside that block. This is not a bug, but it breaks the pattern established by the first three categories and makes it harder to find all category constants.

**Fix:** Move `CategoryPII` into the existing `const` block alongside the other three categories.

---

### IN-02: `DetectPII` excerpt truncation threshold (12 chars) is inconsistent with all other detectors (80 chars)

**File:** `internal/detector/detector.go:435-437`

**Issue:** All other detectors truncate excerpts at 80 characters. `DetectPII` truncates at 12 characters:

```go
if len(excerpt) > 12 {
    excerpt = excerpt[:12] + "..."
}
```

The 12-char limit is intentional (to avoid logging sensitive values) and explained in the doc comment, but it is not tested directly. A future refactor of the excerpt logic could accidentally increase this limit and begin logging PII to stderr.

**Fix:** Add a unit test that verifies `Finding.Excerpt` from `DetectPII` is never longer than 15 chars (12 + 3 for `"..."`). This protects the privacy-preserving truncation from regression.

---

### IN-03: README Security section references `docs/security.md` which does not exist

**File:** `README.md:266`

**Issue:**

```
For full OWASP LLM Top 10 2025 coverage ... see [docs/security.md](docs/security.md).
```

The file `docs/security.md` does not exist in the repository. This is a dead link in the README.

**Fix:** Either create `docs/security.md` with the referenced content, or remove the reference and inline the content in the README Security section.

---

_Reviewed: 2026-05-03_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
