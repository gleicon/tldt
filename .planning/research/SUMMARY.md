# Research Summary — tldt

## Stack Recommendation

**Implement LexRank and TextRank natively in Go.** (~150-200 lines each)

The existing `github.com/JesusIslam/tldr` (= `github.com/didasy/tldr`, same library) uses Hamming/Jaccard similarity — this is NOT correct LexRank. True LexRank requires IDF-modified cosine similarity. Implementing natively gives correctness + full parameter control.

Keep `github.com/DavidBelicza/TextRank` as an optional third algorithm backend (`--algorithm textrank-lib`) for comparison.

### Go Dependencies
```
github.com/didasy/tldr          # keep as "graph" algorithm baseline option
github.com/DavidBelicza/TextRank # legitimate TextRank impl
```
No other NLP deps needed. TF-IDF, cosine similarity, power iteration all ~50 lines each.

## Algorithm Summary

### LexRank (primary, recommended)
- Build TF-IDF vectors per sentence; IDF computed across the corpus
- Compute n×n cosine similarity matrix with IDF weighting
- Apply threshold (default 0.1) to binarize → "Discrete LexRank"
- Run power iteration until L1 delta < 1e-6 (typically 10-30 iters)
- Return top-K sentences in original document order
- **Best for**: YouTube transcripts, technical docs, noisy/long text

### TextRank (secondary)
- Word overlap similarity normalized by log sentence lengths
- No IDF needed — cheaper computation
- PageRank update with damping factor d=0.85
- **Best for**: Short coherent prose, news articles, narrative text

## CLI Architecture

```
stdin / file / positional arg
        ↓
   preprocess (unicode normalize, strip BOM, URL protection)
        ↓
  sentence tokenize (period-boundary with abbreviation list)
        ↓
  algorithm: lexrank | textrank | graph (didasy/tldr)
        ↓
  rank sentences → select top-K
        ↓
  reorder by original position
        ↓
  stdout: summary text (ONLY — pipe safe)
  stderr: token estimate, compression ratio (if TTY or --stats)
```

## Critical UX Rule
When `isatty(stdout) == false`: stdout gets ONLY summary text. No headers, no stats, no decoration.
Stats/metrics go to stderr always (visible in terminal, invisible in pipes).

## Token Estimation
`tokens_estimated ≈ len(text) / 4` — sufficient (±15%) for English prose. Prefix with `~`.

## Test Data Needed
Replace existing Portuguese short-form test data with:
1. English Wikipedia article (medium, ~800 words)
2. Raw YouTube transcript (no punctuation, timestamp artifacts)
3. Long-form technical article (3000+ words, performance baseline)
4. Edge case: < 5 sentences (boundary condition)
5. Code-heavy technical doc (abbreviations, URLs, version numbers)

## Watch Out For
- Sort map keys before TF-IDF vector computation → deterministic output
- Cap similarity matrix at 2000 sentences default (O(n²) memory/time)
- Abbreviation list for sentence tokenizer (Dr., Mr., vs., etc., i.e., e.g.)
- BOM and non-breaking spaces in YouTube/web-scraped text
- Fewer input sentences than `--sentences N` → return all, no error
