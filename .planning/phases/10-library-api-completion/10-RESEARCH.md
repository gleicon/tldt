# Phase 10: Library API Completion — Research

**Researched:** 2026-05-03
**Domain:** Go public API design, internal-to-public type wrapping, package boundary patterns
**Confidence:** HIGH

---

## Summary

Phase 10 is a pure Go extension task with no new dependencies and no external services. The work is surgical: extend `pkg/tldt/tldt.go` with three new things — a public `PIIFinding` type, two new top-level functions (`DetectPII`, `SanitizePII`), and two new fields each on `PipelineOptions` and `PipelineResult` — then add a PII stage inside `Pipeline`.

All underlying PII logic already exists and is tested (344 tests pass). `internal/detector` contains `DetectPII(text string) []Finding` and `SanitizePII(text string) (string, []Finding)` implemented in Phase 9. The public library layer is a thin, type-mapping wrapper: convert `detector.Finding` to `tldt.PIIFinding`, expose the two functions, and insert the PII stage into `Pipeline` between the Unicode sanitize step and the injection-detect step.

The sole design decision the planner must make explicit is the **field name mapping**: `detector.Finding.Sentence` carries the 1-based line number for PII findings, and the requirement specifies `PIIFinding.Line int`. The mapping is `Line = Finding.Sentence` for PII findings (confirmed from `detector.DetectPII` source: `Sentence: lineIdx + 1`).

**Primary recommendation:** Add `PIIFinding` type and two wrapper functions to `pkg/tldt/tldt.go`, extend `PipelineOptions`/`PipelineResult`, and wire the PII stage into `Pipeline`. All changes are confined to two files: `pkg/tldt/tldt.go` and `pkg/tldt/tldt_test.go`.

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LIB-01 | `pkg/tldt` exports a `PIIFinding` type with fields `Pattern string`, `Excerpt string`, `Line int` | `detector.Finding` has `Pattern`, `Excerpt`, `Sentence` (=Line) — direct field mapping |
| LIB-02 | `pkg/tldt.DetectPII(text string) []PIIFinding` — mirrors `detector.DetectPII` with public type | `detector.DetectPII` already implemented; wrapper converts `[]Finding` to `[]PIIFinding` |
| LIB-03 | `pkg/tldt.SanitizePII(text string) (string, []PIIFinding)` — redacts and returns findings | `detector.SanitizePII` already implemented; wrapper maps return types |
| LIB-04 | `PipelineOptions.DetectPII bool`, `PipelineOptions.SanitizePII bool`, `PipelineResult.PIIFindings []PIIFinding`; Pipeline PII stage between sanitize and detect | Pipeline function already has the two surrounding stages; inserting PII stage is additive |
</phase_requirements>

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| PIIFinding public type | pkg/tldt | — | Public API boundary; internal detector.Finding is not exported to library consumers |
| DetectPII wrapper | pkg/tldt | internal/detector | Library tier owns type conversion; detector owns pattern logic |
| SanitizePII wrapper | pkg/tldt | internal/detector | Same as above |
| Pipeline PII stage | pkg/tldt | internal/detector | Pipeline orchestration lives in pkg/tldt; pattern execution in detector |
| PII patterns and regex | internal/detector | — | Already implemented; not touched in this phase |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib only | go 1.26.2 | No new imports needed | All PII logic exists in internal/detector |

No new dependencies are introduced. All required packages (`internal/detector`, `internal/sanitizer`, `internal/summarizer`, `internal/fetcher`) are already imported in `pkg/tldt/tldt.go`.

**Installation:** No `go get` commands required.

---

## Architecture Patterns

### Recommended Project Structure

Only two files change:

```
pkg/tldt/
├── tldt.go        # Add PIIFinding type, DetectPII, SanitizePII, extend Pipeline
└── tldt_test.go   # Add tests for new API surface
```

No new files, no new packages.

### Pattern 1: Public Wrapper Type

**What:** Define a public struct that mirrors the relevant fields of an internal type. Internal type is NOT embedded — the public struct is a deliberate boundary.

**When to use:** When an internal type has more fields than the library consumer needs, or when the field name convention differs (e.g., `Sentence` vs `Line`).

