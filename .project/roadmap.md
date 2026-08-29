# Roadmap — expanded prompt-injection detection

Source: `SPEC.md` (41 requirements, 45 acceptance criteria). `GRILL.md` holds the
reasoning behind the decisions.

## Measurement floor

- [x] Benchmark harness lands in CI, recording pattern pass, `Analyze`, and full
      `Detect` at 2K/16K/32K/256K. Baseline numbers committed so later tasks show
      movement rather than claims. — AC-28

## Prefactors — make the change easy

Nothing below this line has a latency budget to fit inside until these land.

- [x] Detection runs on original input bytes; `--sanitize`, `--sanitize-pii`, and
      `--from-html` move after detection. Tags-block payload survives `--sanitize`
      and is still reported. — FR-32, AC-32
- [x] Pattern pass lowercases once and drops `(?i)`. Benchmark shows the drop from
      ~97.6 ms toward ~48.6 ms at 256 KB. — FR-31.a
- [x] Rare-literal anchor prefilter skips the regex pass when no anchor is present;
      a coverage test asserts every pattern has a matching anchor. Anchor-free
      256 KB completes within 5 ms. — FR-31.b, FR-31.c, AC-29, AC-29.a, AC-30, AC-30.a
- [x] Outlier pass capped at 400 sentences; report states whether it sampled or
      skipped. Full `Detect` on 256 KB completes within 150 ms. — FR-33, AC-43

## Finding provenance — expand, then migrate

- [x] Expand: `Finding` carries a provenance field and `Category` accepts `exfil`.
      Nothing populates them yet; existing output byte-identical.
- [x] Migrate: stderr formatter and `--format json` render provenance when present;
      `pkg/tldt` re-exports unchanged in signature. — NFR-6, AC-26

## Decoder

Each task is one working path from raw bytes through decode, pattern match, and
reported finding.

- [x] Standard base64 decodes and re-runs pattern and PII detection; finding shows
      decoded text with `base64` provenance. Bounds enforced: depth 3, 10x
      expansion, 1 MB chain, 4 MB document. Decoding also applies to text recovered
      from hidden surfaces. — FR-1, FR-2, FR-11, FR-12, AC-1, AC-9, AC-10, AC-27
- [x] Entropy gate splits: PII path keeps `> 4.5` with byte-identical redaction
      output; injection path accepts 8-char tokens at ≥85% printable over ≥6 bytes.
      A 12-char base64 token decoding to `ignore all` is flagged. — FR-3, FR-4, AC-2, AC-3
- [x] Tier 1 encodings: base64url, unpadded base64, hex and `\x`/`\u` escapes,
      percent-encoding, HTML entities, Tags-block ASCII reconstruction. — FR-5, AC-4, AC-6
- [x] Tier 2 encodings: ROT13/Caesar, reversed strings, base32, zero-width binary. — FR-6, AC-5
- [x] Leetspeak folding runs match-time only against a throwaway copy; findings
      carry `injection-obfuscated`, score below literal, quote original bytes at
      original offsets, and never reach the summarizer. — FR-7, FR-8, FR-9, FR-10, AC-7, AC-8

## Document surfaces

- [x] Differential extraction for HTML: set difference between all text and the
      readability output. CSS-hidden text and JSON-LD reported; a hiding technique
      outside the enumerated list still caught; benign nav chrome stays silent. — FR-16, FR-18, FR-19, AC-15, AC-17, AC-18
- [x] PDF: annotation contents, AcroForm field values, text below 4.0 pt or within
      3% of background colour. — FR-13, AC-11, AC-12
- [x] DOCX: footnotes, endnotes, headers, footers, textboxes, tracked-change
      deletions, `docProps/custom.xml`. — FR-14, AC-13
- [x] XLSX: hidden rows and columns, hidden and `veryHidden` sheets, defined names,
      threaded comments. — FR-15, AC-14
- [x] PPTX speaker notes. — FR-17, AC-16

## Semantic layers

- [x] Role and chat-template markers detected in untrusted text. — FR-21, AC-19
- [x] Markdown exfiltration constructs under the `exfil` category, keyed on link
      structure; an ordinary documentation link produces nothing. — FR-22, AC-20, AC-33
- [x] Positional and structural heuristics: whitespace-gap instructions, tail
      placement, many-shot repetition. — FR-23
- [x] Unicode script-run mismatch; no claim of language identification. — FR-24, AC-31

## Verdict and defaults

Both need at least two layers above to exist before they can be verified.

- [x] `CorroborationFloor` exported at 0.5; two distinct layers at or above it mark
      a report suspicious, while ten findings from one layer do not. Calibration
      test asserts the value against the false-positive corpus. — FR-25, FR-26, FR-27, AC-21, AC-22, AC-40
- [x] Per-mode layer defaults with flags and `~/.tldt.toml` keys. Hook mode runs
      decoder, role markers, patterns, and surfaces only; CLI runs everything;
      false-positive corpus produces zero suspicious verdicts under hook
      defaults. — FR-28, FR-29, FR-30, AC-23, AC-24

## Additional formats

Each is independent and can land in any order, or be dropped without affecting
anything above.

- [x] `.ipynb` cell metadata and stored outputs. — AC-34
- [x] Markdown front-matter and HTML comments. — AC-35
- [x] EPUB OPF metadata. — AC-36
- [x] `.eml` non-displayed headers and `text/html` alternative parts. — AC-37
- [x] Standalone SVG `<title>`, `<desc>`, `<metadata>`. — AC-38
- [x] Image EXIF and IPTC caption fields. — AC-39

## Guarantees

- [x] Non-functional assertions covered by test: no network syscall without
      `--url`, usage log stays counts-only, identical output across runs, examples
      compile unmodified. — NFR-4, NFR-5, NFR-6, NFR-8, AC-25, AC-26, AC-41, AC-42
