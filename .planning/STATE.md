---
gsd_state_version: 1.0
milestone: v2.1.0
milestone_name: Library SDK
status: planning
stopped_at: Phase 11 ready for execution - plan created
last_updated: "2026-05-06T19:00:00.000Z"
last_activity: 2026-05-06 -- Phase 11 planning COMPLETE (1 plan, 3 tasks); plan created for zero-internal-imports CLI refactor
progress:
  total_phases: 3
  completed_phases: 2
  total_plans: 5
  completed_plans: 4
  percent: 80
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-02)

**Core value:** Summarize long text (transcripts, articles, docs) into concise extractive summaries without consuming LLM tokens — pipe-safe CLI using LexRank/TextRank.
**Current focus:** Milestone v2.1.0 Library SDK — Phase 10 complete, Phase 11 planned

## Current Position

Phase: 11 — CLI Refactor *(PLANNED, READY FOR EXECUTION)*
Plan: 01 — Complete CLI refactor to zero internal imports
Status: Phase 11 plan created and ready for execution
Last activity: 2026-05-06 — Phase 11 planning complete; 1 plan with 3 tasks covering: pkg/tldt wrapper extensions, main.go atomic refactor, full test verification

### Roadmap Evolution
- Phase 9.1 Library Foundation inserted (URGENT) after Phase 9 — pre-condition for Phase 10: route CLI core ops through pkg/tldt before adding PII API

## Accumulated Context

### Decisions (carried from v1.0 + v2.0)

- Init: Implement LexRank + TextRank natively in Go; keep didasy/tldr as "graph" baseline
- Init: Drop all HTTP/Redis/config infrastructure; pure CLI only
- Init: stdout gets ONLY summary text when piped; stats always go to stderr
- 01-01: Exclude legacy src/ files with //go:build ignore rather than deleting them
- 01-02: Create new tldr.Bag per Summarize() call (Bag is not thread-safe)
- 01-02: resolveInput() precedence: stdin pipe > -f flag > positional args (Unix convention)
- 03: ensemble uses simple average of LexRank+TextRank score vectors (no normalization needed)
- 03: --rouge flag reads reference file, prints ROUGE-1/2/L scores to stderr only
- 04: --url branch is highest priority in resolveInputBytes (URL > stdin > file > positional)
- 04: fetcher uses readability.FromReader NOT FromURL — preserves custom http.Client + io.LimitReader
- 04: external test services (httpstat.us, Wikipedia) are unreliable; all URL tests use httptest.NewServer

### v2.0 Decisions (carried forward)

- v2.0: Clipboard auto-read deferred — --url covers remote input; clipboard adds complexity for marginal gain
- v2.0: REQUIREMENTS use INP/CFG/AI prefix scheme continuing from v1 CLI/SUM/TOK/OUT/TEST/PROJ
- 07: DetectOutliers takes precomputed simMatrix — LexRank exposes via MatrixSummarizer interface; no circular import
- 07: --detect-injection is advisory only — never blocks summarization or modifies stdout
- 07: golang.org/x/text upgraded to v0.36 for NFKC (was already transitive dep via go-readability)
- 07: NFKC does NOT collapse cross-script homoglyphs — documented limitation; UTS#39 needed for that
- 07: Outlier detection uses pre-normalization cosine matrix (not stochastic rows)

### Pending Todos

None.

### Blockers/Concerns

- Phase 6 human UAT still pending (non-blocking): test /tldt skill and auto-trigger hook in live Claude Code session

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| future | Clipboard auto-read (pbpaste/xclip) | Deferred | v2.0 Init |
| future | --url authentication / cookie support | Deferred | v2.0 |
| future | TOML validation/lint command | Deferred | v2.0 |
| future | MCP server mode | Deferred | v2.0 |

### Phase 9.1 Decisions (context gathered 2026-05-06)