**Example:**
```go
// Source: convention from existing pkg/tldt patterns (DetectResult wraps detector.Report)

// PIIFinding is a public wrapper for PII and secret detections.
// Consumers only need Pattern, Excerpt, and Line — not the internal scoring fields.
type PIIFinding struct {
    Pattern string // pattern name: "email", "api-key", "jwt", "credit-card"
    Excerpt string // first 12 chars of matched value + "..." for privacy
    Line    int    // 1-based line number in source text
}

// toPublicPIIFinding converts an internal detector.Finding to PIIFinding.
// Only valid for findings produced by detector.DetectPII (CategoryPII).
func toPublicPIIFinding(f detector.Finding) PIIFinding {
    return PIIFinding{
        Pattern: f.Pattern,
        Excerpt: f.Excerpt,
        Line:    f.Sentence, // detector.DetectPII sets Sentence = lineIdx+1 (1-based)
    }
}

// toPublicPIIFindings converts a slice.
func toPublicPIIFindings(findings []detector.Finding) []PIIFinding {
    out := make([]PIIFinding, len(findings))
    for i, f := range findings {
        out[i] = toPublicPIIFinding(f)
    }
    return out
}
```

### Pattern 2: Function Wrapper with Type Conversion

**What:** Delegate entirely to the internal function, convert the return type.

**Example:**
```go
// Source: mirrors existing Detect/Sanitize wrapper pattern in pkg/tldt/tldt.go

// DetectPII scans text for PII and secret patterns.
// Returns findings with type, excerpt, and 1-based line number.
// Text is not modified. Safe to call on untrusted input.
func DetectPII(text string) []PIIFinding {
    return toPublicPIIFindings(detector.DetectPII(text))
}

// SanitizePII replaces PII/secret matches with [REDACTED:<type>] placeholders.
// Returns the redacted string and the findings that triggered redaction.
func SanitizePII(text string) (string, []PIIFinding) {
    redacted, findings := detector.SanitizePII(text)
    return redacted, toPublicPIIFindings(findings)
}
```

### Pattern 3: PipelineOptions/PipelineResult Extension

**What:** Add fields to existing option/result structs. Zero-value means "disabled" — backward compatible.

**Example:**
```go
// PipelineOptions — add two bool fields (zero-value = disabled, backward compatible)
type PipelineOptions struct {
    Summarize  SummarizeOptions
    Detect     DetectOptions
    Sanitize   bool // existing
    DetectPII  bool // NEW: run PII detection stage
    SanitizePII bool // NEW: run PII redaction stage (implies detection)
}

// PipelineResult — add PIIFindings slice (nil when PII stage not enabled)
type PipelineResult struct {
    Summary     string
    TokensIn    int
    TokensOut   int
    Reduction   int
    Warnings    []string
    Redactions  int
    PIIFindings []PIIFinding // NEW: populated when DetectPII or SanitizePII is true
}
```

### Pattern 4: Pipeline Stage Insertion

**What:** Insert PII stage between existing Unicode sanitize stage and injection-detect stage, mirroring the constraint from LIB-04.

**Stage ordering (confirmed by ROADMAP.md LIB-04 description):**
1. Unicode sanitize (existing `opts.Sanitize`)
2. **PII stage** (new) — SanitizePII runs first, then DetectPII on remaining text
3. Injection detect (existing)
4. Summarize (existing)

**Key behavioral rule:** When `SanitizePII: true`, redaction runs and the redacted text continues through the pipeline. When `DetectPII: true` (without SanitizePII), detection runs but text is unchanged. When both are set, redaction runs first, then detection on the redacted text (findings from the redaction pass populate `PIIFindings`). This mirrors the CLI behavior in `cmd/tldt/main.go` lines 147-167.

**Example:**
```go
func Pipeline(text string, opts PipelineOptions) (PipelineResult, error) {
    var redactions int
    var piiFindings []PIIFinding

    // Step 1: Unicode sanitize (existing)
    if opts.Sanitize {
        inv := sanitizer.ReportInvisibles(text)
        redactions = len(inv)
        text = sanitizer.SanitizeAll(text)
    }

    // Step 2: PII stage (NEW — between sanitize and inject-detect)
    if opts.SanitizePII {
        redacted, findings := detector.SanitizePII(text)
        piiFindings = toPublicPIIFindings(findings)
        text = redacted
    } else if opts.DetectPII {
        findings := detector.DetectPII(text)
        piiFindings = toPublicPIIFindings(findings)
    }

    // Step 3: injection detect (existing)
    var warnings []string
    report := detector.Analyze(text)
    if report.Suspicious {
        warnings = append(warnings, "injection-detect: WARNING -- input flagged as suspicious")
    }

    // Step 4: summarize (existing)
    result, err := Summarize(text, opts.Summarize)
    if err != nil {
        return PipelineResult{}, err
    }

    return PipelineResult{
        Summary:     result.Summary,
        TokensIn:    result.TokensIn,
        TokensOut:   result.TokensOut,
        Reduction:   result.Reduction,
        Warnings:    warnings,
        Redactions:  redactions,
        PIIFindings: piiFindings,
    }, nil
}
```

### Anti-Patterns to Avoid

