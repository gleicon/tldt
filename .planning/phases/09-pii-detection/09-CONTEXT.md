# Phase 9: PII Detection + Output Guard + Docs - Context

**Gathered:** 2026-05-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 9 delivers three deliverables:

1. **PII detection + redaction** (`internal/detector/detector.go` extended): `DetectPII()` identifies email addresses, API key prefixes (Bearer/, sk-, AIza, AKIA), JWTs (three-segment base64url), and 13-16-digit credit card sequences. `SanitizePII()` replaces matches with `[REDACTED:<type>]` placeholders. Wired via `--detect-pii` and `--sanitize-pii` flags in `cmd/tldt/main.go`.
2. **Hook output guard extension** (`internal/installer/hooks/tldt-hook.sh` embedded template): Guard pass extended from `--detect-injection --sentences 999` to `--detect-injection --detect-pii --sentences 999`. PII WARNING lines in summary appear in `[Security warnings - summary]` section.
3. **README Security section** (`README.md`): Standalone `## Security` mini-section covering LLM04 (no ML weights), LLM08 (no vector store), LLM09 (extractive = no hallucination) with one-paragraph rationale each, plus a link to `docs/security.md` for full OWASP coverage.

</domain>

<decisions>
## Implementation Decisions

### PII Package Architecture

- **D-01:** PII detection lives in **`internal/detector`** — not a new package. Add `CategoryPII Category = "pii"` constant, `DetectPII(text string) []Finding`, and `SanitizePII(text string) (string, []Finding)` to the existing detector package. The shared `Finding` and `Category` types mean PII findings are interoperable with injection/encoding findings. Main.go uses one import for all detection.
- **D-02:** `SanitizePII` returns both the redacted string and a `[]Finding` slice so callers can report redaction count to stderr without a second detection pass.

### Hook Output Guard

- **D-03:** Hook output guard is extended to: `tldt --detect-injection --detect-pii --sentences 999`. Both injection and PII WARNING lines from the summary appear in `[Security warnings - summary]`. The existing `grep 'WARNING'` filter in the hook captures both prefixes without additional changes.
- **D-04:** Hook changes target the embedded template source (`internal/installer/hooks/tldt-hook.sh`), NOT any deployed copy. 09-03-PLAN.md depends on 09-01-PLAN.md completing (the `--detect-pii` flag must exist in the binary before the hook template references it).

### WARNING Output Format

- **D-05:** PII warnings use the prefix `pii-detect: WARNING —` (parallel to existing `injection-detect: WARNING —`). Format: `pii-detect: WARNING — [<type>] <excerpt> (line N)`. Types: `email`, `api-key`, `jwt`, `credit-card`. The `grep 'WARNING'` pattern in the hook catches both prefixes automatically. Users can grep `pii-detect` or `injection-detect` to filter by source.

### --sanitize-pii Semantics

- **D-06:** `--sanitize-pii` implies detection — running it without `--detect-pii` still performs redaction AND reports the redaction count to stderr. Redaction happens **before** summarization (input is redacted; the summary is of the redacted text). Format of count report: `pii-detect: N redaction(s) applied` on stderr.
- **D-07:** `--sanitize-pii` and `--sanitize` (Unicode) are independent flags and stack. Both can be combined: Unicode normalization + PII redaction applied in sequence before summarization.

### README Security Section

