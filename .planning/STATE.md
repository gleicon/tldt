---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Extensions
status: complete
stopped_at: milestone complete
last_updated: "2026-05-02T22:30:00.000Z"
last_activity: 2026-05-02 -- Phase 7 (Injection Defense) complete; all 9 SEC requirements verified; 292 tests
progress:
  total_phases: 4
  completed_phases: 4
  total_plans: 11
  completed_plans: 11
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-02)

**Core value:** Summarize long text (transcripts, articles, docs) into concise extractive summaries without consuming LLM tokens — pipe-safe CLI using LexRank/TextRank.
**Current focus:** Milestone v2.0 Extensions — COMPLETE

## Current Position

Phase: 7 — Injection Defense — COMPLETE
Status: All v2.0 phases complete (4/4, 11/11 plans)
Last activity: 2026-05-02 — Phase 7 complete (sanitizer, detector, MatrixSummarizer wiring, README, plans, verification)

Progress: [██████████] 100%

## Accumulated Context

### Decisions (carried from v1.0)

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

### v2.0 Decisions

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

## Deferred Items (updated)

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| v2 | Clipboard auto-read (pbpaste/xclip) | Deferred (not in v2.0) | Init |
| v3+ | --url authentication / cookie support | Deferred | v2.0 |
| v3+ | TOML validation/lint command | Deferred | v2.0 |
| v3+ | MCP server mode | Deferred | v2.0 |
| v3+ | UTS#39 confusables database (cross-script homoglyphs) | Deferred | Phase 7 |

## Session Continuity

Last session: 2026-05-02T22:30:00.000Z
Stopped at: Milestone v2.0 complete — all 4 phases, 11 plans, 292 tests
Resume file: None
