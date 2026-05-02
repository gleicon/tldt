# tldt — Too Long, Didn't Tokenize

Pipe long text in, get a short summary out. No LLM calls. No API keys. No token costs.

```
cat transcript.txt | tldt
~48,000 → ~2,100 tokens saved (96% reduction)
```

Uses LexRank and TextRank — graph-based extractive summarization that picks the most
representative sentences from the original text. Output is always exact quotes, never paraphrased.

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
```

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-f <file>` | — | Read from file |
| `--algorithm` | `lexrank` | `lexrank`, `textrank`, or `graph` |
| `--sentences` | `5` | Number of output sentences |
| `--paragraphs` | `0` | Group sentences into N paragraphs |
| `--format` | `text` | `text`, `json`, or `markdown` |
| `--verbose` | off | Print token stats to stderr |
| `--no-cap` | off | Disable 2000-sentence cap (O(n²) warning) |
| `--explain` | off | Print per-sentence scores to stderr (debug) |

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

Token estimates use `chars / 4`. Stats go to stderr — they never appear on stdout and never break pipes. Enable with `--verbose`:

```bash
tldt -f long-doc.txt --verbose
# stderr: ~12,400 → ~534 tokens (96% reduction)
```

Stats are suppressed by default so scripts that redirect stderr stay clean.

---

## Algorithms

| Algorithm | How it works | Best for |
|-----------|-------------|----------|
| `lexrank` | TF-IDF cosine similarity + power iteration | Articles, reports, dense prose |
| `textrank` | Word overlap + PageRank damping | Transcripts, conversational text |
| `graph` | Bag-of-words (didasy/tldr baseline) | Quick baseline comparison |

---

## Build & test

```bash
make build        # compile to ./tldt
make test         # run all tests
make install      # install to GOPATH/bin
make deps         # tidy + verify modules
make clean        # remove binary
```
