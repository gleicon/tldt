# tldt — v1 Requirements

## v1 Requirements

### Core CLI

- [ ] **CLI-01**: User can invoke `tldt` as a standalone binary from PATH
- [ ] **CLI-02**: User can pipe text via stdin: `cat file.txt | tldt`
- [ ] **CLI-03**: User can specify input file: `tldt -f article.txt`
- [ ] **CLI-04**: User can pass text as positional argument: `tldt "long text..."`
- [ ] **CLI-05**: When stdout is piped, output contains ONLY summary text (no metadata, no decoration)
- [ ] **CLI-06**: When stdout is a TTY, output includes compression stats to stderr
- [ ] **CLI-07**: Empty or whitespace-only input exits 0 with no output (pipe-safe)
- [ ] **CLI-08**: Binary/non-text input detected and rejected with error to stderr

### Summarization

- [ ] **SUM-01**: User can control output sentence count: `--sentences N` (default: 5)
- [ ] **SUM-02**: User can group output sentences into paragraphs: `--paragraphs N`
- [ ] **SUM-03**: User can select algorithm: `--algorithm lexrank|textrank|graph` (default: lexrank)
- [ ] **SUM-04**: When N > available sentences, return all sentences without error
- [ ] **SUM-05**: Output sentences appear in original document order (not score order)
- [ ] **SUM-06**: LexRank algorithm implemented natively with IDF-modified cosine similarity
- [ ] **SUM-07**: TextRank algorithm implemented natively with word-overlap + PageRank
- [ ] **SUM-08**: `graph` algorithm delegates to `github.com/didasy/tldr` as baseline comparison

### Token Awareness

- [ ] **TOK-01**: Tool displays estimated token count before and after: `~12,400 → ~1,380 tokens (89% reduction)`
- [ ] **TOK-02**: Token estimate uses chars/4 heuristic, labeled as estimated
- [ ] **TOK-03**: Token stats displayed to stderr (never stdout) so they don't break pipes

### Output Formats

- [ ] **OUT-01**: Default output is plain text (pipe-safe)
- [ ] **OUT-02**: `--format json` outputs structured JSON: `{summary, algorithm, sentences_in, sentences_out, chars_in, chars_out, tokens_estimated_in, tokens_estimated_out, compression_ratio}`
- [ ] **OUT-03**: `--format markdown` wraps summary in a markdown blockquote with metadata header

### Quality & Testing

- [ ] **TEST-01**: Unit tests for TF-IDF computation (known input → expected vectors)
- [ ] **TEST-02**: Unit tests for cosine similarity (orthogonal → 0.0, identical → 1.0)
- [ ] **TEST-03**: Unit tests for power iteration convergence on toy matrix
- [ ] **TEST-04**: Integration tests using test-data/ files — verify key sentences appear in output
- [ ] **TEST-05**: Edge case tests: empty input, single sentence, N > sentence count, unicode
- [ ] **TEST-06**: Deterministic output: same input always produces same output
- [ ] **TEST-07**: Test data includes: English Wikipedia article, raw YouTube transcript, long-form (3000+ words), sub-5-sentence edge case

### Project Hygiene

- [x] **PROJ-01**: Modern go modules (`go.mod` at repo root, drop old GOPATH/src/ structure)
- [ ] **PROJ-02**: Updated README with: what tldt is, install, usage examples, algorithm explanation, comparison table LexRank vs TextRank
- [ ] **PROJ-03**: Sentence count cap at 2000 (default) with `--no-cap` override flag for O(n²) safety
- [ ] **PROJ-04**: Build via `go build ./...`, test via `go test ./...`

## v2 Requirements (deferred)

- Clipboard auto-read when no stdin/file given (macOS `pbpaste`, Linux `xclip`)
- `--url` flag to fetch and summarize a web page directly
- ROUGE score evaluation mode for testing summary quality
- Language detection and multilingual support
- Streaming output (sentence by sentence as ranked)
- Config file (`~/.tldt.toml`) for persistent defaults
- Ensemble algorithm combining LexRank + TextRank scores (`--algorithm ensemble`)

