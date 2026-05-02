# Roadmap: tldt — Too Long, Didn't Tokenize

## Overview

A brownfield Go web API is transformed into a pure CLI summarization tool. The three phases follow a natural delivery arc: get the project skeleton and module structure working (Foundation), implement the graph algorithms that do the actual work (Algorithms), then harden the CLI for real-world pipeline use (Polish). Each phase ships a verifiable, runnable binary milestone.

## Phases

- [x] **Phase 1: Foundation** - Modernize to go modules, clean CLI skeleton, baseline graph algorithm, test data
- [x] **Phase 2: Algorithms** - Implement LexRank and TextRank natively, expose algorithm/sentence/paragraph flags, full test suite
- [x] **Phase 3: Polish** - TTY detection, output formats (JSON/markdown), pipe safety, O(n²) cap, README

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
**Plans**: 3 plans

**Wave 1**
- [x] 01-01-PLAN.md — Go module init, dependency fetch, Makefile replacement, directory scaffolds

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 01-02-PLAN.md — Graph summarizer wrapper (internal/summarizer) and CLI entry point (cmd/tldt/main.go)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 01-03-PLAN.md — Four English test-data files and integration tests covering all test-data fixtures
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
**Plans**: 4 plans

Plans:
- [x] 02-01-PLAN.md — Sentence tokenizer, Summarizer interface + registry, Graph struct wrapper
- [x] 02-02-PLAN.md — LexRank algorithm (TF-IDF, cosine similarity, power iteration) with unit tests
- [x] 02-03-PLAN.md — TextRank algorithm (word overlap, damped PageRank iteration) with unit tests
- [x] 02-04-PLAN.md — CLI flag wiring (--algorithm, --sentences, --paragraphs), token stats, integration tests

**Wave 1**
- [x] 02-01-PLAN.md — Sentence tokenizer, Summarizer interface + registry, Graph struct wrapper

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 02-02-PLAN.md — LexRank algorithm with TF-IDF, cosine similarity, power iteration, and full unit tests
- [x] 02-03-PLAN.md — TextRank algorithm with word overlap similarity, damped power iteration, and full unit tests

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 02-04-PLAN.md — CLI flags, token stats to stderr, paragraph grouping, integration tests for all algorithms
**UI hint**: no

### Phase 3: Polish
**Goal**: The binary is pipe-safe and production-ready: TTY-aware stats output, structured output formats, and input validation.
**Depends on**: Phase 2
**Requirements**: CLI-05, CLI-06, CLI-07, CLI-08, OUT-01, OUT-02, OUT-03, PROJ-02, PROJ-03, PROJ-04
**Success Criteria** (what must be TRUE):
  1. `cat article.txt | tldt | wc -l` captures only summary lines — no stats, no decoration on stdout
  2. Running `tldt -f article.txt` in a terminal shows `~12,400 -> ~1,380 tokens (89% reduction)` on stderr
  3. `tldt --format json -f article.txt` outputs valid JSON with all required fields (summary, algorithm, compression_ratio, etc.)
  4. `tldt` given empty input exits 0 with no output; binary/non-text input prints an error to stderr and exits non-zero
**Plans**: 4 plans

Plans:
- [x] 03-01-PLAN.md — TTY gate, input validation (empty/binary), sentence cap, stats format fix
- [x] 03-02-PLAN.md — internal/formatter package (FormatText, FormatJSON, FormatMarkdown) with unit tests
- [x] 03-03-PLAN.md — Wire --format flag into main.go, formatter dispatch, JSON stats suppression
- [x] 03-04-PLAN.md — README rewrite for tldt v1

**Wave 1** *(parallel — no shared files)*
- [x] 03-01-PLAN.md — TTY gate, input validation, sentence cap, stats format fix (cmd/tldt/main.go)
- [x] 03-02-PLAN.md — internal/formatter package with all three format functions and unit tests
- [x] 03-04-PLAN.md — README rewrite

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 03-03-PLAN.md — Wire --format flag into main.go using formatter package
**UI hint**: no

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 3/3 | Complete | 2026-05-01 |
| 2. Algorithms | 4/4 | Complete | 2026-05-01 |
| 3. Polish | 4/4 | Complete | 2026-05-02 |

