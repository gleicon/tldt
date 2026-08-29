# Detection coverage — resolved decisions

Scope: expanding tldt's prompt-injection detection beyond the current pattern +
entropy + outlier layers. Covers encoding techniques, document traps, and
semantic detection layers.

Baseline at time of interview: `internal/detector` ships pattern matching,
base64/hex/control-char anomaly flagging, confusables, PII, and LexRank outlier
scoring. `internal/extractor` reaches HTML (12 surfaces), PDF (XMP, info dict,
JavaScript), DOCX (properties, comments, hidden runs, field codes), XLSX
(properties, comments).

---

## Findings that motivated the plan

**Short base64 payloads are structurally undetectable today.** `base64RE`
(detector.go:241) has a 20-character floor, and Shannon entropy of an n-character
string is capped at log2(n). Any token under 23 characters therefore cannot reach
the `entropy > 4.5` gate in `highEntropyBase64` (detector.go:253). Longer encoded
prose does trip the gate — measured ~5.1 for a typical encoded injection sentence —
so the gap is specifically short payloads, not encoded prose in general.

**The encoding layer never decodes for meaning.** `DetectEncoding` reports "a
base64 blob is present, score 0.75" and stops. `DetectPatterns` never sees the
decoded plaintext.

**One threshold serves two distributions.** `highEntropyBase64` is shared by
`DetectEncoding` and `scanPII`. It was tuned for secret material (random keys,
high entropy) and then reused for injection payloads (English prose, moderate
entropy). This is why the threshold cannot simply be lowered.

**The Tags block is stripped but never read.** `internal/sanitizer` removes
U+E0000–U+E01FF (sanitizer.go:33) and counts the characters, but a tag run is
literally ASCII offset by 0xE0000 — a decodable hidden message currently discarded.

**Extractors are uneven in kind, not just count.** HTML and DOCX reach into
document content (hidden runs, field codes); PDF and XLSX reach only metadata. A
PDF's real injection vectors — white or 1pt text in the content stream, and
`/Annot` contents — are entirely unreached.

---

## Decisions

### 1. Decode payloads and re-run detection on the plaintext

**Chosen:** Decode + rescan, bounded. Decode candidates, re-run
`DetectPatterns`/`DetectPII` on the decoded text, and report the finding as the
decoded content with the encoding chain as provenance (e.g. `base64>utf8`).

**Rationale:** Converts a low-confidence prior ("this looks random") into evidence
("this decodes to an instruction-override phrase"), which is what lets every
downstream threshold decision get simpler.

### 2. Split the entropy gate between the PII and injection paths

**Chosen:** Fork `highEntropyBase64`. `scanPII` keeps `entropy > 4.5`. The
injection path drops the gate, lowers the length floor to roughly 8 characters,
decodes everything base64-shaped, and reports only when the decoded text is
printable and matches a pattern.

**Rationale:** Once a decoded pattern hit is the evidence, the entropy prior is no
longer load-bearing on the injection path — and it is the specific thing blocking
short payloads. Keeping it on the PII path avoids regressing redaction behavior.

### 3. Decoder set spans all three tiers

**Chosen:**
- Tier 1 (high yield, low false positive): base64url (`-_` alphabet) and unpadded
  base64; hex, `\x`, and `\u` escapes decoded rather than merely flagged;
  percent-encoding; HTML entities (`&#105;` / `&#x69;`); Unicode Tags-block ASCII
  reconstruction.
- Tier 2 (cheap transforms): ROT13 and Caesar shifts, reversed strings, base32,
  zero-width binary encoding.
- Tier 3 (fuzzy normalization): leetspeak and character-substitution folding.
  Homoglyph normalization already exists via `DetectConfusables`, which is wired
  into `Analyze` (detector.go:670).

**Rationale:** Tiers 1 and 2 are reversible decodings that self-reject on failure,
so their false-positive cost is near zero. Breadth was preferred over minimal scope.

### 4. Tier 3 folding is match-time only, in its own confidence band

**Chosen:** Fold a throwaway copy of the text solely to run `DetectPatterns`
against. Never emit folded text as an excerpt — always quote original bytes and
offsets. A hit found only after folding scores around 0.6 (versus ~0.85 for a
literal hit) and carries `pattern: "injection-obfuscated"`. Folded text never
enters the encoding category and never reaches the summarizer.

**Rationale:** Lossy folding never self-rejects — `1gn0r3` always "succeeds" —
so it cannot share the decoder's report path without dragging every finding's
confidence down. Document-wide folding was rejected because it corrupts excerpts
and offsets and mangles legitimate non-ASCII text in pt-BR and es, both shipped
`aidetect` languages.

### 5. Decoder bounds

**Chosen:** Maximum 3 decode levels. Abort a chain when cumulative output exceeds
roughly 10x the encoded token or a 1 MB absolute ceiling. Cap total decoded bytes
across the whole document at roughly 4 MB.

**Rationale:** Depth 3 covers observed real-world layering with headroom. The
expansion ratio is what actually stops a decompression-style bomb, and the
document-wide budget is what stops many small blobs from adding up. A wall-clock
deadline was rejected because it makes findings nondeterministic and breaks test
reproducibility.

