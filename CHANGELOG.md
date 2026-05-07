# Changelog

All notable changes to this project are documented in this file.

## [1.0.0] - 2026-05-06

### Added

- Extractive summarization with four algorithms: LexRank, TextRank, graph (baseline), and ensemble
- Stateless Go library API in `pkg/tldt` for embedding in other applications
- URL fetching with SSRF protection and redirect limits
- PII detection and redaction (emails, API keys, JWTs, credit cards)
- Prompt injection detection (patterns, encoding anomalies, statistical outliers)
- Unicode sanitization (invisible characters, NFKC normalization)
- Config file support (`~/.tldt.toml`) with compression presets
- Claude Code skill integration with auto-trigger hook
- ROUGE evaluation for summary quality measurement
- JSON and Markdown output formats

### Library API

The public API surface is `github.com/gleicon/tldt/pkg/tldt`:

- `Summarize(text, SummarizeOptions)` - Extractive summarization
- `Fetch(url, FetchOptions)` - URL fetching with SSRF protection
- `Detect(text, DetectOptions)` - Injection pattern detection
- `Sanitize(text)` - Unicode normalization and cleaning
- `DetectPII(text)` - PII/secret detection
- `SanitizePII(text)` - PII redaction
- `Pipeline(text, PipelineOptions)` - Full processing pipeline

All functions are stateless and safe for concurrent use.

### Security

- SSRF blocking for private IP ranges and cloud metadata endpoints
- Redirect chain limits (5 hops maximum)
- Cross-script homoglyph detection (UTS#39)
- PII redaction before summarization
- Output guard in Claude Code hook

### Technical Details

- Pure Go implementation with no external API dependencies
- Deterministic output: identical input produces identical output
- Pipe-safe: stdout contains only summary text when redirected
- 361 unit tests covering all algorithms and edge cases

[1.0.0]: https://github.com/gleicon/tldt/releases/tag/v1.0.0
