# tldt — Too Long, Didn't Tokenize

Pipe long text in, get a short summary out. No LLM calls. No API keys. No token costs.

```
cat transcript.txt | tldt
~48,000 → ~2,100 tokens saved (96% reduction)
```

Graph-based extractive summarization: picks the most representative sentences from the original
text. Output is always exact quotes, never paraphrased.

---

## Install

```bash
go install github.com/gleicon/tldt/cmd/tldt@latest
```

Or build from source:

```bash
git clone https://github.com/gleicon/tldt
cd tldt
make install
```

---

## Usage

```bash
# stdin pipe
cat article.txt | tldt

# file
tldt -f article.txt

# inline text
tldt "paste your text here"

# show token savings on stderr
tldt -f article.txt --verbose

# choose algorithm
tldt -f article.txt --algorithm ensemble

# evaluate against a reference summary (ROUGE)
tldt -f article.txt --rouge reference.txt

# structured output
tldt -f article.txt --format json
tldt -f article.txt --format markdown
```

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-f <file>` | — | Read from file |
| `--url <url>` | — | Fetch webpage and summarize extracted text |
| `--algorithm` | `lexrank` | `lexrank`, `textrank`, `graph`, or `ensemble` |
| `--sentences` | `5` | Number of output sentences |
| `--level` | — | Named preset: `aggressive` (3), `standard` (5), `lite` (10) |
| `--paragraphs` | `0` | Group sentences into N paragraphs |
| `--format` | `text` | `text`, `json`, or `markdown` |
| `--verbose` | off | Print token stats to stderr |
| `--no-cap` | off | Disable 2000-sentence cap (O(n²) warning) |
| `--explain` | off | Print per-sentence scores to stderr (debug) |
| `--rouge <file>` | — | Print ROUGE-1/2/L scores to stderr vs reference file |
| `--sanitize` | off | Strip invisible Unicode and NFKC-normalize before summarizing |
| `--detect-injection` | off | Report prompt injection patterns and encoding anomalies to stderr |
| `--injection-threshold` | `0.99` | Outlier score [0,1] above which sentences are flagged |
| `--detect-pii` | off | Report PII/secrets (emails, API keys, tokens, private keys, JWTs, SSNs, credit cards) to stderr |
| `--sanitize-pii` | off | Redact PII/secrets (detected patterns plus high-entropy key material) with `[REDACTED:<type>]` before summarizing |
| `--from-html` | off | Convert HTML input to Markdown before summarizing |
| `--install-skill` | off | Install tldt skill and UserPromptSubmit hook |
| `--skill-dir <dir>` | — | Override skill install directory |
| `--target <app>` | — | Install target: `claude`, `codex`, `cursor`, `opencode`, `agents`, or `all` |

> `--sanitize-pii` favors over-redaction: its high-entropy gate can also redact dense base64 that is not secret (content hashes, signatures, key fingerprints) as `[REDACTED:secret]`. Use `--detect-pii` to report matches without modifying text.

---

## Output formats

**Text** (default — pipe-safe, stdout only):
```
The researchers found a 40% improvement in efficiency...
Further tests confirmed the results held across platforms...
```

**JSON** (`--format json`):
```json
{
  "summary": ["sentence one", "sentence two"],
  "algorithm": "lexrank",
  "sentences_in": 142,
  "sentences_out": 5,
  "chars_in": 9840,
  "chars_out": 431,
  "tokens_estimated_in": 2460,
  "tokens_estimated_out": 107,
  "compression_ratio": 0.956
}
```

**Markdown** (`--format markdown`):
```markdown
<!-- tldt | algorithm: lexrank | sentences: 5 | compression: 95% -->
> The researchers found a 40% improvement...
```

---

## Token savings

Token estimates use `chars / 4`. Stats go to stderr — never appear on stdout, never break pipes.
Enable with `--verbose`:

```bash
tldt -f long-doc.txt --verbose
# stderr: ~12,400 → ~534 tokens (96% reduction)
```

Stats are suppressed by default so scripts that redirect stderr stay clean.

### Usage log

Each summarization appends a **counts-only** record (timestamp + token counts, never any content) to `~/.tldt/usage.jsonl`, and `tldt stats` reports the totals:

```bash
tldt stats            # aggregate token savings
tldt stats --daily    # per-day breakdown
tldt stats --json     # machine-readable
tldt stats --reset    # clear the log
```

Logging is on by default; tldt prints a one-time notice when it first creates the file. The log only ever grows — clear it with `tldt stats --reset`. To opt out entirely, add to `~/.tldt.toml`:

```toml
[stats]
enabled = false
```

---

## Algorithms

| Algorithm | How it works | Best for |
|-----------|-------------|----------|
| `lexrank` | TF-IDF cosine similarity + eigenvector centrality | Articles, reports, dense prose |
| `textrank` | Word overlap + PageRank damping | Transcripts, conversational text |
| `graph` | Bag-of-words baseline (didasy/tldr) | Quick baseline comparison |
| `ensemble` | Average of LexRank + TextRank scores | General use, balanced results |

Both `lexrank` and `textrank` implement `--explain` for per-sentence score diagnostics.
`ensemble` combines both score vectors before selecting sentences.

---

## ROUGE evaluation

Measure summary quality against a human-written reference:

```bash
tldt -f article.txt --rouge human_summary.txt --sentences 5
# stderr:
# rouge-1  P=0.5200 R=0.4800 F1=0.4990
# rouge-2  P=0.2100 R=0.1900 F1=0.1995
# rouge-l  P=0.4800 R=0.4400 F1=0.4590
```

ROUGE scores are always printed to stderr and never affect stdout output.

---

## URL input

Fetch and summarize a webpage directly — no manual copy-paste:

```bash
tldt --url https://example.com/article
tldt --url https://example.com/article --sentences 3 --format json
```

HTML boilerplate (nav, ads, footers) is stripped using the readability algorithm. Redirects are followed automatically. Fetch errors (4xx/5xx, timeouts) exit non-zero with a message to stderr.

**HTML comment injection scanning**: when `--detect-injection` is combined with `--url`, tldt also scans all `<!-- HTML comment -->` nodes from the raw HTML. Readability strips comments from visible text, making them an invisible channel for indirect prompt injection (OWASP LLM01). This scanning layer fires even on JS SPAs where readability finds no body text.

```bash
tldt --url https://example.com/page --detect-injection
# stderr: injection-detect[html-comment 0]: WARNING — comment flagged as suspicious
#           [pattern] social-engineering (score=0.85): You have only one attempt
```

---

## Config file

Persist your preferred defaults in `~/.tldt.toml`:

```toml
algorithm = "ensemble"
sentences = 7
format    = "text"
level     = "standard"
```

CLI flags always override the config file. Missing or malformed TOML silently falls back to built-in defaults — never an error.

**Named presets** (shorter than `--sentences`):

| Preset | Sentences | Use case |
|--------|-----------|----------|
| `aggressive` | 3 | Dense compression, key takeaways only |
| `standard` | 5 | Default balance |
| `lite` | 10 | Light compression, more context |

```bash
tldt -f article.txt --level aggressive   # 3 sentences
```

---

## AI assistant skill integration

Install tldt as a skill across AI coding assistants. The default run auto-detects every assistant with an existing config directory and installs to all of them:

```bash
tldt --install-skill                    # auto-detect and install everywhere
tldt --install-skill --target claude    # one assistant (auto-creates its dir)
tldt --install-skill --skill-dir /path  # write the skill to an explicit dir
```

After install, use `/tldt <url | file | text>` inside the assistant to summarize inline.

| `--target` | Installs |
| --- | --- |
| `claude` | skill + advisory hook + `settings.json` |
| `codex` | plugin (skill + advisory hook) via a local marketplace |
| `opencode` | skill + advisory plugin |
| `cursor` | skill only |
| `agents` | skill only |
| `all` | every assistant above (default) |

**Advisory hook**: when installed, a `UserPromptSubmit` hook runs local injection/PII detection on each prompt and adds a security warning to the AI context only when the input is flagged. The warning carries **metadata only** (finding kind, pattern, location) — never the matched text, so a flagged injection payload is never echoed back into the model's context. It never summarizes, replaces, or blocks the prompt. The Claude/Codex hook is a two-line shell script that delegates to `tldt --hook-output` (no `jq` or `python3` dependency); OpenCode gets the equivalent advisory plugin, which reads tldt's structured `--detect-only --format json` output.

**Alternate Claude locations**:

```bash
tldt --install-skill --config-dir ~/alt/.claude   # override the Claude config base
tldt --install-skill --project                     # repo-local ./.claude/ install
```

`--config-dir` takes precedence over `$CLAUDE_CONFIG_DIR`, then the `~/.claude` default. `--project` installs into the current repo and registers the hook in `.claude/settings.local.json` via `$CLAUDE_PROJECT_DIR`, so no machine-specific path is committed.

---

## Prompt injection defense

When using tldt to pre-process untrusted content before it enters an AI context, enable the defense layers:

```bash
# Sanitize invisible Unicode and NFKC-normalize, then summarize
cat untrusted.txt | tldt --sanitize