- **D-08:** `## Security` is a standalone mini-section in `README.md` with three named paragraphs (LLM04, LLM08, LLM09) and a link to `docs/security.md`. Each paragraph gives the architectural rationale in 2-3 sentences — enough for a reader to understand the immunity without visiting the full doc. Does NOT duplicate content from `docs/security.md` (LLM01/02/05/10 coverage lives there).
- **D-09:** Placement in README: after the `## Algorithms` or `## Flags` section, before `## Installation` — where security-conscious evaluators naturally look.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` §SEC-14, SEC-15, DOC-01 — 3 requirements for this phase
- `.planning/ROADMAP.md` §Phase 9 — Goal, success criteria, wave breakdown, cross-cutting constraints

### Detection (target package for 09-01)
- `internal/detector/detector.go` — Existing `Category`, `Finding`, `Report`, `DetectPatterns()`, `DetectEncoding()`, `Analyze()` — PII functions extend this file
- `internal/detector/detector_test.go` — Existing test patterns; new PII tests follow same structure
- `internal/sanitizer/sanitizer.go` — Reference for how SanitizeAll pattern works (PII redaction follows same before-summarization contract)

### Flag wiring (target file for 09-03)
- `cmd/tldt/main.go` — All flag wiring; `--detect-pii` and `--sanitize-pii` follow same pattern as `--sanitize` / `--detect-injection` (lines ~35-45 for flag defs, stderr reporting pattern throughout)

### Hook (target file for 09-03 wave 2)
- `internal/installer/hooks/tldt-hook.sh` — Embedded template source; guard section at bottom adds `--detect-pii` to existing guard command
- `internal/installer/embed.go` — go:embed wiring; confirms hook template path

### Documentation
- `README.md` — Target for `## Security` section (DOC-01)
- `docs/security.md` — Phase 8 OWASP reference; README ## Security links here; do NOT duplicate its LLM01/02/05/10 content
- `.planning/REQUIREMENTS.md` §DOC-01 — Exact requirement for README Security section

### Prior phase context
- `.planning/phases/08-network-hardening/08-CONTEXT.md` — Phase 8 decisions (hook architecture, additionalContext structure, WARNING grep pattern)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/detector/detector.go` `Category` type + `Finding` struct: PII findings drop in as `CategoryPII` constant + `Finding{Category: CategoryPII, ...}` — zero changes to consuming code needed.
- `internal/detector/detector.go` `Analyze()`: If this function aggregates all detection, consider whether PII should be included in `Analyze()` output or remain a separate call. Researcher to check Analyze() body.
- `internal/sanitizer/sanitizer.go` `SanitizeAll()` pattern: Returns transformed string; `SanitizePII()` follows same signature convention (input string → output string + report).

### Established Patterns
- **Advisory-only detection**: `--detect-pii` findings go to stderr, stdout = summary only. Never blocks summarization.
- **Pre-summarization redaction**: `--sanitize-pii` redacts input before the summarization pipeline; summary is of redacted text (mirrors `--sanitize` Unicode behavior).
- **No live network in tests**: All flag/integration tests use in-process test data — no external calls.
- **Stderr format**: `<prefix>: WARNING — [<type>] <detail> (line N)`. Grep-safe. Consistent with injection-detect prefix.
- **Flag.Visit override**: CLI flags override config defaults (established in phase 5); new flags follow same pattern.

### Integration Points
- `internal/detector/detector.go`: Add 4 regex patterns + `DetectPII()` + `SanitizePII()` + `CategoryPII` constant.
- `cmd/tldt/main.go`: Add `--detect-pii` and `--sanitize-pii` bool flags; wire before summarizer call (after `--sanitize` Unicode block, before `summarizer.Summarize()`).
- `internal/installer/hooks/tldt-hook.sh`: Guard command line: replace `--detect-injection` with `--detect-injection --detect-pii` (one-word change to existing guard invocation).
- `README.md`: Add `## Security` section — 3 paragraphs + link; no changes to any other README section.

</code_context>

<specifics>
## Specific Ideas

- `SanitizePII` redaction format is `[REDACTED:<type>]` exactly — e.g. `[REDACTED:credit-card]`, `[REDACTED:jwt]`, `[REDACTED:email]`, `[REDACTED:api-key]`.
- Stderr redaction count format: `pii-detect: N redaction(s) applied` (singular/plural).
- PII warning excerpt format mirrors injection: `pii-detect: WARNING — [email] alice@example.com (line 1)`. For long values (JWTs, credit cards), truncate to first 12 chars + `...` in the warning message (not in the redacted output).
- Hook guard change is minimal: one flag added to existing command. The `grep 'WARNING'` in `SUMMARY_WARNINGS=$(grep 'WARNING' "$GUARD_FILE" || true)` already catches `pii-detect: WARNING` without changes.
- README `## Security` placement: after flags table, before installation — consistent with where security posture belongs in a tools README.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 9-PII Detection + Output Guard + Docs*
*Context gathered: 2026-05-03*
