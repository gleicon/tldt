# Phase 2: Algorithms - Research

**Researched:** 2026-05-01
**Domain:** Extractive summarization algorithms — LexRank (TF-IDF + cosine + power iteration) and TextRank (word overlap + PageRank) in native Go
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Extend `internal/summarizer/` — add `lexrank.go`, `textrank.go`, `tokenizer.go`, and `summarizer.go` (dispatch). No sub-packages.
- **D-02:** Use a `Summarizer` interface with a registry:
  ```go
  type Summarizer interface {
      Summarize(text string, n int) ([]string, error)
  }

  func New(algo string) (Summarizer, error) {
      switch algo {
      case "lexrank":  return &LexRank{}, nil
      case "textrank": return &TextRank{}, nil
      case "graph":    return &Graph{}, nil
      default: return nil, fmt.Errorf("unknown algorithm: %s", algo)
      }
  }
  ```
- **D-03:** Keep stdlib `flag` for CLI. Add three flags to `cmd/tldt/main.go`:
  ```go
  algorithm := flag.String("algorithm", "lexrank", "algorithm: lexrank|textrank|graph")
  sentences := flag.Int("sentences", 5, "number of output sentences")
  paragraphs := flag.Int("paragraphs", 0, "group sentences into N paragraphs (0 = off)")
  ```
- **D-04:** Regexp heuristic in `internal/summarizer/tokenizer.go` shared by both algorithms:
  ```go
  var sentenceEnd = regexp.MustCompile(`[.!?]["''"]?\s+[A-Z"'"]`)
  ```
- **D-05:** `--paragraphs N` distributes output sentences into N paragraphs evenly (blank-line separated).
- **D-06:** If N > sentences, cap paragraphs to sentence count silently (one sentence per paragraph). No error.
- **D-07:** Default `--paragraphs 0` means no grouping — flat output.
- **D-08:** Default plain-text output: one sentence per line (`\n` separator). Breaking change from Phase 1's space-join.
- **D-09:** Token compression stats go to stderr, always in Phase 2.
- **D-10:** Token estimate uses `len(text) / 4` heuristic, labeled as estimated (TOK-02).

### Claude's Discretion
- LexRank: TF-IDF weighting, cosine similarity matrix construction, power iteration convergence tolerance, stable sort for determinism.
- TextRank: word co-occurrence window size, PageRank damping factor, convergence tolerance.
- Test structure: how to unit-test TF-IDF vectors, cosine similarity, and power iteration convergence.
- Determinism strategy: what makes power iteration non-deterministic and how to fix it in Go.

### Deferred Ideas (OUT OF SCOPE)
- Hybrid/ensemble algorithm mode (combine LexRank + TextRank scores).
- SymSpell integration for spell correction pre-processing.
- TTY detection for stats suppression (Phase 3 scope: CLI-05, CLI-06).
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SUM-01 | `--sentences N` flag (default: 5) | D-03 locked; `flag.Int("sentences", 5, ...)` wired into `Summarize(text, n)` |
| SUM-02 | `--paragraphs N` grouping | Post-processing after `Summarize()` returns; divide sentences into N groups |
| SUM-03 | `--algorithm` flag selects lexrank/textrank/graph | `New(algo)` registry in D-02 |
| SUM-04 | N > available sentences → return all without error | Both algorithms: `if n > len(sentences) { n = len(sentences) }` |
| SUM-05 | Output sentences in original document order | Collect top-N indices, `sort.Ints(indices)`, return `sentences[i]` in order |
| SUM-06 | LexRank with IDF-modified cosine similarity | TF-IDF vectors + cosine similarity matrix + power iteration — full algorithm in lexrank.go |
| SUM-07 | TextRank with word-overlap + PageRank | Sentence similarity by word overlap count, normalized; power iteration in textrank.go |
| TOK-01 | Display `~N → ~M tokens (P% reduction)` | Compute after Summarize(); emit to stderr |
| TOK-02 | Token estimate = chars/4, labeled estimated | `fmt.Fprintf(os.Stderr, "~%s → ~%s tokens (%d%% reduction)\n", ...)` |
| TOK-03 | Token stats to stderr only | All stats via `fmt.Fprintf(os.Stderr, ...)` |
| TEST-01 | Unit tests for TF-IDF computation | Known input → expected IDF-modified vectors; test in lexrank_test.go |
| TEST-02 | Unit tests for cosine similarity | Orthogonal vectors → 0.0; identical vectors → 1.0; test in lexrank_test.go |
| TEST-03 | Unit tests for power iteration convergence | Toy 3×3 stochastic matrix → known stationary distribution; test in lexrank_test.go |
| TEST-04 | Integration tests using test-data/ files | Extend integration_test.go to call LexRank and TextRank via new `New(algo)` registry |
| TEST-05 | Edge case tests: empty, single sentence, N > count, unicode | Tokenizer edge cases + algorithm edge cases |
| TEST-06 | Deterministic output: same input → same output | Fixed sorted traversal of data structures; no map iteration dependency in hot path |
</phase_requirements>

---

## Summary

Phase 2 implements two extractive summarization algorithms — LexRank and TextRank — natively in Go with zero external NLP dependencies. The core challenge is not algorithmic complexity (both are well-understood) but getting four properties right simultaneously: **mathematical correctness**, **output determinism**, **test verifiability**, and **interface conformance** with the `Summarizer` interface established in CONTEXT.md.