## v3 Requirements (backlog)

### AI Assistant Integration (SKILL-01)

Install tldt as a callable skill/tool inside Claude Code, OpenCode, Copilot, and compatible
AI assistants — modeled after the RTK hook-based installation pattern.

**Compression levels** (`--level lite|standard|aggressive`):
- `lite` — 10 sentences, ratio ~70%, preserves more context
- `standard` — 5 sentences (default), ratio ~85%
- `aggressive` — 3 sentences, ratio ~95%, maximum token savings

Level maps to `--sentences` + target compression ratio. If actual ratio falls short of target,
retry with fewer sentences (one step down).

**Auto-trigger rules** (fire without explicit invocation):
- Paste or file input exceeds N tokens (configurable threshold, default 4000)
- File extension matches a watched list (`.txt`, `.md`, `.pdf`, `.transcript`)
- Piped input from clipboard exceeds threshold

**Per-assistant installation targets:**
- Claude Code — hook in `settings.json` (`UserPromptSubmit` or `PreToolUse` on Read)
- OpenCode — tool definition in agent config
- GitHub Copilot — VS Code extension skill via `@tldt` mention
- Generic — MCP server wrapper exposing `tldt_summarize` tool

**Skill contract:**
```
Input:  { text: string, level?: "lite"|"standard"|"aggressive" }
Output: { summary: string, tokens_in: int, tokens_out: int, compression: float }
```

## Out of Scope

- HTTP server / web API — dropped entirely (was resumator template artifact)
- Redis / database — no persistence for CLI use case
- Authentication — not applicable
- LLM integration — antithetical to purpose
- Abstractive summarization — LexRank/TextRank are extractive only
- Exact tiktoken token counting — heuristic sufficient; no Go tiktoken port

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| CLI-01 | Phase 1 — Foundation | Pending |
| CLI-02 | Phase 1 — Foundation | Pending |
| CLI-03 | Phase 1 — Foundation | Pending |
| CLI-04 | Phase 1 — Foundation | Pending |
| CLI-05 | Phase 3 — Polish | Pending |
| CLI-06 | Phase 3 — Polish | Pending |
| CLI-07 | Phase 3 — Polish | Pending |
| CLI-08 | Phase 3 — Polish | Pending |
| SUM-01 | Phase 2 — Algorithms | Pending |
| SUM-02 | Phase 2 — Algorithms | Pending |
| SUM-03 | Phase 2 — Algorithms | Pending |
| SUM-04 | Phase 2 — Algorithms | Pending |
| SUM-05 | Phase 2 — Algorithms | Pending |
| SUM-06 | Phase 2 — Algorithms | Pending |
| SUM-07 | Phase 2 — Algorithms | Pending |
| SUM-08 | Phase 1 — Foundation | Pending |
| TOK-01 | Phase 2 — Algorithms | Pending |
| TOK-02 | Phase 2 — Algorithms | Pending |
| TOK-03 | Phase 2 — Algorithms | Pending |
| OUT-01 | Phase 3 — Polish | Pending |
| OUT-02 | Phase 3 — Polish | Pending |
| OUT-03 | Phase 3 — Polish | Pending |
| TEST-01 | Phase 2 — Algorithms | Pending |
| TEST-02 | Phase 2 — Algorithms | Pending |
| TEST-03 | Phase 2 — Algorithms | Pending |
| TEST-04 | Phase 2 — Algorithms | Pending |
| TEST-05 | Phase 2 — Algorithms | Pending |
| TEST-06 | Phase 2 — Algorithms | Pending |
| TEST-07 | Phase 1 — Foundation | Pending |
| PROJ-01 | Phase 1 — Foundation | Complete |
| PROJ-02 | Phase 3 — Polish | Pending |
| PROJ-03 | Phase 3 — Polish | Pending |
| PROJ-04 | Phase 3 — Polish | Pending |