---

## Milestone v2.0: Extensions

### Overview

v2.0 expands tldt's reach in three focused phases: URL input adds a new content source without touching the core summarization pipeline; Configuration persists user defaults and introduces compression presets as a friendlier interface over raw `--sentences`; AI Integration ships tldt as an installable Claude Code skill with an auto-trigger hook. Each phase is independently deliverable and depends on the previous.

### Phases

- [ ] **Phase 4: URL Input** - User can pass a URL to tldt and receive an extractive summary of the fetched page
- [ ] **Phase 5: Configuration** - User preferences persist across invocations via ~/.tldt.toml; compression presets simplify common sentence counts
- [ ] **Phase 6: AI Integration** - tldt ships as an installable Claude Code skill file with an auto-trigger hook that fires when input exceeds a token threshold

## Phase Details

### Phase 4: URL Input
**Goal**: Users can summarize a live webpage by passing its URL to tldt — no manual copy-paste required.
**Depends on**: Phase 3
**Requirements**: INP-01, INP-02
**Success Criteria** (what must be TRUE):
  1. `tldt --url https://example.com/article` fetches the page, strips HTML boilerplate, and prints an extractive summary to stdout
  2. `tldt --url https://example.com/article | wc -l` produces only summary text on stdout — no headers, no HTML, no decoration
  3. `tldt --url https://httpstat.us/404` exits non-zero and prints a descriptive error to stderr
  4. `tldt --url https://httpstat.us/301` follows the redirect transparently and still produces a summary
**Plans**: 2 plans

Plans:
- [ ] 04-01-PLAN.md — go-readability dependency + internal/fetcher package (Fetch function + 5 unit tests)
- [ ] 04-02-PLAN.md — Wire --url flag into main.go; fix 5 existing resolveInputBytes call sites; add 2 URL integration tests

**Wave 1**
- [ ] 04-01-PLAN.md — go-readability dependency + internal/fetcher package (Fetch function + 5 unit tests)

**Wave 2** *(blocked on Wave 1 completion)*
- [ ] 04-02-PLAN.md — --url flag wiring in main.go; main_test.go fixes and new URL integration tests
**UI hint**: no

### Phase 5: Configuration
**Goal**: Users can persist their preferred flags in ~/.tldt.toml and use named compression presets instead of raw sentence counts.
**Depends on**: Phase 4
**Requirements**: CFG-01, CFG-02, CFG-03, CFG-04, CFG-05
**Success Criteria** (what must be TRUE):
  1. After writing `algorithm = "ensemble"` and `sentences = 7` to `~/.tldt.toml`, running `tldt -f article.txt` (no extra flags) uses those values
  2. `tldt --sentences 3 -f article.txt` overrides a `sentences = 7` entry in `~/.tldt.toml` and returns exactly 3 sentences
  3. Deleting or corrupting `~/.tldt.toml` does not cause an error — the tool silently falls back to built-in defaults
  4. `tldt --level aggressive -f article.txt` returns 10 sentences; `--level lite` returns 3; `--level standard` returns 5
**Plans**: TBD
**UI hint**: no

### Phase 6: AI Integration
**Goal**: tldt is installable as a Claude Code skill and fires automatically when pasted or file-sourced text exceeds a configurable token threshold.
**Depends on**: Phase 5
**Requirements**: AI-01, AI-02, AI-03, AI-04
**Success Criteria** (what must be TRUE):
  1. A user can copy the shipped skill file into their Claude Code skills directory and invoke tldt on selected text from within a Claude Code session — the summary appears inline in the conversation
  2. The skill passes text to tldt via stdin and the returned summary replaces the raw input in the conversation context
  3. With the auto-trigger hook installed and threshold set to 2000 tokens, pasting a 3000-token block causes tldt to summarize it automatically before it enters the AI context
  4. After auto-trigger fires, the tool reports the token savings (e.g. `~3,200 -> ~480 tokens (85% reduction)`) before inserting the summary
**Plans**: TBD
**UI hint**: no

## v2.0 Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 4. URL Input | 0/2 | Not started | - |
| 5. Configuration | 0/? | Not started | - |
| 6. AI Integration | 0/? | Not started | - |