LexRank builds a sentence similarity graph using IDF-modified cosine similarity, then applies power iteration (the Markov chain / eigenvector centrality method from the original Erkan & Dragomir 2004 paper) to rank sentences. The algorithm has four well-characterized parameters: threshold for graph construction (0.1 is standard from the original paper), convergence epsilon for power iteration (0.0001 is standard, matching `didasy/tldr`'s default tolerance), damping factor (0.85, matching PageRank convention), and IDF formula (log(N/df) where N is sentence count for single-document use).

TextRank uses a simpler sentence similarity metric (normalized word overlap count, as defined in Mihalcea & Tarau 2004), the same PageRank-inspired power iteration, and a damping factor of 0.85. The canonical convergence epsilon is 0.0001.

Determinism in Go requires disciplined avoidance of map iteration in ranking-sensitive code paths. Go maps deliberately randomize iteration order. The fix is straightforward: extract and sort keys before any iteration that affects scores or rankings.

**Primary recommendation:** Implement both algorithms as self-contained structs in `lexrank.go` and `textrank.go` using only `math`, `sort`, `strings`, and `regexp` from stdlib. All data structures that feed into ranking must be traversed via sorted indices or slices, never raw map iteration.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Sentence tokenization | `internal/summarizer/tokenizer.go` | Consumed by lexrank.go and textrank.go | Single tokenizer shared by both algorithms per D-04 |
| TF-IDF computation (LexRank) | `internal/summarizer/lexrank.go` | — | Entirely internal to LexRank struct |
| Cosine similarity matrix | `internal/summarizer/lexrank.go` | — | O(n²) pairwise, internal to LexRank |
| Power iteration (LexRank) | `internal/summarizer/lexrank.go` | — | Markov chain convergence |
| Word overlap similarity (TextRank) | `internal/summarizer/textrank.go` | — | Normalized shared-word count |
| Power iteration (TextRank) | `internal/summarizer/textrank.go` | — | Same convergence pattern as LexRank |
| Algorithm dispatch / registry | `internal/summarizer/summarizer.go` | — | `New(algo)` → Summarizer interface |
| CLI flag wiring | `cmd/tldt/main.go` | — | Extend existing main.go with 3 new flags |
| Token stats output | `cmd/tldt/main.go` | — | Post-Summarize() computation, emitted to stderr |
| Paragraph grouping | `cmd/tldt/main.go` | — | Post-processing on `[]string` returned by Summarize() |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `math` | Go 1.26.2 | `math.Log`, `math.Sqrt`, `math.Abs` for TF-IDF and cosine | Zero-dependency; all needed math is here [VERIFIED: pkg.go.dev/math] |
| stdlib `sort` | Go 1.26.2 | `sort.Ints`, `sort.Strings`, `sort.SliceStable` for determinism | Required for deterministic sort of ranked indices [VERIFIED: pkg.go.dev/sort] |
| stdlib `strings` | Go 1.26.2 | `strings.Fields`, `strings.ToLower`, `strings.ContainsRune` | Word tokenization and normalization [VERIFIED: pkg.go.dev/strings] |
| stdlib `regexp` | Go 1.26.2 | Sentence boundary detection (D-04 decision) | `regexp.MustCompile` for precompiled sentence splitter [VERIFIED: pkg.go.dev/regexp] |
| stdlib `unicode` | Go 1.26.2 | `unicode.IsLetter`, `unicode.IsDigit` for word cleaning | Correct handling of unicode punctuation [VERIFIED: pkg.go.dev/unicode] |

### No New External Dependencies Required
Both algorithms are implementable using only stdlib. No `go get` needed for Phase 2. [VERIFIED: algorithm descriptions in Erkan 2004 and Mihalcea 2004 — both reducible to linear algebra over bags-of-words]

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/didasy/tldr` | v0.7.0 | Retained as `graph` algorithm baseline | Already in go.mod; `Graph` struct continues to work |

---

## Architecture Patterns

### System Architecture Diagram

```
Input text (string)
    |
    v
[ tokenizer.go: TokenizeSentences(text) ]
    |
    v
[]string sentences (original order preserved)
    |
    +--[lexrank.go]-------------------------------+
    |  1. buildIDFVector(sentences) → map[word]float64
    |  2. buildTFIDFMatrix(sentences, idf) → [][]float64
    |  3. buildCosineMatrix(tfidf) → [][]float64 (n×n)
    |  4. applyThreshold(cosine, 0.1) → [][]float64 (binary/weighted)
    |  5. powerIteration(matrix, epsilon=0.0001) → []float64 scores
    |  6. selectTopN(scores, n) → []int (sorted indices, original order)
    |
    +--[textrank.go]------------------------------+
    |  1. buildWordOverlapMatrix(sentences) → [][]float64 (n×n)
    |  2. powerIteration(matrix, damping=0.85, epsilon=0.0001) → []float64
    |  3. selectTopN(scores, n) → []int (sorted indices, original order)
    |
    +--[graph.go]---------------------------------+
       delegates to didasy/tldr (unchanged)

Each path returns []string (selected sentences in document order)
    |
    v
[ cmd/tldt/main.go ]
    |
    +-- token stats → stderr
    +-- paragraph grouping → post-process []string
    +-- output: one sentence per line → stdout
```

### Recommended Project Structure (Phase 2 additions)

```
internal/summarizer/
├── tokenizer.go         # NEW: TokenizeSentences(), shared sentence splitter
├── tokenizer_test.go    # NEW: edge cases for sentence tokenizer
├── lexrank.go           # NEW: LexRank struct implementing Summarizer
├── lexrank_test.go      # NEW: TF-IDF unit tests, cosine unit tests, power iteration test
├── textrank.go          # NEW: TextRank struct implementing Summarizer
├── textrank_test.go     # NEW: word overlap unit tests, power iteration test
├── summarizer.go        # EXTEND: add New(algo) registry + Summarizer interface
├── graph.go             # UNCHANGED
├── graph_test.go        # UNCHANGED
└── integration_test.go  # EXTEND: add test cases calling New("lexrank") and New("textrank")
```

---

## LexRank Algorithm: Precise Implementation Specification

### Step 1: Sentence Tokenization

Use `tokenizer.go` (D-04). The regexp `[.!?]["''"]?\s+[A-Z"'"]` splits on sentence-ending punctuation followed by whitespace and a capital letter. This is a heuristic — it handles 95%+ of English prose but will miss some edge cases (abbreviations like "Dr. Smith"). [ASSUMED: heuristic is sufficient for Phase 2 test fixtures; locked by D-04]

Trim whitespace from each sentence after splitting.

### Step 2: IDF Computation (for IDF-modified cosine similarity)

For single-document summarization, "documents" are sentences. IDF is computed over the sentence corpus:

```
IDF(word) = log( N / df(word) )
```

Where:
- N = total number of sentences
- df(word) = number of sentences containing the word (document frequency)

This matches the formula in Erkan & Dragomir 2004. [CITED: https://www.cs.cmu.edu/afs/cs/project/jair/pub/volume22/erkan04a-html/erkan04a.html]

Use `math.Log` (natural log). Store IDF values in a `map[string]float64`.

**Word normalization:** lowercase + strip non-alphanumeric (same approach as `didasy/tldr`'s `createDictionary`). [VERIFIED: tldr.go createDictionary source in module cache]

### Step 3: TF-IDF Vector for Each Sentence

For each sentence, build a TF-IDF vector:

```
TF(word, sentence) = (count of word in sentence) / (total words in sentence)
TFIDF(word, sentence) = TF(word, sentence) * IDF(word)
```

The vector has one dimension per unique word in the vocabulary. Store as `[]float64` indexed by sorted vocabulary position (not map — sorted slice for determinism).

**Implementation note:** Build vocabulary as sorted `[]string` of all unique words. Map each word to its index in this sorted slice. This ensures vectors are comparable across sentences without map-order dependency. [VERIFIED: sort.Strings behavior confirmed — pkg.go.dev/sort]

### Step 4: IDF-Modified Cosine Similarity Between Two Sentences

```
         Σ(idf(w)² * tf(w,s1) * tf(w,s2))
sim(s1, s2) = ─────────────────────────────────────────────────────────
              √(Σ idf(w)² * tf(w,s1)²) × √(Σ idf(w)² * tf(w,s2)²)
```

This is the IDF-modified cosine similarity from the original LexRank paper. [CITED: https://www.cs.cmu.edu/afs/cs/project/jair/pub/volume22/erkan04a-html/erkan04a.html]

The denominator is the L2 norm of each IDF-weighted TF vector. If either norm is zero (all-zero vector, which occurs for very short sentences with no vocabulary overlap), return 0.0.

**Go implementation:**
```go
// Source: LexRank paper (Erkan & Dragomir 2004) [CITED]
func idfCosine(v1, v2, idf []float64) float64 {
    dot, n1, n2 := 0.0, 0.0, 0.0
    for i := range v1 {
        w := idf[i] * idf[i]
        dot += w * v1[i] * v2[i]
        n1  += w * v1[i] * v1[i]
        n2  += w * v2[i] * v2[i]
    }
    if n1 == 0 || n2 == 0 {
        return 0
    }
    return dot / (math.Sqrt(n1) * math.Sqrt(n2))
}
```

### Step 5: Similarity Matrix Construction with Threshold

Build an n×n matrix where entry [i][j] = `idfCosine(sentences[i], sentences[j])`.

For the thresholded (standard LexRank) variant:
- If `sim(i,j) >= threshold` → matrix[i][j] = 1.0 (or the raw similarity for continuous LexRank)
- If `sim(i,j) < threshold` → matrix[i][j] = 0.0

**Standard threshold: 0.1** (from Erkan & Dragomir 2004 and confirmed across multiple implementations including crabcamp/lexrank). [CITED: https://www.cs.cmu.edu/afs/cs/project/jair/pub/volume22/erkan04a-html/erkan04a.html]

Use continuous (unthresholded) as default — it produces better results on short single documents. Expose as a constant `lexrankThreshold = 0.0` (disabled) or use the raw similarity values directly in the matrix.

**Recommendation for Phase 2 (Claude's Discretion):** Use continuous LexRank (no threshold) for simplicity and better behavior on the test fixture sizes (50–500 sentences). This matches SUM-06 requirement "IDF-modified cosine similarity" without requiring a tuned threshold. [ASSUMED: continuous variant is appropriate for Phase 2 test corpus sizes]

### Step 6: Row-Normalize the Matrix

For power iteration to converge, the matrix must be row-stochastic (each row sums to 1.0):

```go
for i := range matrix {
    sum := 0.0
    for _, v := range matrix[i] {
        sum += v
    }
    if sum > 0 {
        for j := range matrix[i] {
            matrix[i][j] /= sum
        }
    } else {
        // dangling row: assign uniform probability
        for j := range matrix[i] {
            matrix[i][j] = 1.0 / float64(n)
        }
    }
}
```

[ASSUMED: dangling row handling via uniform distribution is standard for PageRank-family algorithms]

### Step 7: Power Iteration

Power iteration computes the stationary distribution (dominant eigenvector) of the row-stochastic matrix:

```go
// Source: power iteration algorithm (standard) [ASSUMED: matches didasy/tldr tolerance=0.0001]
func powerIterate(matrix [][]float64, epsilon float64) []float64 {
    n := len(matrix)
    scores := make([]float64, n)
    for i := range scores {
        scores[i] = 1.0 / float64(n)  // uniform initialization
    }
    for {
        next := make([]float64, n)
        for i := range next {
            for j := range scores {
                next[i] += matrix[j][i] * scores[j]
            }
        }
        // check convergence: L1 norm of difference
        diff := 0.0
        for i := range scores {
            diff += math.Abs(next[i] - scores[i])
        }
        scores = next
        if diff < epsilon {
            break
        }
    }
    return scores
}
```

**Convergence epsilon: 0.0001** — matches `didasy/tldr`'s `DEFAULT_TOLERANCE` constant. [VERIFIED: tldr.go source in module cache, line: `DEFAULT_TOLERANCE = 0.0001`]

**Convergence guarantee:** A row-stochastic matrix with all positive entries converges under power iteration by the Perron-Frobenius theorem. For sparse matrices (many zeros), add a damping factor to ensure aperiodicity. Standard damping: 0.85 (used by LexRank when applied with the PageRank formulation). [CITED: https://nlp.stanford.edu/IR-book/html/htmledition/the-pagerank-computation-1.html]

**Practical iteration cap:** Add a max-iterations guard (e.g., 1000) to prevent infinite loops on pathological inputs. [ASSUMED: 1000 iterations is sufficient for any realistic single-document corpus]

### Step 8: Select Top-N and Restore Original Order

```go
// Sort sentence indices by score (descending), take top n
type scored struct{ idx int; score float64 }
ranked := make([]scored, len(scores))
for i, s := range scores {
    ranked[i] = scored{i, s}
}
// STABLE sort to ensure determinism when scores are equal
sort.SliceStable(ranked, func(a, b int) bool {
    return ranked[a].score > ranked[b].score
})
if n > len(ranked) {
    n = len(ranked)
}
top := make([]int, n)
for i := 0; i < n; i++ {
    top[i] = ranked[i].idx
}
// Restore document order
sort.Ints(top)
result := make([]string, n)
for i, idx := range top {
    result[i] = sentences[idx]
}
return result, nil
```

[VERIFIED: `sort.SliceStable` exists in stdlib sort package — pkg.go.dev/sort]

---

## TextRank Algorithm: Precise Implementation Specification

### Step 1: Sentence Tokenization

Same `TokenizeSentences()` from `tokenizer.go` as LexRank.

### Step 2: Word Overlap Similarity

From the original Mihalcea & Tarau 2004 paper, sentence similarity is:

```
             |{w | w ∈ Si AND w ∈ Sj}|
sim(Si, Sj) = ─────────────────────────────────────────
              log(|Si|) + log(|Sj|)
```

Where `|Si|` is the word count of sentence i. The logarithmic normalization penalizes long sentences less aggressively than linear division. [CITED: https://web.eecs.umich.edu/~mihalcea/papers/mihalcea.emnlp04.pdf]

**Word normalization:** lowercase + strip non-alphanumeric punctuation. No stop-word removal required — the original paper does not apply stop-word filtering for sentence extraction (only for keyword extraction).

**Edge case:** If either sentence has 0 or 1 word after normalization, return 0.0 (log(0) undefined; log(1) = 0 causing division by zero).

```go
// Source: Mihalcea & Tarau 2004 [CITED]
func wordOverlapSim(s1, s2 []string) float64 {
    if len(s1) <= 1 || len(s2) <= 1 {
        return 0
    }
    set := make(map[string]struct{}, len(s1))
    for _, w := range s1 {
        set[w] = struct{}{}
    }
    common := 0
    for _, w := range s2 {
        if _, ok := set[w]; ok {
            common++
        }
    }
    if common == 0 {
        return 0
    }
    return float64(common) / (math.Log(float64(len(s1))) + math.Log(float64(len(s2))))
}
```

### Step 3: Similarity Matrix, Row-Normalization, Power Iteration

Same as LexRank steps 5–7:
- Build n×n matrix with `wordOverlapSim(i, j)`
- Row-normalize to stochastic matrix
- Apply `powerIterate(matrix, damping=0.85, epsilon=0.0001)`

**Damping factor in TextRank:** Apply the standard PageRank teleportation formula:

```
score(i) = (1 - d) / n  +  d × Σ_j (matrix[j][i] × score[j])
```

Where `d = 0.85`. [CITED: TextRank paper, damping factor; confirmed by crabcamp/lexrank and DavidBelicza/TextRank implementations]

### Step 4: Select Top-N and Restore Original Order

Same as LexRank Step 8.

**Window size (Claude's Discretion):** TextRank for keyword extraction uses a co-occurrence window, but for sentence extraction the similarity is full sentence-to-sentence overlap — no window parameter is needed. Window size is a keyword extraction concept only. [CITED: Mihalcea & Tarau 2004 — sentence extraction uses all-pairs overlap, not windowed co-occurrence]

---

## Determinism Strategy

### Root Cause of Non-Determinism in Go

Three sources of non-determinism exist in a naive implementation:

1. **Map iteration order** — Go deliberately randomizes map key iteration since Go 1.12. Any loop over a `map[string]float64` that feeds into score computation will produce different orderings across runs.
2. **`sort.Slice` instability** — `sort.Slice` is not guaranteed stable; equal elements can be permuted. When two sentences have identical scores, their relative ranking is undefined.
3. **Float accumulation order** — Summing floats in different orders yields slightly different results. If iteration order varies (due to map), accumulated sums vary, causing microscopically different scores.

[VERIFIED: Go sort package docs — "sort.Slice is not guaranteed to be stable" — pkg.go.dev/sort]
[VERIFIED: Go spec on map iteration randomization — go.dev/blog/maps]

### Fix

| Source | Fix |
|--------|-----|
| Map iteration in vocabulary | Build vocabulary as `[]string`, sort once with `sort.Strings`, use slice index — never iterate over `map[string]float64` in scoring loops |
| Map iteration for IDF values | Store IDF as `[]float64` indexed by vocabulary position (parallel array to sorted vocab slice) |
| Tie-breaking in ranked sort | Use `sort.SliceStable` instead of `sort.Slice` for final score ranking |
| Float sum consistency | Iterate over slices (ordered, deterministic) not maps |

**Pattern:**

```go
// DETERMINISTIC: vocabulary as sorted slice
vocab := make([]string, 0, len(wordSet))
for w := range wordSet {
    vocab = append(vocab, w)
}
sort.Strings(vocab) // sort ONCE, use index thereafter

wordIdx := make(map[string]int, len(vocab))
for i, w := range vocab {
    wordIdx[w] = i
}
idf := make([]float64, len(vocab)) // indexed by vocab position
```

This pattern means all vector operations iterate over `[]float64` slices, never over maps. [VERIFIED: sort.Strings behavior — pkg.go.dev/sort]

---

## Tokenizer Implementation

D-04 specifies a regexp heuristic. The provided pattern needs augmentation to produce a usable `[]string` of sentences rather than match positions:

```go
// Source: D-04 decision [CITED: 02-CONTEXT.md]
var sentenceEnd = regexp.MustCompile(`[.!?]["'"]?\s+[A-Z]`)

func TokenizeSentences(text string) []string {
    text = strings.TrimSpace(text)
    if text == "" {
        return nil
    }
    var sentences []string
    for {
        loc := sentenceEnd.FindStringIndex(text)
        if loc == nil {
            break
        }
        // boundary is at loc[0]+1 (after the punctuation, before the space)
        boundary := loc[0] + 1
        sentence := strings.TrimSpace(text[:boundary])
        if sentence != "" {
            sentences = append(sentences, sentence)
        }
        text = strings.TrimSpace(text[boundary:])
    }
    // last remaining text is the final sentence
    if text != "" {
        sentences = append(sentences, text)
    }
    return sentences
}
```

**Known limitation:** The regexp will fail to split at abbreviations ("Dr. Smith", "U.S. Senate"), hyphenated compounds used mid-sentence, and sentences ending without terminal punctuation (headlines, bullet points). This is acceptable for Phase 2 and documented in CONTEXT.md as a known constraint. [ASSUMED: heuristic is adequate for the four test-data fixtures; SymSpell/NLP tokenizer is deferred]

**Unicode handling:** `regexp` operates on UTF-8 natively. The pattern `[A-Z]` only matches ASCII capitals — non-English sentence starts (e.g., accented capitals) will not be detected. This is acceptable since the test corpus is English. [VERIFIED: Go regexp/syntax — pkg.go.dev/regexp/syntax]

---

## Summarizer Interface and Registry

`summarizer.go` currently contains only `package summarizer` (19 bytes). Phase 2 extends it:

```go
// Source: D-02 decision [CITED: 02-CONTEXT.md]
package summarizer

import "fmt"

// Summarizer is the common interface for all extractive summarization algorithms.
// Summarize returns up to n sentences from text in original document order.
// If n > sentence count, all sentences are returned (SUM-04).
type Summarizer interface {
    Summarize(text string, n int) ([]string, error)
}

// New returns a Summarizer for the named algorithm.
// Valid names: "lexrank", "textrank", "graph".
func New(algo string) (Summarizer, error) {
    switch algo {
    case "lexrank":
        return &LexRank{}, nil
    case "textrank":
        return &TextRank{}, nil
    case "graph":
        return &Graph{}, nil
    default:
        return nil, fmt.Errorf("unknown algorithm: %s", algo)
    }
}
```

The existing `graph.go` exports a package-level `Summarize()` function, not a struct. Phase 2 wraps it in a `Graph` struct:

```go
// graph.go addition — wrap existing Summarize() in a struct to satisfy interface
type Graph struct{}

func (g *Graph) Summarize(text string, n int) ([]string, error) {
    return Summarize(text, n) // calls existing package-level function
}
```

[VERIFIED: current `graph.go` source at /Users/gleicon/code/go/src/github.com/gleicon/tldt/internal/summarizer/graph.go — only package-level Summarize(), no struct]

---

## CLI Changes (cmd/tldt/main.go)

The existing `main.go` calls `summarizer.Summarize(text, defaultSentences)` directly. Phase 2 changes this to use the registry:

```go
// Phase 2 main.go additions (extending existing file)
algorithm := flag.String("algorithm", "lexrank", "algorithm: lexrank|textrank|graph")
sentences  := flag.Int("sentences", 5, "number of output sentences")
paragraphs := flag.Int("paragraphs", 0, "group sentences into N paragraphs (0 = off)")

// after flag.Parse() and resolveInput():
s, err := summarizer.New(*algorithm)
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}

