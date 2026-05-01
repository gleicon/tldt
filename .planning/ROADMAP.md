# Roadmap: tldt — Too Long, Didn't Tokenize

## Overview

A brownfield Go web API is transformed into a pure CLI summarization tool. The three phases follow a natural delivery arc: get the project skeleton and module structure working (Foundation), implement the graph algorithms that do the actual work (Algorithms), then harden the CLI for real-world pipeline use (Polish). Each phase ships a verifiable, runnable binary milestone.

## Phases

- [ ] **Phase 1: Foundation** - Modernize to go modules, clean CLI skeleton, baseline graph algorithm, test data
- [ ] **Phase 2: Algorithms** - Implement LexRank and TextRank natively, expose algorithm/sentence/paragraph flags, full test suite
- [ ] **Phase 3: Polish** - TTY detection, output formats (JSON/markdown), pipe safety, O(n²) cap, README

## Phase Details

### Phase 1: Foundation
**Goal**: A working go-modules project with a CLI binary that accepts text input and produces extractive summaries via the graph baseline algorithm.
**Depends on**: Nothing (first phase)
**Requirements**: PROJ-01, CLI-01, CLI-02, CLI-03, CLI-04, SUM-08, TEST-07
**Success Criteria** (what must be TRUE):
  1. `go build ./...` and `go test ./...` succeed with no errors from the repo root
  2. `echo "text..." | tldt` runs without panicking and returns non-empty output
  3. `tldt -f article.txt` and `tldt "text..."` both produce output without error
  4. `github.com/didasy/tldr` graph algorithm is selectable and produces output on all test-data/ files
**Plans**: TBD
**UI hint**: no

### Phase 2: Algorithms
**Goal**: LexRank and TextRank are implemented natively in Go and selectable via flags, with a deterministic, fully-tested summarization pipeline.
**Depends on**: Phase 1
**Requirements**: SUM-01, SUM-02, SUM-03, SUM-04, SUM-05, SUM-06, SUM-07, TOK-01, TOK-02, TOK-03, TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06
**Success Criteria** (what must be TRUE):
  1. `tldt --algorithm lexrank --sentences 3 -f article.txt` returns exactly 3 sentences in original document order
  2. `tldt --algorithm textrank --sentences 5 -f article.txt` returns a different (but valid) 5-sentence summary
  3. `go test ./...` passes all unit tests including TF-IDF vectors, cosine similarity, and power iteration convergence
  4. Running the same input twice always produces identical output (deterministic)
**Plans**: TBD
**UI hint**: no

### Phase 3: Polish
**Goal**: The binary is pipe-safe and production-ready: TTY-aware stats output, structured output formats, and input validation.
**Depends on**: Phase 2
**Requirements**: CLI-05, CLI-06, CLI-07, CLI-08, OUT-01, OUT-02, OUT-03, PROJ-02, PROJ-03, PROJ-04
**Success Criteria** (what must be TRUE):
  1. `cat article.txt | tldt | wc -l` captures only summary lines — no stats, no decoration on stdout
  2. Running `tldt -f article.txt` in a terminal shows `~12,400 → ~1,380 tokens (89% reduction)` on stderr
  3. `tldt --format json -f article.txt` outputs valid JSON with all required fields (summary, algorithm, compression_ratio, etc.)
  4. `tldt` given empty input exits 0 with no output; binary/non-text input prints an error to stderr and exits non-zero
**Plans**: TBD
**UI hint**: no

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 0/TBD | Not started | - |
| 2. Algorithms | 0/TBD | Not started | - |
| 3. Polish | 0/TBD | Not started | - |
