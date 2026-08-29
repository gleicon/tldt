# Project Map

## Overview

tldt ("Too Long, Didn't Tokenize") is a Go CLI and embeddable library that shrinks long text
before it reaches an LLM. It performs graph-based *extractive* summarization — output is always
verbatim sentences from the input, never paraphrase — with no API calls or model dependencies.
Alongside summarization it ships a security pipeline for untrusted text: prompt-injection
detection, PII/secret scanning and redaction, Unicode steganography sanitization, hidden-surface
extraction from HTML/PDF/DOCX/XLSX, and AI-generated-content scoring. It targets developers piping
context into coding agents, and installs itself as a Claude Code / Codex / Cursor / OpenCode skill
plus a `UserPromptSubmit` hook.

## Stack

- **Language/runtime**: Go 1.26.2, module `github.com/gleicon/tldt`. Also builds to `js/wasm`.
- **Key deps**: `didasy/tldr` (summarization primitives), `go-shiori/go-readability` (article
  extraction), `JohannesKaufmann/html-to-markdown/v2`, `BurntSushi/toml` (config), `golang.org/x/net`,
  `golang.org/x/text`.
- **Build/test** (see `Makefile`): `make build`, `make test` (runs `test-injection` first, then
  `go test ./...`), `make test-race`, `make test-cover` (two-pass: unit profile + `GOCOVERDIR`
  subprocess coverage of `cmd/tldt`), `make bench`, `make lint` (`.golangci.yml`), `make install`,
  `make install-skill`, `make wasm` / `make demo` (builds into `docs/`), `make release`
  (`.goreleaser.yaml`).

## Repo map

| Path | Contents |
|---|---|
| `cmd/tldt/` | CLI entry point. `main.go` holds all flags and the stage pipeline (`resolveInputBytes` → `applyMutatingStages` → `runDetectionStderr` → `summarize` → `writeOutput`); `detect.go`, `stats.go`. |
| `pkg/tldt/` | Sole public API surface. `tldt.go` exposes `Summarize`, `Detect`, `DetectPII`, `SanitizePII`, `Sanitize`, `Fetch`/`FetchRaw`, `ConvertHTML`, `DetectAI`, `EvalROUGE`, `Pipeline`; `doc.go` is the package guide. |
| `internal/summarizer/` | Algorithms behind the `Summarizer` interface: `lexrank.go`, `textrank.go`, `graph.go`, `ensemble.go`, plus `tokenizer.go`, `explain.go`, `rouge.go`. |
| `internal/detector/` | Prompt-injection detection: pattern/taxonomy matching, base64+hex entropy anomalies, LexRank statistical outliers, PII findings, `confusables.go`, wordlist `data/`. Advisory only. |
| `internal/aidetect/` | AI-text scoring: `detector.go` (excess-vocabulary, Kobak et al. 2024), `linguistic.go` (stylometric signals), embedded `wordlists/` per language (en, pt-BR, es). |
| `internal/sanitizer/` | Invisible/zero-width/bidi character stripping and NFKC normalization. |
| `internal/surfaces/` | `HiddenSurface` type and `Source*` constants shared by all extractors. |
| `internal/extractor/` | Hidden-surface extraction per format: `html.go`, `pdf.go`, `docx.go`, `xlsx.go`. |
| `internal/fetcher/` | URL fetch + readability article extraction (`--url`). |
| `internal/htmlmd/` | HTML → Markdown conversion (`--from-html`). |
| `internal/config/` | `~/.tldt.toml` loading, level presets, `SecurityConfig`; never returns an unusable config. |
| `internal/formatter/` | Output rendering: text, JSON, Markdown. |
| `internal/usage/` | Counts-only JSONL usage log at `~/.tldt/usage.jsonl`. |
| `internal/installer/` | Writes skill/hook templates to agent config dirs; embedded templates under `skills/`, `hooks/`, `codex/`, `opencode/`. |
| `internal/testutil/` | Non-test helper package (zip/docx fixture builders) importable across test files. |
| `wasm/` | `js/wasm` entry point exporting `summarize()` to JavaScript. |
| `docs/` | Static demo site: `index.html`, `library.html`, `demo.html`/`demo.js`, `security.md`. WASM build output lands here. |
| `examples/` | Runnable library examples: `basic/`, `pipeline/`, `html-processor/`, `openapi-client/`. |
| `test-data/` | Fixture corpora (wikipedia, transcripts, long-form, edge cases). |
| `dist/` | GoReleaser cross-platform build output. |
| `.project/` | Session working notes (`PROJECT.md`, this map). |
