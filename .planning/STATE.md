---
gsd_state_version: 1.0
milestone: v2.1.0
milestone_name: Library SDK
status: planning
stopped_at: Phase 9.1 complete - ready to execute Phase 10
last_updated: "2026-05-06T17:55:00.000Z"
last_activity: 2026-05-06 -- Phase 9.1 Library Foundation COMPLETE (2 plans executed); pkg/tldt is load-bearing; ready for Phase 10 PII API extension
progress:
  total_phases: 3
  completed_phases: 1
  total_plans: 2
  completed_plans: 2
  percent: 33
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-02)

**Core value:** Summarize long text (transcripts, articles, docs) into concise extractive summaries without consuming LLM tokens — pipe-safe CLI using LexRank/TextRank.
**Current focus:** Milestone v1.2.0 OWASP Security Hardening — COMPLETE (all 2 phases, 7 plans)

## Current Position

Phase: 9.1 — Library Foundation *(COMPLETE)*
Plan: 02 — Partial CLI Refactor *(COMPLETE)*
Status: Completed - Phase 9.1 Library Foundation DONE
Last activity: 2026-05-06 — Phase 9.1 complete (2/2 plans); fetcher.Fetch and detector.Analyze routed through pkg/tldt; 353 tests pass; Phase 10 ready to execute

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

Last session: 2026-05-06T17:55:00.000Z
Stopped at: Phase 9.1 COMPLETE — Both plans executed successfully
Resume file: None — Phase 9.1 done, ready for Phase 10 execution

## Phase 9.1 Summary

**Completed:** 2026-05-06  
**Plans:** 2/2 (9.1-01 Library Enhancement, 9.1-02 Partial CLI Refactor)  
**Tests:** 353 passing, no regressions  
**Key Result:** pkg/tldt is now a load-bearing public API — CLI routes Fetch and Detect operations through it, proving external embeddability before Phase 10 PII extensions
