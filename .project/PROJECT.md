# tldt — Too Long, Didn't Tokenize

## Overview

`tldt` is a Go CLI and embeddable library that compresses long-form text (articles, transcripts, docs, web pages) into short extractive summaries — and acts as a security preprocessor for untrusted text fed to LLMs. It runs entirely locally: no LLM calls, no API keys, no token cost. Summaries are always exact quotes from the source (extractive, never paraphrased), produced by graph-based ranking (LexRank/TextRank). Beyond summarization it offers Unicode sanitization, prompt-injection detection, and PII/secret detection — positioning it as middleware that guards input to AI coding agents (Claude Code, Cursor, OpenCode). It ships a Claude Code skill + `UserPromptSubmit` hook installer.

## Stack

- **Language / runtime:** Go 1.26.2 (module `github.com/gleicon/tldt`), targets darwin/arm64 and others.
- **Key dependencies:** `didasy/tldr` (baseline graph summarizer), `go-shiori/go-readability` (HTML→text extraction), `JohannesKaufmann/html-to-markdown/v2` (HTML→Markdown), `BurntSushi/toml` (config), `golang.org/x/text` (Unicode NFKC). Algorithms (LexRank, TextRank, ensemble, ROUGE) are implemented natively in `internal/summarizer`.
- **Build/test (see `Makefile`):**
  - `make build` → compiles `./cmd/tldt` to `./tldt`
  - `make test` → `go test ./...`; `make test-race`, `make test-cover`, `make bench`
  - `make install` → `go install ./cmd/tldt`; `make install-skill` → installs Claude Code skill + hook
  - `make lint` → `go vet ./...`
  - Release via `.goreleaser.yaml` (`make release VERSION=vX.Y.Z`)

## Repo map

| Path | Holds |
|------|-------|
| `cmd/tldt/` | CLI entry point (`main.go`) — flag parsing, input resolution (stdin/`-f`/`--url`/positional), wires `pkg/tldt`. `main_test.go` runs subprocess tests. |
| `pkg/tldt/` | Public embeddable API — `Summarize`, `Detect`, `Sanitize`, `Fetch` (HTML→text), `FetchRaw` (hardened raw fetch for JSON/non-HTML), `DetectPII`, `SanitizePII`, and `Pipeline` (sanitize→inject→PII→summarize). Only public surface; wraps `internal/`. |
| `internal/summarizer/` | Native ranking algorithms: `lexrank`, `textrank`, `ensemble`, `graph` (didasy wrapper), `rouge` evaluation, `tokenizer`, sentence `graph`, `explain` diagnostics. |
| `internal/detector/` | Prompt-injection / encoding-anomaly detection + Unicode confusables (`data/confusables.txt`). |
| `internal/sanitizer/` | Invisible-Unicode stripping + NFKC normalization; PII/secret detection & redaction. |
| `internal/fetcher/` | Hardened URL fetch — custom `http.Client`, `io.LimitReader` (5MB), SSRF blocklist (RFC1918/loopback/metadata), ≤5 redirect cap. |
| `internal/htmlmd/` | HTML→Markdown conversion (readability + html-to-markdown). |
| `internal/formatter/` | Output rendering: text / json / markdown. |
| `internal/config/` | `~/.tldt.toml` loader (default algorithm, sentences, format, level). |
| `internal/installer/` | Installs Claude Code/Cursor/OpenCode skill + hook; `hooks/tldt-hook.sh`, embedded assets (`embed.go`). |
| `examples/` | Standalone Go programs (own go.mod) demonstrating library use: `basic`, `pipeline`, `html-processor`, `openapi-client`. |
| `docs/` | `security.md` (OWASP LLM Top 10 coverage), `index.html`, `library.html`. |
| `test-data/` | Real-world fixtures (Wikipedia, YouTube transcript, longform, edge cases). |

## Constraints

- **No LLM / no network dependency for core function** — summarization is purely local and deterministic; adding cloud/LLM calls is antithetical to the tool's purpose.
- **Extractive only** — output must be verbatim source sentences; no abstractive/generative summarization.
- **Pipe-safe stdout** — only the summary goes to stdout; all stats/warnings/diagnostics go to stderr (never break shell pipes). TTY detection governs interactive behavior.
- **No HTTP server / persistence** — the web API, Redis, and auth from the `resumator` origin are dropped. The legacy `resumator` tree (`src/`, `assets/`, `SSL/`) was removed; see git history if needed.
- **CLI depends on the library** — `cmd/tldt` routes core operations through `pkg/tldt` (direct `internal/` imports limited to `config`, `formatter`, `installer`). Keep `pkg/tldt` the authoritative API.
- **Security at the boundary** — URL fetching enforces SSRF blocklist, byte cap, and redirect cap; untrusted text is validated where it enters.
- **Go conventions (per AGENTS.md):** `go test -race ./...` clean, `golangci-lint`/`go vet` clean, table-driven tests, no real network/filesystem in unit tests (use `httptest`), functions kept under ~70 lines.