### 6. Deepen the four existing formats first, then add new ones

**Chosen — deepen:**
- PDF: annotations (`/Annot` contents), AcroForm field values, white and tiny
  content-stream text.
- DOCX: footnotes and endnotes, headers and footers, textboxes
  (`w:txbxContent`), tracked-change deletions (`w:del`), `custom.xml`.
- XLSX: hidden rows and columns, hidden and `veryHidden` sheets, defined names,
  `threadedComments`.
- HTML: CSS-hidden text, JSON-LD script blocks.
- PPTX: added at this stage because speaker notes are invisible when presented and
  fully readable by a model, and it reuses the existing OOXML zip walker.

**Chosen — then add:** `.ipynb` (cell metadata and stored outputs), Markdown
(front-matter and HTML comments), EPUB, `.eml`, standalone SVG, image EXIF/IPTC.

**Rationale:** `extractWTextNodes` (docx.go:135) is already shared between DOCX and
XLSX comments, so each new OOXML part costs a zip-entry name plus one call. The
marginal cost per new surface is near zero; the marginal cost per new format is a
parser. Deepening first buys more coverage per unit of work.

### 7. Differential extraction is the primary rule for visually hidden text

**Chosen:** Dump all text the file contains, dump the text the normal reader path
produces (go-readability for HTML, the text layer for PDF, the body for DOCX), and
report the set difference as a `HiddenSurface`. Enumerated heuristics
(`display:none`, `visibility:hidden`, `opacity:0`, `font-size:0`, off-screen
positioning, `1 1 1 rg` fills, small `Tf` sizes) become a supplement for cases
where both dumps agree but the text is still invisible.

**Rationale:** An enumerated list is a blocklist — it only catches techniques
already written down, and CSS offers unbounded ways to hide text. The attacker
picks the technique, so the rule has to be generic. Noise is not a concern because
`reportHiddenSurfaces` (main.go:466) already prints a surface only when patterns
hit it.

### 8. Semantic detection layers in scope

**Chosen:** All four.
- Role and chat-template markers: `<|im_start|>`, `[INST]`, `<s>`, `### Human:`,
  `Assistant:`, `System:`, `</system>`, fake `<function_calls>` and tool-result
  blocks, and stray code-fence or XML-tag closures that break out of a delimiter.
- Markdown exfiltration constructs (see decision 9).
- Positional and structural heuristics: instructions after a large whitespace gap,
  at the extreme tail of a document, and many-shot repetition of Q/A blocks.
- Language-mismatch: sentences whose language differs from the document's dominant
  language.

**Rationale:** Role markers are a fixed literal set with near-zero false positives
in prose and need no new machinery. The remaining three are weaker priors, accepted
into scope on the understanding that decisions 9 and 10 constrain where they fire.

### 9. Markdown exfiltration checks are input-side, in their own category

**Chosen:** Scan untrusted input for planted exfil constructs — image and link
forms such as `![](http://attacker/?d=<data>)`, autolinks, and reference-style
links pointing at non-allowlisted hosts. New `Category` value `exfil`, reported
like any other finding.

**Rationale:** A document containing an exfil construct is seeding the payload the
model will later echo, so input-side scanning stays inside tldt's existing
input-only contract. Output scanning is where exfil is actually caught and is the
higher-value placement, but it is a separate product surface with a different
threat model and different hook plumbing — deliberately deferred, not dismissed.

### 10. Verdict aggregation: max plus independent-layer corroboration

**Chosen:** `Suspicious` keeps `maxScore > DefaultDetectionThreshold` as the
headline trigger, and gains a second: it also fires when two or more distinct
layers each exceed a corroboration floor of roughly 0.5. Weak signals compound
across layers, never within a single layer.

**Rationale:** Pure max (detector.go:679) is sound only while every layer is
high-precision. With weak-prior layers added, three independent 0.6 signals — an
obfuscated pattern hit, tail placement, a language-mismatched sentence — is far
more alarming than any one alone, yet max would report 0.6 and leave the verdict
clean. Restricting compounding to distinct layers prevents ten positional hits from
manufacturing a verdict. Noisy-OR was rejected because it changes the meaning of
every existing score and the per-layer scores were never calibrated as
probabilities.

### 11. Hook mode defaults to high-precision layers only

**Chosen:** `--hook-output` enables the decoder (tiers 1 and 2), role and template
markers, existing patterns, and document surfaces. Positional heuristics,
language-mismatch, and leetspeak folding are opt-in via config or flag. Full CLI
runs enable everything.

**Rationale:** A false positive in hook mode injects a warning envelope into a real
user's prompt on every turn, so the false-positive cost is paid constantly while
the false-negative cost is paid rarely. A noisy advisory is one the user learns to
ignore, which costs the high-precision layers their value too. Raising the hook
threshold instead was rejected because it interacts opaquely with the new
corroboration rule.

---

## Composition note

Hidden surfaces already route through `tldt.Detect` (main.go:454). Once the decoder
lives inside `Detect`, base64 planted in a PDF annotation or an HTML comment is
decoded and pattern-matched with no additional wiring — extraction and decoding
stay orthogonal layers meeting at a single call site.
