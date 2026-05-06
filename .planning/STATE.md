---
gsd_state_version: 1.0
milestone: v2.1.0
milestone_name: Library SDK
status: planning
stopped_at: Phase 9.1 inserted (Library Foundation)
last_updated: "2026-05-06T00:00:00.000Z"
last_activity: 2026-05-06 -- Phase 9.1 Library Foundation inserted before Phase 10; 3 phases total in v2.1.0; Phase 10 plans ready (2 plans)
progress:
  total_phases: 3
  completed_phases: 0
  total_plans: 2
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-02)

**Core value:** Summarize long text (transcripts, articles, docs) into concise extractive summaries without consuming LLM tokens — pipe-safe CLI using LexRank/TextRank.
**Current focus:** Milestone v1.2.0 OWASP Security Hardening — COMPLETE (all 2 phases, 7 plans)

## Current Position

Phase: 9.1 — Library Foundation *(INSERTED)*
Plan: Not planned yet
Status: Ready to plan
Last activity: 2026-05-06 — Phase 9.1 inserted before Phase 10; Phase 10 plans already ready (2 plans, 2 waves)

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

## Session Continuity

Last session: 2026-05-06T16:30:00.000Z
Stopped at: Phase 9.1 context gathered, ready for planning
Resume file: .planning/phases/9.1-library-foundation/9.1-CONTEXT.md
