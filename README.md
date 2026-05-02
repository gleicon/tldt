# tldt — Too Long, Didn't Tokenize

A CLI tool that summarizes long-form text using graph-based extractive algorithms, so you can paste a fraction of the tokens into an AI coding agent without losing the semantic core.

## What it does

tldt uses LexRank and TextRank to select the most representative sentences from a document — the actual sentences, not generated paraphrases. Feed it a YouTube transcript, a long article, or a wall of documentation, and it returns 5 (or N) sentences that best capture the document.

The token savings are the point: cut a 12,000-token transcript down to ~600 tokens before pasting it into Claude or GPT. No LLM required to summarize; no API calls; no network.

## Install

Prerequisite: Go 1.21+

```bash
go install github.com/gleicon/tldt/cmd/tldt@latest
```

## Usage

### Pipe from stdin

```bash
cat article.txt | tldt
```

### File input

```bash
tldt -f article.txt
```

### Positional argument

```bash
tldt "Paste your long text here..."
```

### YouTube transcript pipeline

```bash
yt-dlp --skip-download --write-auto-sub --sub-format vtt -o transcript youtube.com/watch?v=XXXX
cat transcript.vtt | tldt --sentences 10
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-f` | — | Input file path |
| `--algorithm` | `lexrank` | Summarization algorithm: `lexrank`, `textrank`, `graph` |
| `--sentences` | `5` | Number of output sentences |
| `--paragraphs` | `0` | Group output into N paragraphs (0 = off) |
| `--format` | `text` | Output format: `text`, `json`, `markdown` |
| `--no-cap` | false | Disable 2000-sentence cap (caution: O(n²) on large input) |
| `--explain` | false | Print algorithm diagnostics to stderr (debug) |

## Output formats

### text (default)

Plain sentences, one per line. Pipe-safe.

```
The study found a 40% improvement in latency across all test cases.
Researchers attribute this to the new prefetch strategy introduced in v3.
```

### json

Structured output with compression metadata:

```json
{
  "summary": ["The study found a 40% improvement...", "Researchers attribute this..."],
  "algorithm": "lexrank",
  "sentences_in": 48,
  "sentences_out": 2,
  "chars_in": 14200,
  "chars_out": 183,
  "tokens_estimated_in": 3550,
  "tokens_estimated_out": 45,
  "compression_ratio": 0.987
}
```

### markdown

Blockquote with metadata header:

```markdown
<!-- tldt | algorithm: lexrank | sentences: 2 | compression: 87% -->
> The study found a 40% improvement in latency across all test cases.
>
> Researchers attribute this to the new prefetch strategy introduced in v3.
```

## Algorithms

### LexRank (default)

Graph-based algorithm using TF-IDF cosine similarity. Each sentence is a node; edge weights are cosine similarity between TF-IDF vectors. Sentences are ranked by eigenvector centrality — sentences most similar to the most other sentences rank highest. Best for long articles and dense technical content.

### TextRank

Graph-based algorithm using word-overlap similarity (Jaccard-style). Ranks sentences by PageRank-style iteration with a damping factor. Often favors shorter, more distinct sentences. Best for conversational text and transcripts.

### graph

Delegates to `github.com/didasy/tldr` as a baseline comparison implementation.

## Algorithm comparison

| Property | LexRank | TextRank |
|----------|---------|----------|
| Similarity metric | TF-IDF cosine | Word overlap |
| Ranking method | Eigenvector centrality | PageRank damping |
| Best for | Long articles, dense technical content | Conversational text, transcripts |
| Deterministic | Yes | Yes |

## Build and test

```bash
go build ./...
go test ./...
```

## Token savings

Token estimates use the `chars/4` heuristic and are always written to stderr, never stdout, so they don't break pipes. When running interactively you'll see a line like:

```
tokens: ~3,550 -> ~45 (87% reduction)
```
