# Features Research — CLI Summarization Tools

**Project:** tldt (Too Long, Didn't Tokenize)
**Researched:** 2026-05-01
**Confidence:** HIGH (stack + patterns), MEDIUM (benchmarks/metrics)

---

## Table Stakes (must have)

Features that every developer CLI summarization tool provides. Absence makes the tool feel broken or unusable.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| stdin pipe input | Unix philosophy; every developer expects `cat file.txt \| tldt` | Low | Without this, tool is not composable |
| File input via flag | Standard fallback when pipe is not available | Low | `-f file.txt` or `--file` |
| Tunable sentence count | The one control users need most; compression depth | Low | `--sentences N`; default 5 is appropriate |
| Plain text output to stdout | Output must be pipeable with no decoration | Low | Any spinners, progress indicators, or ANSI color must go to stderr only when TTY is detected |
| Exit codes: 0 on success, non-zero on failure | Scripts depend on this; missing it silently breaks automation | Low | Errors to stderr, never to stdout |
| `--help` flag with usage examples | Discoverable; `sumy --help` sets this expectation | Low | Show at minimum: input methods, key flags, one example invocation |
| Process text in under 2 seconds for typical articles | Developers abandon slow CLI tools; extractive summarization is local/deterministic | Low | LexRank on a 10k-word article should complete in well under 1s |

**Key reference:** sumy CLI (`sumy lex-rank --length=10 --url=...`) established the pattern for algorithm selection as a positional or flag argument. Simon Willison's `llm` CLI tool established the pattern for piping data between tools in AI workflows.

---

## Differentiators (nice to have)

Features that set tldt apart from raw Python library invocations. These are what make a CLI tool feel purpose-built for AI developer workflows.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Token count estimate (before/after) | Directly answers "how much did I save?" for AI users; no other extractive summarization CLI shows this | Low-Med | Approximation is fine: 1 token ≈ 4 chars or use tiktoken-style heuristic; show as `~12,000 tokens → ~1,400 tokens (88% reduction)` |
| Compression ratio in output metadata | Reinforces the tool's core value on every run | Low | Derive from input/output char counts |
| `--format json` flag | Enables scripting and automation; JSON with `summary`, `sentences_in`, `sentences_out`, `tokens_estimated_in`, `tokens_estimated_out`, `compression_ratio`, `algorithm` | Low | Default format stays plain text |
| `--format markdown` flag | Direct paste into AI chat or docs; wraps summary in blockquote or section | Low | Useful when summary feeds a Markdown document |
| `--algorithm lexrank\|textrank` flag | Expert users can tune; LexRank is better for news/technical docs, TextRank for narrative | Med | Default: lexrank; expose tradeoff in help text |
| Sentence count in plain-text footer (stderr only) | Quick feedback: "Summarized to 5 sentences (was 42)" confirms the tool ran | Low | Only when stdout is a TTY; suppress when piped |
| Positional text argument | `tldt "long text..."` for one-liners in shell scripts | Low | Third input method after stdin and -f |
| `--paragraphs N` flag | Groups output sentences into readable paragraphs | Low | Cosmetic but improves human readability |
| URL input mode | `tldt --url https://...` to fetch and summarize an article inline | Med | sumy supports this; useful for research workflows |
| Quiet mode (`-q` / `--quiet`) | Suppress all stderr output for fully silent operation in scripts | Low | Pair with `--format json` for zero-noise automation |

---

## Anti-features (explicitly avoid)

Things that similar tools do that tldt must not do, given its target use case (AI developer workflows, Unix composability).

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Mixing diagnostic output into stdout | Breaks pipes; any tool receiving tldt's output will get corrupted data | All logs, progress, and diagnostics to stderr; only the summary to stdout |
| Spinners/animations when stdout is piped | Turns progress bars into literal garbage in CI logs and when piped to `pbcopy` or `llm` | Detect `isatty(stdout)`; never animate when stdout is not a terminal |
| Requiring network access or external APIs | Antithetical to purpose; the whole point is local, offline, free | LexRank and TextRank are pure graph algorithms on the input text |
| LLM-based summarization | Defeats the purpose; tldt saves tokens, not spends them | Extractive only: select actual sentences from the source |
| Config files required to run | Friction; existing `resumator.conf` approach is wrong for a CLI | All config via flags with sensible defaults; no config file required |
| Abstracted output that loses sentence identity | For AI contexts, extracted sentences must be coherent and self-contained | LexRank's centrality scoring naturally produces this; preserve sentence boundaries |
| Hard-coded sentence counts | Users have different needs; a README needs 3 sentences, a YouTube transcript needs 10 | Always expose `--sentences N` |
| Verbose JSON with internal scoring data by default | Noise for most users | Expose scores only with `--verbose` or `--debug` |
| Interactive prompts | CLIs that ask questions mid-pipeline break automation | All inputs via flags; fail fast with a clear error message if input is missing |

---

## Output Formats

Based on how developers use tools in AI assistant workflows (piping to `pbcopy`, feeding to `llm`, dropping into Markdown docs).

| Format | Flag | Stdout Content | When to Use |
|--------|------|---------------|-------------|
| Plain text (default) | (none) | Summary sentences joined by newlines | Human reading, piping to clipboard, piping to `llm` |
| JSON | `--format json` | Structured object (see schema below) | Scripting, automation, logging pipelines |
| Markdown | `--format markdown` | Summary in a `> blockquote` or `## Summary` section | Direct paste into AI chat, PR descriptions, docs |

### JSON Schema (recommended)

```json
{
  "summary": "The extracted summary text.",
  "algorithm": "lexrank",
  "sentences_in": 42,
  "sentences_out": 5,
  "chars_in": 12840,
  "chars_out": 1120,
  "tokens_estimated_in": 3210,
  "tokens_estimated_out": 280,
  "compression_ratio": 0.87
}
```

Token estimation approach: divide character count by 4 (GPT/Claude tokenizer heuristic). This is an approximation, but close enough for the "how much did I save?" use case. Label it clearly as `tokens_estimated_*` to set expectations.

**Stderr-only output** (only when stdout is TTY, always suppressed when piped):
```
Summarized 42 sentences → 5 sentences (~87% token reduction)
```

---

## Integration Patterns

How developers use CLI summarization tools in real workflows. These patterns should drive UX decisions.

| Pattern | Example | Requirements |
|---------|---------|-------------|
| Pipe to clipboard | `cat transcript.txt \| tldt \| pbcopy` | stdout must be clean plain text only |
| Pipe to AI CLI | `cat article.txt \| tldt \| llm "explain this"` | stdout must be clean; exit 0 on success |
| Pipe to file | `tldt -f long-doc.txt > summary.txt` | stdout clean; no decoration |
| Shell substitution | `llm "summarize: $(tldt -f notes.txt)"` | stdout clean; no trailing newlines or metadata |
| JSON for scripting | `tldt --format json -f doc.txt \| jq .compression_ratio` | Valid JSON to stdout; nothing else |
| Process multiple files | `for f in transcripts/*.txt; do tldt -f "$f" > "summaries/$(basename $f)"; done` | Reliable exit codes; no interactive prompts |
| Claude Code custom command | `/summarize` slash command calling `tldt` under the hood | Quiet mode important; MCP-compatible patterns |
| CI/CD pipeline | Summarize changelogs or docs in build scripts | No TTY available; pure text output required |

**Critical rule derived from patterns:** When `isatty(stdout)` is false (i.e., piped), output ONLY the summary text to stdout. All metadata, progress, and diagnostics must go to stderr. This is the single most important UX constraint.

---

## Test Data Recommendations

Covering the realistic input types tldt will receive from AI developer workflows.

| Category | Source | Characteristics | Why Include |
|----------|--------|----------------|-------------|
| News article (English) | Wikipedia featured articles (e.g., "Attention (machine learning)", "Large language model") | 800–3000 words, well-structured paragraphs | Standard NLP benchmark format; known gold summaries available |
| YouTube transcript (technical talk) | Raw transcript with timestamps stripped; no punctuation normalization | Run-on sentences, no paragraph breaks, speaker artifacts | Realistic worst-case for sentence tokenization |
| Technical documentation | Go stdlib docs, README files, RFC excerpts | Structured headers, code blocks mixed with prose | Tests handling of non-prose text |
| Short text (< 100 words) | Any short paragraph | Fewer sentences than `--sentences N` default | Must not crash or produce empty output |
| Non-English text | The existing `body.txt` test data (Brazilian Portuguese news) | Non-ASCII characters, different stop-word set | Regression coverage; LexRank is language-agnostic for basic cases |
| Long-form content (10k+ words) | Wikipedia article on "Natural language processing" | Performance baseline; memory usage test | Validates sub-2-second performance target |

**Recommended corpus structure:**
```
test-data/
  short-article-en.txt       (< 200 words)
  medium-article-en.txt      (800-1500 words, Wikipedia-style)
  long-article-en.txt        (3000+ words)
  youtube-transcript-en.txt  (raw, no punctuation normalization)
  technical-doc-en.txt       (mixed prose + code references)
  article-pt.txt             (existing body.txt — keep it)
```

**Gold summaries:** For unit tests, manually write expected 3-sentence summaries for medium-article-en.txt so regressions can be detected without ROUGE tooling.

---

## Quality Metrics

How extractive summarization quality is measured. Relevant for benchmarking algorithm changes and validating LexRank implementation.

### ROUGE (primary metric for extractive summarization)

ROUGE (Recall-Oriented Understudy for Gisting Evaluation) is the standard evaluation metric for summarization. It measures n-gram overlap between generated summary and a reference (human-written) summary.

| Variant | What It Measures | When to Use |
|---------|-----------------|-------------|
| ROUGE-1 | Unigram overlap (individual word matches) | Quick sanity check; high for extractive since source words are preserved |
| ROUGE-2 | Bigram overlap | Better signal for extractive quality than ROUGE-1 |
| ROUGE-L | Longest Common Subsequence | Best for sentence-level extractive summarization; preferred for tldt |
| ROUGE-Lsum | LCS computed per sentence then averaged | Recommended for extractive tasks per research literature |

**Practical note for tldt:** Since extractive summarization copies sentences verbatim, ROUGE scores will be naturally high compared to abstractive models. Use ROUGE primarily to catch regressions when changing algorithms or scoring functions, not as an absolute quality target.

### Secondary metrics (informational, not required)

| Metric | What It Measures | Tooling |
|--------|-----------------|---------|
| Compression ratio | (chars_out / chars_in); lower = more aggressive compression | Compute in-tool |
| Coverage | What fraction of the original topics appear in summary | Qualitative review |
| Token reduction % | (1 - tokens_out / tokens_in) * 100 | Compute in-tool; the metric users care most about |
| Sentence position bias | Whether high-scoring sentences cluster at document start | Run on test corpus; LexRank should be position-neutral |

### Practical benchmark approach for tldt

1. Create a small test corpus (5–10 articles with human reference summaries)
2. Run `tldt --algorithm lexrank` and `tldt --algorithm textrank` on each
3. Compute ROUGE-L F1 against reference summaries (use `rouge-score` Python package or implement in Go tests)
4. Assert ROUGE-L F1 > 0.3 on English news articles as a regression gate
5. Track token reduction ratios across the test corpus to validate the compression value proposition

**CNN/DailyMail dataset** (287k article-summary pairs) is the canonical research benchmark but is overkill for a CLI tool. Use 10–20 articles from it as spot-check data, not full evaluation.

---

## Sources

- [sumy — GitHub (miso-belica/sumy)](https://github.com/miso-belica/sumy) — CLI flags, algorithm options, active maintenance (latest release Feb 2026)
- [sumy — PyPI](https://pypi.org/project/sumy/) — Installation and usage reference
- [PyTLDR — GitHub (jaijuneja/PyTLDR)](https://github.com/jaijuneja/PyTLDR) — Predecessor pattern; Python 2 only, abandoned
- [steipete/summarize — GitHub](https://github.com/steipete/summarize) — Modern pattern: transcript-first, metrics output, cache-aware, streaming
- [simonw/llm — GitHub](https://github.com/simonw/llm) — Gold standard for AI-aware CLI tool UX (stdin piping, plugin architecture)
- [Command Line Interface Guidelines — clig.dev](https://clig.dev/) — Authoritative UX rules (stdout/stderr separation, isatty, exit codes)
- [UX patterns for CLI tools — Lucas F. Costa](https://lucasfcosta.com/2022/06/01/ux-patterns-cli-tools.html) — Anti-patterns: mixing stdout/stderr, non-pipeable output
- [ROUGE metric — Wikipedia](https://en.wikipedia.org/wiki/ROUGE_(metric)) — Metric definitions
- [Introduction to Text Summarization with ROUGE Scores — Towards Data Science](https://towardsdatascience.com/introduction-to-text-summarization-with-rouge-scores-84140c64b471/) — ROUGE variants and when to use each
- [CNN/DailyMail dataset — Hugging Face](https://huggingface.co/datasets/abisee/cnn_dailymail) — Benchmark dataset reference
- [TLDR Go library — GitHub (JesusIslam/tldr)](https://github.com/JesusIslam/tldr) — Current implementation (LexRank-based, in use by project)
- [NLP Progress — Summarization](http://nlpprogress.com/english/summarization.html) — SOTA benchmark context
