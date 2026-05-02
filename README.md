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
| `--injection-threshold` | `0.85` | Outlier score [0,1] above which sentences are flagged |
| `--print-threshold` | off | Print configured hook token threshold to stdout and exit |
| `--install-skill` | off | Install tldt Claude Code skill and UserPromptSubmit hook |

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

---

## Config file

Persist your preferred defaults in `~/.tldt.toml`:

```toml
algorithm = "ensemble"
sentences = 7
format    = "text"
level     = "standard"

[hook]
threshold = 2000   # tokens; auto-trigger hook fires above this
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

## Claude Code skill integration

Install tldt as a Claude Code skill so you can invoke it directly inside a session:

```bash
tldt --install-skill                    # auto-detect Claude Code install dirs
tldt --install-skill --target claude    # specific app only
tldt --install-skill --skill-dir /path  # explicit directory
```

After install, use `/tldt <text>` inside Claude Code to summarize inline.

**Auto-trigger hook**: when installed, the hook fires automatically when your paste or file input exceeds a token threshold (default: 2000). The summarized version enters the AI context instead of the raw text, with token savings reported to stderr.

```bash
tldt --print-threshold   # print current threshold (from config) and exit
```

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
| Pattern | Direct overrides (`ignore all previous instructions`), role injection, delimiter injection (`[INST]`, `<system>`), jailbreaks (DAN mode), exfiltration requests |
| Encoding | Base64 payloads (entropy-gated), `\x`-escaped hex sequences, raw hex strings, abnormal control character density |
| Outlier | Sentences statistically dissimilar from document neighbors (off-topic injection) — uses LexRank cosine similarity matrix |

Tune the outlier threshold:

```bash
cat doc.txt | tldt --detect-injection --injection-threshold 0.90   # stricter
```

**What is NOT detected:** Cross-script homoglyph substitution (Cyrillic `а` vs Latin `a`). NFKC normalization handles compatibility variants (fullwidth, ligatures) but not confusable cross-script characters. For that threat model, a UTS#39 confusables database is required.

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