- **Embedding `detector.Finding` in `PIIFinding`:** Leaks internal type into public API. Always use a distinct struct.
- **Exporting `detector.Finding` directly:** The internal package should remain internal; `pkg/tldt` is the only public boundary.
- **Using pointer receivers on `PIIFinding`:** These are plain value types — use value semantics.
- **Returning `nil` vs empty slice inconsistency:** When no findings are present, returning `nil` is acceptable and idiomatic in Go; do not allocate an empty slice for zero results.
- **Calling `detector.DetectPII` twice in Pipeline when both SanitizePII and DetectPII are true:** The current CLI avoids this correctly; in the library version, when `SanitizePII` is set, the findings from the sanitize pass already represent all PII — no second scan is needed.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| PII regex patterns | New regex definitions | `detector.DetectPII` / `detector.SanitizePII` | Already implemented, tested in Phase 9 with 344 tests passing |
| Type conversion boilerplate | Manual field copies per call site | `toPublicPIIFinding` helper | DRY; one conversion path to test and maintain |
| PII stage in Pipeline | Inline pattern matching | Delegate to detector package | Single source of truth for PII logic |

**Key insight:** This phase is pure API surface work — the hard algorithmic work (PII patterns, redaction logic) is already done. Any custom re-implementation would diverge from the tested internal implementation.

---

## Common Pitfalls

### Pitfall 1: Field Name Mismatch (Line vs Sentence)
**What goes wrong:** Planner writes `PIIFinding{Line: f.Line}` — `detector.Finding` has no `Line` field; it uses `Sentence`.
**Why it happens:** The requirement says `Line int` but the internal struct uses `Sentence int` (because the detector uses `Sentence` for injection findings and repurposes it as line number for PII).
**How to avoid:** Always use `f.Sentence` when mapping to `PIIFinding.Line`. The conversion helper `toPublicPIIFinding` is the authoritative mapping point.
**Warning signs:** Compilation error: `f.Line undefined (type detector.Finding has no field or method Line)`.

### Pitfall 2: Forgetting to Handle the `SanitizePII` Implies Detection Case
**What goes wrong:** `Pipeline` with `SanitizePII: true` returns an empty `PIIFindings` slice because only the `DetectPII` branch populates it.
**Why it happens:** Developer writes separate branches without capturing findings from the sanitize pass.
**How to avoid:** In the `opts.SanitizePII` branch, capture the findings returned by `detector.SanitizePII` and assign to `piiFindings`. See Pattern 4 above.
**Warning signs:** `TestPipeline_SanitizePIIFindings` test fails with empty slice.

### Pitfall 3: Pipeline Stage Ordering
**What goes wrong:** PII stage runs after injection-detect, so `Warnings` may fire on PII-containing text before redaction.
**Why it happens:** Developer appends PII stage to the end of Pipeline for simplicity.
**How to avoid:** PII stage (step 2) must run before injection-detect (step 3) — this is specified in LIB-04 and mirrors how the CLI orders `--sanitize-pii` before `--detect-injection` (main.go lines 144–189).

### Pitfall 4: Breaking Existing Pipeline Tests
**What goes wrong:** Adding `PIIFindings` to `PipelineResult` causes existing tests that initialize `PipelineResult` by field name to fail to compile if the struct is used as a literal with positional fields anywhere.
**Why it happens:** Struct literal with positional fields breaks when new fields are added.
**How to avoid:** All existing tests use named field initialization (verified in `tldt_test.go`) — adding `PIIFindings []PIIFinding` is backward compatible. No existing test initializes `PipelineResult{}` with positional syntax.

---

## Code Examples

Verified patterns from the existing codebase:

### Existing Wrapper Pattern (SanitizeReport wrapping sanitizer types)
```go
// Source: pkg/tldt/tldt.go line 63-67
// SanitizeReport is the output metadata from Sanitize.
type SanitizeReport struct {
    RemovedCount int
    Invisibles   []sanitizer.InvisibleReport
}
```
Note: `SanitizeReport` does embed `sanitizer.InvisibleReport` directly. For `PIIFinding` we do NOT embed `detector.Finding` — we define a distinct struct (LIB-01 specifies exact fields Pattern/Excerpt/Line with no internal types).

### Existing PipelineResult (before Phase 10)
```go
// Source: pkg/tldt/tldt.go line 71-78
type PipelineResult struct {
    Summary    string
    TokensIn   int
    TokensOut  int
    Reduction  int
    Warnings   []string
    Redactions int
}
```
Phase 10 adds `PIIFindings []PIIFinding` as a seventh field.

### detector.DetectPII signature (internal — confirmed from source)
```go
// Source: internal/detector/detector.go line 426
func DetectPII(text string) []Finding
```

