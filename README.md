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
| `--algorithm` | `lexrank` | `lexrank`, `textrank`, `graph`, or `ensemble` |
| `--sentences` | `5` | Number of output sentences |
| `--paragraphs` | `0` | Group sentences into N paragraphs |
| `--format` | `text` | `text`, `json`, or `markdown` |
| `--verbose` | off | Print token stats to stderr |
| `--no-cap` | off | Disable 2000-sentence cap (O(n²) warning) |
| `--explain` | off | Print per-sentence scores to stderr (debug) |
| `--rouge <file>` | — | Print ROUGE-1/2/L scores to stderr vs reference file |

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
