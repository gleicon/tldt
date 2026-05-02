---
phase: 07
plan: 03
title: CLI wiring and matrix integration
status: complete
completed: 2026-05-02
commit: 498cd95
---

# Summary 07-03: CLI wiring, MatrixSummarizer, README

## What was built

**MatrixSummarizer interface** in `internal/summarizer/lexrank.go`:
- `SummarizeWithMatrix` returns top-N sentences AND raw (pre-normalization) cosine similarity matrix
- `Summarize` now delegates to `SummarizeWithMatrix` — no duplicated logic
- Pre-normalization snapshot critical: row-normalized (stochastic) matrix gives mean ~1/n per row, making every sentence appear as outlier

**main.go wiring**:
- `--sanitize` path: strip invisibles → report count to stderr → pass cleaned text to summarizer
- `--detect-injection` path (pre-summarization): invisible char report + pattern/encoding analysis via `detector.Analyze`
- Summarization dispatch: type-assert `MatrixSummarizer`; if available use `SummarizeWithMatrix` to capture matrix; else fall back to `Summarize`
- `--detect-injection` outlier pass (post-summarization): call `detector.DetectOutliers` with tokenized sentences and captured matrix; report findings to stderr

**README** updated with sections: URL input, config file (~/.tldt.toml), Claude Code skill integration, prompt injection defense.

## Key decisions

- Outlier detection runs AFTER summarization so the LexRank matrix is available — this is the only ordering that works without circular imports or matrix re-computation
- `simMatrix` variable initialized to nil; only non-nil when `MatrixSummarizer` succeeds — safe fallback for textrank/graph/ensemble