### detector.SanitizePII signature (internal — confirmed from source)
```go
// Source: internal/detector/detector.go line 457
func SanitizePII(text string) (string, []Finding)
```

### How detector sets Line in PII findings
```go
// Source: internal/detector/detector.go line 429-449
lines := strings.Split(text, "\n")
for lineIdx, line := range lines {
    // ...
    findings = append(findings, Finding{
        Category: CategoryPII,
        Sentence: lineIdx + 1, // 1-based line number
        // ...
    })
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CLI-only access to PII detection | Library API wrapping internal/detector | Phase 10 (this phase) | Any Go program can use PII detection without fork |
| `detector.Finding` with 6 fields | `PIIFinding` with 3 focused fields | Phase 10 (this phase) | Library consumer API is simpler and stable |
| `Pipeline` with 3 stages | `Pipeline` with 4 stages (PII inserted) | Phase 10 (this phase) | Library consumers get PII-aware pipeline in one call |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `SanitizeReport` embedding `sanitizer.InvisibleReport` is acceptable as prior art but NOT the pattern for `PIIFinding` — we use a distinct struct | Architecture Patterns | Low — spec is explicit: `PIIFinding` has `Pattern string`, `Excerpt string`, `Line int` only |
| A2 | Both `SanitizePII: true` and `DetectPII: true` simultaneously populates `PIIFindings` from the redaction pass (no second scan) | Pattern 4 / Pipeline stage | Low — this mirrors CLI behavior; confirmed from main.go lines 147-167 |

---

## Open Questions

1. **When both `SanitizePII: true` and `DetectPII: true` are set in Pipeline, should `PIIFindings` reflect pre-redaction findings or post-redaction findings?**
   - What we know: `detector.SanitizePII` returns findings from the unredacted text. The CLI runs sanitize first, then detect on the redacted text (which finds nothing). So for the library pipeline the natural choice is: use findings from the `SanitizePII` pass (pre-redaction).
   - What's unclear: Whether library consumers might expect detection to run independently even after redaction.
   - Recommendation: Use findings from `SanitizePII` pass only (avoids double-scan, mirrors CLI behavior). Document this in function comments.

2. **Should `toPublicPIIFinding` / `toPublicPIIFindings` be unexported helpers or inline conversions?**
   - Recommendation: Unexported helpers — keeps conversion logic in one testable place, consistent with existing pattern of unexported `applySummarizeDefaults`.

---

## Environment Availability

Step 2.6: SKIPPED — this phase is code-only changes to an existing Go module with no external dependencies. All required internal packages are already in the module.

---

## Security Domain

`security_enforcement` not set to false in config.json — included.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | detector package handles input; pkg/tldt wraps it |
| V6 Cryptography | no | — |

### Known Threat Patterns for pkg/tldt public API

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| `PIIFinding.Excerpt` contains sensitive data in logs | Information Disclosure | Excerpt is already truncated to 12 chars + "..." by `detector.DetectPII` — maintained in wrapper |
| Library consumer logs full `PIIFinding` slice | Information Disclosure | Document that `Excerpt` is intentionally truncated; Pattern name (not matched value) is safe to log |
| `SanitizePII` called but redacted text still stored | Information Disclosure | Out of scope for this library — document that library returns redacted text; storage is caller's responsibility |

---

## Sources

### Primary (HIGH confidence)
- `internal/detector/detector.go` — `DetectPII`, `SanitizePII`, `Finding`, `CategoryPII` implementations [VERIFIED: codebase read]
- `pkg/tldt/tldt.go` — current public API surface, `PipelineOptions`, `PipelineResult`, `Pipeline` [VERIFIED: codebase read]
- `pkg/tldt/tldt_test.go` — existing test patterns to follow [VERIFIED: codebase read]
- `cmd/tldt/main.go` lines 144-167 — PII stage ordering and behavioral contracts [VERIFIED: codebase read]

### Secondary (MEDIUM confidence)
- `.planning/REQUIREMENTS.md` LIB-01 through LIB-04 — canonical field names and function signatures [VERIFIED: project file]
- `.planning/ROADMAP.md` Phase 10 description — stage ordering constraint ("between Unicode sanitize and injection-detect stages") [VERIFIED: project file]

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all internal packages already imported and tested
- Architecture: HIGH — existing wrapper patterns are clear from current `pkg/tldt/tldt.go`; internal implementations verified by source read
- Pitfalls: HIGH — field name mismatch (`Sentence` vs `Line`) is a concrete, verified trap; stage ordering confirmed from CLI source

**Research date:** 2026-05-03
**Valid until:** Until internal/detector API changes (stable — no planned changes in Phase 10 scope)