# Detect injection patterns, encoding anomalies, and statistical outliers
cat untrusted.txt | tldt --detect-injection

# Both together (recommended for untrusted input)
cat untrusted.txt | tldt --sanitize --detect-injection
```

All detection output goes to **stderr only** — stdout always contains just the summary. Detection is **advisory**: tldt never blocks or modifies input without `--sanitize`.

**What gets detected:**

| Layer | Detects |
|-------|---------|
| Pattern | Direct overrides (`ignore all previous instructions`), role injection, delimiter injection (`[INST]`, `<system>`), jailbreaks (DAN mode), exfiltration requests, social engineering |
| Social engineering | Header manipulation (`append … User-Agent header`, `add a custom header`), urgency threats (`you have only one attempt`), punishment threats (`flagged as malicious`, `IP banned`) |
| Encoding | Base64 payloads (entropy-gated), `\x`-escaped hex sequences, raw hex strings, abnormal control character density |
| Outlier | Sentences statistically dissimilar from document neighbors (off-topic injection) — uses LexRank cosine similarity matrix |
| Confusable | Cross-script homoglyphs: Cyrillic `а` → Latin `a`, Greek `ο` → Latin `o`, etc. — UTS#39 lookup (Unicode 17.0, ~700 entries). NFKC normalization alone cannot collapse these; they require the lookup table. |
| HTML comments | `<!-- HTML comments -->` — stripped by readability; scanned separately on `--url` fetches |
| HTML attributes | `placeholder`, `alt`, `aria-label`, `title`, `data-*` on any element — invisible to readability |
| HTML meta | `<meta name/property content="">` — head tags stripped by readability |
| HTML noscript | `<noscript>` fallback content |
| HTML hidden inputs | `<input type="hidden" value="">` |
| HTML textarea prefill | Pre-filled `<textarea>` content |
| DOCX surfaces | Document properties (`dc:title`, `dc:subject`, `dc:description`, keywords), inline comments, hidden text runs (`w:hidden`), field codes (`w:instrText`) — via `-f file.docx --detect-injection` |
| XLSX surfaces | Document properties, cell comments — via `-f file.xlsx --detect-injection` |
| PDF surfaces | XMP metadata packet, Info dictionary (`/Title`, `/Keywords`, `/Subject`, `/Author`), JavaScript actions (`/JS`) — via `-f file.pdf --detect-injection` |

Tune the outlier threshold:

```bash
cat doc.txt | tldt --detect-injection --injection-threshold 0.90   # stricter
```

---

## Security

tldt's architecture provides structural immunity to three OWASP LLM Top 10 2025 categories:

**LLM04 — Model Denial of Service**: tldt is a pure CLI binary. There is no model server, no inference endpoint, and no shared resource that a caller can exhaust. Each invocation is an isolated process that exits when summarization completes — no pooling, no queuing, no per-request GPU allocation.

**LLM08 — Vector and Embedding Weaknesses**: tldt uses no embeddings and no vector store. Similarity scores are computed from raw TF-IDF cosine similarity and word-overlap ratios on the input text alone. There is no persistent index to poison, no retrieval path to manipulate, and no external knowledge base to corrupt.

**LLM09 — Misinformation**: tldt is a purely extractive summarizer. Every sentence in the output is copied verbatim from the source document — no paraphrasing, no generation, no inference. Hallucination is structurally impossible: if a sentence appears in the summary, it existed in the input.

For full OWASP LLM Top 10 2025 coverage including LLM01 (prompt injection defense), LLM02 (insecure output handling), LLM05 (supply chain), and LLM10 (model theft), see [docs/security.md](docs/security.md).

---

## Build & test

```bash
make build            # compile to ./tldt
make test             # run all tests
make test-verbose     # tests with output
make test-cover       # unit + subprocess coverage report
make test-race        # run with race detector
make bench            # run benchmarks
make install          # install to GOPATH/bin
make deps             # tidy + verify modules
make lint             # go vet
make clean            # remove binary
```