charsIn := len(text)
result, err := s.Summarize(text, *sentences)
if err != nil {
    fmt.Fprintln(os.Stderr, "summarization failed:", err)
    os.Exit(1)
}

// Token stats to stderr (TOK-01, TOK-02, TOK-03)
charsOut := len(strings.Join(result, " "))
tokIn  := charsIn  / 4
tokOut := charsOut / 4
reduction := 0
if tokIn > 0 {
    reduction = int(float64(tokIn-tokOut) / float64(tokIn) * 100)
}
fmt.Fprintf(os.Stderr, "~%s → ~%s tokens (%d%% reduction)\n",
    formatTokens(tokIn), formatTokens(tokOut), reduction)

// Output: one sentence per line (D-08, breaking change from Phase 1)
if *paragraphs > 0 {
    output = groupIntoParagraphs(result, *paragraphs)
} else {
    fmt.Println(strings.Join(result, "\n"))
}
```

**formatTokens helper:** Format with commas for readability (`~12,400`). Use `strconv.AppendInt` or `fmt.Sprintf` with manual comma insertion. [ASSUMED: simple comma-format helper, ~10 lines]

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TF-IDF library | Custom NLP framework | stdlib `math` + sorted slices | Full TF-IDF for extractive summarization is ~50 lines; no library needed |
| PageRank solver | Graph library | Power iteration loop (~20 lines) | Power method is trivial for dense n×n where n < 5000 |
| Sentence tokenizer | NLP library (NLTK, spaCy) | Regexp heuristic (D-04) | External NLP libraries are explicitly excluded; heuristic sufficient for English prose |
| Cosine similarity | Vector library | Inline dot product (~10 lines) | Simple enough to inline; libraries would add dependencies |
| Sorted iteration | `golang.org/x/exp/maps` | `sort.Strings` on extracted keys | stdlib is sufficient; no external package needed for key sorting |
| Stop-word removal | NLP toolkit | None (skip for Phase 2) | Neither algorithm requires stop-word removal for sentence extraction; deferred with SymSpell |

**Key insight:** Both LexRank and TextRank for extractive summarization reduce to three operations — similarity matrix construction, matrix normalization, and power iteration. Each is 10–30 lines of Go. The total implementation is ~300 lines across two files, all stdlib.

---

## Common Pitfalls

### Pitfall 1: Map Iteration in Scoring Path

**What goes wrong:** Building TF-IDF vectors by iterating `for word, idf := range idfMap` produces different vector element orderings across runs. Two sentences with identical term distributions produce vectors with shuffled dimension alignment — dot products are computed incorrectly and scores vary run-to-run.
**Why it happens:** Go map iteration is randomized since Go 1.12 as a security measure.
**How to avoid:** Build a sorted vocabulary slice once (`sort.Strings(vocab)`), assign each word a fixed index, then build `[]float64` vectors using these indices. Never use map range in the vector construction or scoring loop.
**Warning signs:** `TEST-06` fails — same input produces different output on successive runs.

### Pitfall 2: sort.Slice vs sort.SliceStable for Final Ranking

**What goes wrong:** Using `sort.Slice` to sort sentences by score when two sentences have exactly equal scores: their relative order is undefined across Go versions and runs.
**Why it happens:** `sort.Slice` uses an unstable sort algorithm (quicksort variant).
**How to avoid:** Use `sort.SliceStable` for the final sentence ranking step. [VERIFIED: pkg.go.dev/sort]
**Warning signs:** Test assertions on tie-broken rankings fail intermittently.

### Pitfall 3: Division by Zero in Cosine Similarity

**What goes wrong:** A sentence with all stopwords or a single unknown word produces an all-zero TF-IDF vector. Dividing by the L2 norm of a zero vector causes NaN or panic.
**Why it happens:** Short sentences or sentences with words not in the vocabulary produce zero vectors.
**How to avoid:** Check `norm > 0` before dividing. Return `sim = 0.0` if either vector has zero norm. Explicitly test with `edge_short.txt` (3-sentence test fixture).
**Warning signs:** `NaN` in the similarity matrix; power iteration diverges or returns all-NaN scores.

### Pitfall 4: Dangling Nodes in the Markov Matrix

**What goes wrong:** A sentence that has zero similarity to all other sentences (its row sums to 0 after thresholding) produces a non-stochastic row, which breaks power iteration convergence.
**Why it happens:** Threshold of 0.1 may exclude all edges for very short or very unusual sentences.
**How to avoid:** After computing the matrix, check for zero rows and replace them with uniform probability (`1/n` for all entries). Using continuous LexRank (no threshold) in Phase 2 avoids this entirely for most inputs.
**Warning signs:** Power iteration fails to converge or returns uniform scores for all sentences.

### Pitfall 5: Sentence Tokenizer Splitting on Abbreviations

**What goes wrong:** The D-04 regexp splits on `"Dr. Smith"` into `["Dr.", "Smith said..."]` producing a mangled sentence. This corrupts the TF-IDF vectors and produces incorrect summaries on the longform test fixture.
**Why it happens:** The pattern `[.!?]\s+[A-Z]` cannot distinguish abbreviations from sentence ends.
**How to avoid:** Accept the known limitation — document it as a heuristic. Do not attempt to fix abbreviations in Phase 2 (SymSpell/NLP tokenizer is deferred). The test fixtures should not be written to rely on abbreviation handling.
**Warning signs:** `TEST-04` integration tests produce unexpected sentence splits on `longform_3000.txt`.

### Pitfall 6: Graph Struct Breaking Existing graph.go

**What goes wrong:** Adding `type Graph struct{}` with a `Summarize()` method to `graph.go` conflicts with the existing package-level `Summarize()` function — both are in `package summarizer`.
**Why it happens:** The method `(g *Graph) Summarize(...)` and the function `Summarize(...)` coexist fine, but callers in existing tests that call `Summarize(text, n)` directly still work. The conflict appears if integration tests are updated to call `New("graph").Summarize(...)` before the struct wrapper is added.
**How to avoid:** Add the `Graph` struct wrapper in the same commit as `summarizer.go`'s `New()` function. Update `main.go` in the same wave. Existing `graph_test.go` calls the package-level `Summarize()` — leave those tests unchanged.
**Warning signs:** `go build ./...` fails with "method and function with same name" (this is NOT actually an error in Go — method receiver makes them different symbols; document this to avoid confusion).

---

## Code Examples

### TF-IDF Vector Construction (Deterministic)

```go
// Source: LexRank paper (Erkan & Dragomir 2004) [CITED] + Go sort stdlib [VERIFIED]
func buildVocabAndIDF(sentences [][]string) ([]string, []float64) {
    N := len(sentences)
    // count document frequency for each word
    df := make(map[string]int)
    for _, words := range sentences {
        seen := make(map[string]bool)
        for _, w := range words {
            if !seen[w] {
                df[w]++
                seen[w] = true
            }
        }
    }
    // build sorted vocabulary for deterministic indexing
    vocab := make([]string, 0, len(df))
    for w := range df {
        vocab = append(vocab, w)
    }
    sort.Strings(vocab) // determinism: fixed word→index mapping

    idf := make([]float64, len(vocab))
    for i, w := range vocab {
        idf[i] = math.Log(float64(N) / float64(df[w]))
    }
    return vocab, idf
}
```

### Cosine Similarity (Unit-Testable)

```go
// Source: LexRank paper (Erkan & Dragomir 2004) [CITED]
// idfCosine computes IDF-modified cosine similarity between two TF vectors.
// v1, v2 are TF vectors (indexed by vocab position).
// idf is the IDF weight for each vocab position.
func idfCosine(v1, v2, idf []float64) float64 {
    dot, n1, n2 := 0.0, 0.0, 0.0
    for i := range v1 {
        w := idf[i] * idf[i]
        dot += w * v1[i] * v2[i]
        n1  += w * v1[i] * v1[i]
        n2  += w * v2[i] * v2[i]
    }
    if n1 == 0 || n2 == 0 {
        return 0
    }
    return dot / (math.Sqrt(n1) * math.Sqrt(n2))
}

