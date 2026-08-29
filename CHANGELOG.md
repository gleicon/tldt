# Changelog

All notable changes to this project are documented in this file.

## [1.4.1] - 2026-08-29

### Added

- **Phrase and template layer for AI-generated-content detection**: the excess-vocabulary detector now catches multi-word tells that single-token matching cannot, which are among the strongest signals in current research. Two new per-language wordlist fields:
  - `phrases`: literal, case-insensitive substrings matched against the raw text (`it's important to note`, `in the ever-evolving landscape`, `dive into`) — the word tokenizer strips apostrophes and spaces, so these are unreachable as tokens.
  - `templates`: regex patterns for structural tics with variable slots (`not just X, but Y`; `it's not X, it's Y`; pt-BR `não apenas X mas Y`). The `not just X, but Y` construction alone appears in roughly 6% of AI messages in one dataset.
- The phrase signal is strictly additive and monotonic: word `density`/`variety` are unchanged, so word-only text scores identically to prior versions and a matched phrase only raises the score (`phrase_signal = min(0.35, 0.12*phrases + 0.25*templates)`; templates weigh more). Matched tells are reported with the actual matched text, not the pattern.
- `Result` gains `Phrases`, `PhraseSignal`; the `--detect-only --format json` `ai_detection` object gains `phrases` and `phrase_signal`.
- New single-word markers from 2025-2026 studies and community lists, postdating the Kobak academic corpus: English `load-bearing`, `scaffolding`, `boast`, `primarily`, `surpass`, `elevate`, `compelling`, `unwavering`, `garner`, `broader`; Brazilian Portuguese `aprimorar`, `mergulhar`, `panorama`, `cenário`, `otimizar`; plus phrase/template sets for `en`, `pt-BR`, and `es`.

## [1.4.0] - 2026-08-29

### Added

- **Payload decoding**: the injection detector now decodes obfuscated payloads and re-runs pattern/PII detection on the recovered plaintext, reporting the decoded content and the encoding chain (`Provenance`) rather than only flagging that something is encoded. Supported: standard/URL-safe/unpadded base64, base32, hex, `\x`/`\u` escapes, percent-encoding, HTML entities, Unicode Tags block, zero-width binary, ROT13, and reversal. Chains compose up to depth 3, bounded to 10x expansion, 1 MB per chain, and 4 MB per document.
- Closed the short-payload blind spot: encoded tokens under 23 characters, previously unreachable behind the entropy gate, are now decoded and matched (gate retained on the PII path).
- **Role/chat-template markers**: detects `<|im_start|>`, `[INST]`, `### Human:`, `</system>`, fabricated `<function_calls>` blocks, and forged conversation turns.
- **Obfuscation folding**: matches injection phrases through leetspeak/character substitution (`1gn0r3 4ll pr3v10us`), match-time only, scored below a literal match, under the `obfuscated` category.
- **Markdown exfiltration detection**: flags links/images whose URL carries encoded or templated data; keyed on link structure, not a host allowlist.
- **Positional heuristics**: instructions after a whitespace gap, in the document tail, or many-shot forged turns (weak prior).
- **Unicode script-mismatch detection**: sentences whose dominant script differs from the document (script analysis, not language identification; weak prior).
- **Layer corroboration**: two distinct weak layers each scoring >= 0.50 mark input suspicious even when neither crosses the 0.70 threshold; same-layer findings never corroborate. Exposed as `CorroborationFloor` and `Report.CorroboratingLayers`.
- **Detection profiles**: `tldt.HookLayers()` (high-precision subset for the UserPromptSubmit hook) and `tldt.DefaultLayers()` (full CLI set), selectable via `DetectOptions.Layers`, CLI flags, and `~/.tldt.toml` `[security]` keys.
- New CLI flags: `--detect-exfil`, `--detect-positional`, `--detect-script-mismatch`, `--fold-obfuscation`.
- **Document surface extraction** deepened and extended: PDF annotations, AcroForm field values, and white/sub-4pt content-stream text; DOCX footnotes/endnotes, headers/footers, textboxes, tracked-change deletions, and `custom.xml`; XLSX hidden/`veryHidden` sheets and defined names; PPTX speaker notes; HTML CSS-hidden text, JSON-LD, and a differential pass that reports any text present in the raw file but absent from the reader path. New formats: `.ipynb`, Markdown, EPUB, `.eml`, SVG, and image EXIF/IPTC captions.
- New library example `examples/detection` demonstrating detection profiles, decoded-payload provenance, and the corroboration verdict.

### Changed

- Detection now runs on the original input bytes, ahead of all mutating stages (`--sanitize`, `--sanitize-pii`, `--from-html`), so sanitization can no longer destroy a payload before the detector reads it.
- The pattern pass is anchor-prefiltered and folds case once instead of per pattern: on text carrying no injection anchor it skips the regex stage entirely (~3.4 ms vs ~98 ms for a 256 KB input on an Apple M5).
- The statistical outlier pass is capped at 250 sentences with even-spaced sampling to bound its O(n^2) cost; `DetectResult.OutlierScope` reports whether sampling occurred.

### Performance

- Full detection over a 256 KB input (Apple M5): ~133 ms on the hook profile (within the 150 ms budget), ~200 ms with every weak-prior layer enabled.

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
- Comprehensive unit test suite covering all algorithms and edge cases

[1.0.0]: https://github.com/gleicon/tldt/releases/tag/v1.0.0
