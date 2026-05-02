---
phase: 07
plan: 02
title: Detector package
status: complete
completed: 2026-05-02
commit: 6e2793f
---

# Summary 07-02: internal/detector package

## What was built

`internal/detector` package with four exported functions and supporting types:

- `DetectPatterns` — 6 injection categories, 16 compiled regexes, multi-word phrases to minimize false positives
- `DetectEncoding` — base64 (entropy-gated), hex-escape sequences, raw hex strings, control char density
- `DetectOutliers` — cosine-similarity outlier scoring from LexRank matrix; takes pre-normalization matrix
- `Analyze` — combined pattern+encoding report; outlier detection handled separately (requires matrix)

28 unit tests. All findings carry Category, Sentence index, byte Offset, Score, Pattern name, and Excerpt.

## Key decisions

- Pattern regexes use multi-word phrases (e.g., `ignore all previous instructions` not just `ignore`) — reduces false positives on common verbs
- Base64 detection uses entropy gate (>4.5 bits/char) + decode validation to avoid flagging short random strings
- `DetectOutliers` takes a precomputed matrix parameter — no circular import with summarizer package; LexRank exposes matrix via `MatrixSummarizer` interface
- Detection is always advisory: `Report.Suspicious` is a flag, not a gate. Callers decide what to do.
- `DefaultDetectionThreshold = 0.70` for pattern/encoding; `DefaultOutlierThreshold = 0.85` for statistical outliers (separate because different scale)