// TestCosine_Identical: idfCosine(v, v, idf) == 1.0
// TestCosine_Orthogonal: idfCosine([1,0,0], [0,1,0], [1,1,1]) == 0.0
```

### Power Iteration (Unit-Testable with Known Stationary Distribution)

```go
// Source: standard power method [ASSUMED matches didasy/tldr DEFAULT_TOLERANCE=0.0001]
// powerIterate returns the stationary distribution of a row-stochastic matrix.
// epsilon: convergence threshold (use 0.0001).
// maxIter: safety cap (use 1000).
func powerIterate(matrix [][]float64, epsilon float64, maxIter int) []float64 {
    n := len(matrix)
    p := make([]float64, n)
    for i := range p {
        p[i] = 1.0 / float64(n)
    }
    for iter := 0; iter < maxIter; iter++ {
        next := make([]float64, n)
        for i := range next {
            for j := range p {
                next[i] += matrix[j][i] * p[j]
            }
        }
        diff := 0.0
        for i := range p {
            diff += math.Abs(next[i] - p[i])
        }
        p = next
        if diff < epsilon {
            break
        }
    }
    return p
}

// Unit test: 2×2 stochastic matrix [[0.5,0.5],[0.5,0.5]] → stationary [0.5, 0.5]
// Unit test: 3×3 identity matrix    → each row stays at initial [1/3, 1/3, 1/3]
```

### Token Stats Output

```go
// Source: D-09, D-10 decisions [CITED: 02-CONTEXT.md]
func formatTokens(n int) string {
    s := strconv.Itoa(n)
    // insert commas every 3 digits from the right
    if len(s) <= 3 {
        return s
    }
    var b strings.Builder
    rem := len(s) % 3
    if rem > 0 {
        b.WriteString(s[:rem])
        if len(s) > rem {
            b.WriteByte(',')
        }
    }
    for i := rem; i < len(s); i += 3 {
        b.WriteString(s[i : i+3])
        if i+3 < len(s) {
            b.WriteByte(',')
        }
    }
    return b.String()
}
// fmt.Fprintf(os.Stderr, "~%s → ~%s tokens (%d%% reduction)\n",
//     formatTokens(charsIn/4), formatTokens(charsOut/4), reduction)
```

### Test Pattern for Power Iteration Convergence (TEST-03)

```go
func TestPowerIterate_UniformMatrix(t *testing.T) {
    // A 3×3 matrix where every row is [1/3, 1/3, 1/3]
    // Stationary distribution is [1/3, 1/3, 1/3]
    n := 3
    m := make([][]float64, n)
    for i := range m {
        m[i] = []float64{1.0 / 3, 1.0 / 3, 1.0 / 3}
    }
    got := powerIterate(m, 0.0001, 1000)
    for i, v := range got {
        if math.Abs(v - 1.0/3) > 0.001 {
            t.Errorf("scores[%d] = %f, want ~0.333", i, v)
        }
    }
}
```

---

## Anti-Patterns to Avoid

- **Ranging over `map[string]float64` in scoring:** Produces non-deterministic output. Use sorted `[]string` vocab + `map[string]int` for index lookup only.
- **`sort.Slice` for final ranking:** Use `sort.SliceStable` to guarantee deterministic tie-breaking.
- **Sharing state between `Summarize()` calls:** Both `LexRank{}` and `TextRank{}` must be stateless; build all intermediate structures (idf, matrix, scores) within each `Summarize()` call and discard them afterward. No instance variables.
- **Calling `didasy/tldr` from lexrank.go/textrank.go:** These are independent implementations. `didasy/tldr` is only used by `graph.go`.
- **Testing with only the short test fixtures:** `edge_short.txt` (3 sentences) will have a 3×3 similarity matrix. Power iteration on a 3×3 matrix always converges in 1–2 iterations regardless of correctness. Use `wikipedia_en.txt` or `longform_3000.txt` for meaningful algorithm correctness testing.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Centrality-only LexRank (degree counting) | Continuous LexRank (eigenvector centrality) | Original 2004 paper | Better sentence ranking for multi-document clusters |
| Word overlap only (TextRank original) | IDF-weighted similarity variants | Post-2004 | Better quality; Phase 2 implements the original paper's formula |
| PageRank with separate damping formula | Incorporate damping into power iteration directly | Standard practice | Simpler implementation, same result |
| `ioutil.ReadAll` | `io.ReadAll` | Go 1.16 | Already using correct form in Phase 1 code |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Continuous LexRank (no threshold) is appropriate for single-document Phase 2 test corpus | LexRank Step 5 | Low — worst case, add threshold constant later; continuous is the more general variant |
| A2 | Power iteration with epsilon=0.0001 and maxIter=1000 is sufficient for all test fixtures | Power Iteration | Low — 1000 iterations is far more than needed for any document under 2000 sentences |
| A3 | Dangling row uniform distribution is the standard fix | Pitfall 4, Normalization | Low — standard PageRank treatment; confirmed in PageRank literature |
| A4 | The D-04 regexp tokenizer is adequate for the 4 English test fixtures | Tokenizer section | Medium — longform_3000.txt may contain abbreviations; if tests fail due to tokenization, the sentence splitter needs a simple lookahead rule for common abbreviations |
| A5 | `formatTokens` comma-formatting implementation (10 lines, stdlib-only) | CLI Changes | Low — alternative: use `golang.org/x/text/message` for locale-aware formatting, but stdlib is sufficient |
| A6 | Adding `type Graph struct{}` to `graph.go` does not break existing `graph_test.go` | Graph Struct section | Low — Go allows both method and package-level function with same name on different receivers |

---

## Open Questions

1. **Should LexRank use a damping factor or rely on the row-normalization alone?**
   - What we know: The original LexRank paper uses power iteration on a row-stochastic matrix without an explicit damping factor (unlike PageRank on the web). Damping is an option but not required.
   - What's unclear: Whether test fixtures have isolated sentences (zero similarity to all others) that would cause non-convergence without damping.
   - Recommendation: Start without explicit damping (pure power iteration on normalized matrix); add uniform damping `d=0.85` if convergence tests show issues. The dangling-row fix handles most edge cases.

2. **Should TextRank stop-word removal be included?**
   - What we know: The original Mihalcea & Tarau 2004 paper does not apply stop-word removal for sentence extraction (only for keyword extraction).
   - What's unclear: Whether common English stop words ("the", "a", "is") inflate similarity scores between unrelated sentences sharing only function words.
   - Recommendation: Skip stop-word removal in Phase 2. If integration test results show poor quality, add a simple hard-coded stop-word set as a constant in `textrank.go`.

---

## Environment Availability

Step 2.6: All Phase 2 work is code-only. No new external tools, services, or CLIs are required. Go 1.26.2 is already confirmed available.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All build/test | Yes | go1.26.2 darwin/arm64 | — |
| stdlib `math`, `sort`, `strings`, `regexp` | LexRank, TextRank | Yes (stdlib) | go1.26.2 | — |
| `github.com/didasy/tldr` | graph.go (unchanged) | Yes (go.mod) | v0.7.0 | — |

**Missing dependencies with no fallback:** None.

---

## Security Domain

Phase 2 adds no network access, persistence, authentication, or user data handling. The binary remains a local offline CLI tool. ASVS controls continue to not apply. No new attack surface introduced by adding two in-memory summarization algorithms.

---

## Sources

### Primary (HIGH confidence)
- `tldr.go` source at `/Users/gleicon/code/go/pkg/mod/github.com/didasy/tldr@v0.7.0/tldr.go` — `DEFAULT_TOLERANCE=0.0001`, `DEFAULT_DAMPING=0.85`, power iteration pattern, vector construction approach [VERIFIED: directly read]
- `pkg.go.dev/sort` — `sort.Slice` is not stable; `sort.SliceStable` guarantees stability [VERIFIED: official Go docs]
- `pkg.go.dev/math` — `math.Log`, `math.Sqrt`, `math.Abs` confirmed available [VERIFIED: official Go docs]

### Secondary (MEDIUM confidence)
- Erkan & Dragomir 2004, "LexRank: Graph-based Lexical Centrality as Salience in Text Summarization" — IDF-modified cosine similarity formula, threshold 0.1, power iteration approach [CITED: https://www.cs.cmu.edu/afs/cs/project/jair/pub/volume22/erkan04a-html/erkan04a.html]
- Mihalcea & Tarau 2004, "TextRank: Bringing Order into Texts" — word overlap similarity formula, damping factor 0.85, convergence approach [CITED: https://web.eecs.umich.edu/~mihalcea/papers/mihalcea.emnlp04.pdf]
- crabcamp/lexrank Python implementation (234 stars, MIT) — confirmed threshold=0.1, continuous variant, power method structure [WebSearch verified, architecture confirmed via WebFetch]
- DavidBelicza/TextRank Go implementation — existence confirmed (223 stars, MIT, Go 1.8+, supports goroutines) but NOT used as dependency [WebSearch verified]

### Tertiary (LOW confidence)
- oneuptime.com blog post on Go map non-determinism — sorted keys solution confirmed via multiple WebSearch results [WebSearch — not official docs but consistent with Go spec]

---

## Metadata

**Confidence breakdown:**
- LexRank algorithm specifics: HIGH — core formulas from original 2004 paper (JAIR publication, stable source); convergence parameters verified from didasy/tldr source
- TextRank algorithm specifics: HIGH — core formula from original 2004 paper; damping factor 0.85 confirmed universally
- Go determinism strategy: HIGH — map non-determinism is documented Go language behavior; sort.Slice instability is documented in stdlib
- Tokenizer implementation: MEDIUM — regexp heuristic is locked by D-04 but its adequacy for all test fixtures is assumed
- Test patterns: HIGH — derived from existing test files in the codebase (`graph_test.go`, `integration_test.go`) which set the conventions to follow

**Research date:** 2026-05-01
**Valid until:** 2026-11-01 (algorithm mathematics are stable; Go stdlib APIs do not change between minor versions)
