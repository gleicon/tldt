# tldt — Too Long, Didn't Tokenize

## What This Is

**tldt** is a CLI tool for developers and AI users to summarize long-form text (YouTube transcripts, articles, documentation) into concise, meaningful summaries **without using LLMs or consuming tokens**.

It uses graph-based extractive summarization algorithms (LexRank and TextRank) to identify the most centroid sentences — the ones that are most representative of the document as a whole. Unlike LLM-based summarization, it preserves the original meaning by selecting actual sentences from the source text rather than generating new ones.

The analogy: **rtk saves shell tokens by cutting verbosity. tldt saves LLM tokens by summarizing text before you paste it into a coding agent.**

## Core Value

Let developers paste long articles/transcripts into AI coding agents with dramatically less token cost while preserving the semantic core of the content.

## What This Is NOT

- Not a replacement for LLM summarization when abstractive (generative) summaries are needed
- Not a web service (HTTP API dropped — pure CLI)
- Not dependent on any external API, network, or cloud service

## Context

- **Origin**: Evolved from `resumator`, a Go web API template using `github.com/JesusIslam/tldr` (TextRank-based)
- **Transformation**: Drop HTTP server entirely. Full rewrite as CLI tool with LexRank + TextRank support
- **Language**: Go (with modern go modules replacing old GOPATH style)
- **Runtime**: Go 1.26.2 on darwin/arm64

## Target Users

- Developers using AI coding assistants (Claude Code, Cursor, Copilot, etc.)
- Anyone who needs to feed long documents to AI models efficiently
- Workflows: YouTube summary → paste into agent, article research → summarize → agent context

## Key Design Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Pure CLI, no HTTP server | Simpler, composable via pipes, fits developer workflow | Drop all HTTP/Redis/config infra |
| LexRank + TextRank both supported | LexRank (eigenvector centrality) vs TextRank (PageRank) have different tradeoffs; expose both | `--algorithm lexrank\|textrank` flag |
| Implement algorithms natively in Go | No external NLP deps, evaluate existing Go libs first | Depends on library evaluation |
| Stdin + file + positional arg input | Maximum composability in shell pipelines | `cat file | tldt`, `tldt -f file.txt`, `tldt "text"` |
| Tunable sentence count | Key parameter for token savings tradeoff | `--sentences N` flag (default: 5) |
| Modern go modules | Go 1.26.2, module-based, testable | `go.mod` at repo root |

## Requirements

### Validated

- ✓ Basic extractive summarization working — existing (TextRank via JesusIslam/tldr)
- ✓ Tunable sentence count — existing (via config)
- ✓ Modern go modules setup — Validated in Phase 1: Foundation (PROJ-01)
- ✓ CLI binary replaces web server — Validated in Phase 1: Foundation (CLI-01–04)
- ✓ stdin pipe input (`echo text | tldt`) — Validated in Phase 1: Foundation (CLI-02)
- ✓ File input (`tldt -f file.txt`) — Validated in Phase 1: Foundation (CLI-03)
- ✓ Positional text arg (`tldt "text..."`) — Validated in Phase 1: Foundation (CLI-04)
- ✓ graph algorithm via didasy/tldr — Validated in Phase 1: Foundation (SUM-08)
- ✓ Comprehensive test suite with real-world test data — Validated in Phase 1: Foundation (TEST-07)

### Active

- [ ] LexRank algorithm implemented natively
- [ ] TextRank algorithm implemented natively
- [ ] `--algorithm` flag to choose lexrank|textrank|graph
- [ ] `--sentences N` flag (default 5)
- [ ] `--paragraphs N` flag (groups sentences into paragraphs)
- [ ] Token count estimate output (before/after compression ratio)
- [ ] Updated README with usage examples and algorithm explanation

### Out of Scope

- HTTP server / web API — dropped entirely
- Redis / database storage — no persistence needed for CLI
- Authentication / rate limiting — not applicable
- LLM integration — antithetical to tool's purpose
- Abstractive summarization — LexRank/TextRank are extractive only

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Evaluate JesusIslam/tldr + didasy/tldr | May save implementation time if they're LexRank-capable | Phase 1 research task |
| Implement from scratch if needed | Ensures correctness, no hidden deps | Phase 2 if library evaluation fails |
| Support both LexRank + TextRank | Users can compare; different texts favor different algorithms | Architecture decision for Phase 2 |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-05-01 after Phase 1 Foundation complete*
