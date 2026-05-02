---
phase: 02-algorithms
plan: "04"
subsystem: cli
tags: [cli, integration, algorithms, token-stats, paragraph-grouping]
dependency_graph:
  requires: [02-01, 02-02, 02-03]
  provides: [CLI-algorithm-flag, CLI-sentences-flag, CLI-paragraphs-flag, token-stats-stderr, integration-tests]
  affects: [cmd/tldt/main.go, internal/summarizer/integration_test.go]
tech_stack:
  added: []
  patterns:
    - algorithm registry dispatch via summarizer.New()
    - token compression stats on stderr (chars/4 heuristic)
    - paragraph grouping with silent cap
    - one-sentence-per-line output format
key_files:
  created: []
  modified:
    - cmd/tldt/main.go
    - internal/summarizer/integration_test.go
decisions:
  - "chars/4 heuristic for token count estimation (D-09)"
  - "silent cap for --paragraphs exceeding sentence count (D-06)"
  - "result variable name avoids conflict with sentences flag pointer"
metrics:
  duration: "8 minutes"
  completed: "2026-05-02T01:42:12Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
---

# Phase 2 Plan 04: CLI Integration and Integration Tests Summary

CLI integration wiring algorithm registry to --algorithm/--sentences/--paragraphs flags with token compression stats on stderr and 10 new integration tests covering LexRank and TextRank on all 4 test fixtures.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Wire CLI flags, token stats, and paragraph grouping | f9a10f1 | cmd/tldt/main.go |
| 2 | Extend integration tests for LexRank and TextRank | d7aa50c | internal/summarizer/integration_test.go |

## What Was Built

### Task 1: CLI Flags, Token Stats, Paragraph Grouping (cmd/tldt/main.go)

- Removed `const defaultSentences` and the direct `summarizer.Summarize()` call
- Added `--algorithm` flag (default: "lexrank"), `--sentences` flag (default: 5), `--paragraphs` flag (default: 0 = off)
- Dispatches to `summarizer.New(*algorithm)` registry — invalid algorithm names return error and exit 1 (threat T-02-07)
- Emits `~N -> ~M tokens (P% reduction)` to stderr after every run
- Outputs one sentence per line by default; `--paragraphs N` groups sentences into N blank-line-separated paragraphs
- Added `formatTokens(n int) string` helper with comma-separated thousands formatting
- Added `groupIntoParagraphs(sentences []string, n int) string` helper with silent cap when N exceeds sentence count

### Task 2: Integration Tests (internal/summarizer/integration_test.go)

Added 10 new test functions without modifying existing ones (`repoRoot`, `readTestFile`, `TestSummarize_*`):

- `TestNew_LexRank_WikipediaEn` — LexRank on wikipedia_en.txt, n=5, asserts non-empty and len<=5
- `TestNew_LexRank_YoutubeTranscript` — LexRank on youtube_transcript.txt, n=5, asserts non-empty
- `TestNew_LexRank_Longform` — LexRank on longform_3000.txt, n=5, asserts exactly 5 sentences
- `TestNew_LexRank_EdgeShort` — LexRank on edge_short.txt, n=5, asserts len<=3 (silent cap SUM-04)
- `TestNew_TextRank_WikipediaEn` — TextRank on wikipedia_en.txt
- `TestNew_TextRank_YoutubeTranscript` — TextRank on youtube_transcript.txt
- `TestNew_TextRank_Longform` — TextRank on longform_3000.txt, asserts exactly 5 sentences
- `TestNew_TextRank_EdgeShort` — TextRank on edge_short.txt, asserts len<=3
- `TestNew_UnknownAlgorithm` — registry returns error for unknown name (TEST-05)
- `TestNew_LexRank_Deterministic_RealData` — two calls produce identical output on wikipedia_en.txt (TEST-06)

## Verification Results

```
go build ./...                        # success
go test ./... -count=1                # 49 tests passing across 2 packages
echo "..." | tldt --algorithm lexrank --sentences 3    # 3 lines on stdout
echo "..." | tldt --algorithm textrank --sentences 3   # 3 lines on stdout
echo "..." | tldt --algorithm graph --sentences 3      # 3 lines on stdout (backward compat)
stderr: ~16 -> ~9 tokens (43% reduction)               # correct format
```

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — all flags are wired to real algorithm implementations.

## Threat Flags

No new security-relevant surfaces beyond those in the plan's threat model.

## Self-Check: PASSED

- cmd/tldt/main.go exists and contains all required patterns
- internal/summarizer/integration_test.go contains all 10 new test functions
- Commit f9a10f1 exists (Task 1)
- Commit d7aa50c exists (Task 2)
- 49 tests pass, 0 failures