- D-01: Comprehensive integration tests at pkg/tldt layer — edge cases, not just callable verification
- D-02: Add structured FetchResult type with metadata (StatusCode, ContentType, FinalURL) + sentinel errors
- D-03: Wrap all errors with context (`fmt.Errorf("tldt.FunctionName: %w", err)`) — tradeoff: callers can't use internal error types directly
- D-04: Big-bang refactor — replace all 4 internal imports in main.go in one atomic commit
- D-05: internal/config, internal/formatter, internal/installer stay as direct imports — not part of pkg/tldt scope

### Phase 9.1 Execution Decisions (2026-05-06)

- D-09.1-02: Approved partial refactor scope — route only fetcher.Fetch and detector.Analyze through pkg/tldt; defer summarizer/sanitizer/detector.PII routing to Phase 10 when PII APIs are available
- D-09.1-02a: Architecture gap identified — pkg/tldt lacks PII detection/sanitization exports; Phase 10 will add DetectPII/SanitizePII/PIIFinding to enable full internal/detector removal
- D-09.1-02b: internal/detector import kept for PII functions (SanitizePII, DetectPII, DetectOutliers, DefaultOutlierThreshold) — minimal surface area, will be removed in Phase 10

## Session Continuity

Last session: 2026-05-06T18:30:00.000Z
Stopped at: Phase 10 COMPLETE — Both waves executed successfully with full TDD cycle preservation
Resume file: None — Phase 10 done, pkg/tldt library API complete

## Phase 9.1 Summary

**Completed:** 2026-05-06  
**Plans:** 2/2 (9.1-01 Library Enhancement, 9.1-02 Partial CLI Refactor)  
**Tests:** 353 passing, no regressions  
**Key Result:** pkg/tldt is now a load-bearing public API — CLI routes Fetch and Detect operations through it, proving external embeddability before Phase 10 PII extensions

## Phase 10 Summary

**Completed:** 2026-05-06  
**Plans:** 2/2 (10-01 PIIFinding Type & Wrappers, 10-02 Pipeline PII Stage)  
**Waves:** 2 (Wave 1: 10-01, Wave 2: 10-02)  
**Tests:** 360 passing (7 new, 0 regressions)  
**TDD Cycle:** Full RED/GREEN preservation across both waves  
**Key Results:**
- `PIIFinding{Pattern, Excerpt, Line}` type exported
- `DetectPII(text) []PIIFinding` and `SanitizePII(text) (string, []PIIFinding)` exported
- PipelineOptions extended with `DetectPII` and `SanitizePII` bool fields
- PipelineResult extended with `PIIFindings []PIIFinding` field
- PII stage runs between Unicode sanitize and injection detect (order: sanitize → PII → inject-detect → summarize)
- Library consumers can now do PII-aware summarization: `tldt.Pipeline(text, PipelineOptions{DetectPII: true, Summarize: tldt.SummarizeOptions{Sentences: 3}})`

## Phase 11 Plan Summary

**Planned:** 2026-05-06  
**Plans:** 1 (11-01 CLI Refactor — Zero Internal Imports)  
**Wave:** 1 (single atomic refactor)  
**Requirements:** CLI-10, CLI-11  
**Scope:**

1. **Extend pkg/tldt** with remaining wrappers for main.go dependencies:
   - Sanitizer: `SanitizeAll(text) string`, `ReportInvisibles(text) []InvisibleReport`
   - Detector: `DefaultOutlierThreshold` constant, `DetectOutliers(sentences, simMatrix, threshold)` 
   - Summarizer: `NewSummarizer(algo)`, `TokenizeSentences(text)`, `EvalROUGE(system, reference)`
   - Types: `Summarizer`, `Explainer`, `MatrixSummarizer` interfaces, `ExplainInfo`, `SentenceScore`, `ROUGEScore`, `F1Score`

2. **Refactor cmd/tldt/main.go** atomically:
   - REMOVE imports: `internal/detector`, `internal/sanitizer`, `internal/summarizer`
   - KEEP per D-05: `internal/config`, `internal/formatter`, `internal/installer`
   - Replace all function calls with `tldt.X` equivalents
   - Preserve all CLI flags and behavior

3. **Verify**: All 360+ tests pass, zero behavioral regressions
