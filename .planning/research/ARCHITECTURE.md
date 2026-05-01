# Architecture Research — LexRank vs TextRank

**Project:** tldt — Too Long, Didn't Tokenize
**Researched:** 2026-05-01
**Overall confidence:** HIGH (algorithms from original papers, MEDIUM for Go-specific ecosystem)

---

## LexRank Algorithm

### Conceptual foundation

LexRank (Erkan & Radev, 2004) treats a document as a graph where sentences are nodes and edges are weighted by how similar sentences are to each other. The insight is that a sentence which is semantically similar to many other sentences is likely central to the document's topic — the same logic as PageRank applied to sentences instead of web pages.

The key difference from simple frequency-based methods: a sentence earns high score not just by resembling one other sentence, but by resembling many sentences that themselves resemble many sentences. This eigenvector centrality property prevents clusters of near-duplicate low-quality sentences from inflating each other's scores (the "local trap problem").

### Step-by-step

**Step 1 — Tokenize and preprocess sentences**

Split the input into sentences. For each sentence, tokenize into words, lowercase, remove stopwords. This gives you a token set per sentence.

**Step 2 — Compute IDF across the entire document**

IDF (inverse document frequency) weights words by how discriminating they are. In LexRank, the "documents" are the individual sentences:

```
IDF(w) = log( N / df(w) )
```

Where:
- `N` = total number of sentences
- `df(w)` = number of sentences that contain word `w`

Words appearing in every sentence get IDF = 0 and contribute nothing to similarity. Rare but shared words get high IDF and dominate similarity scores.

**Step 3 — Build TF vectors for each sentence**

For each sentence `i`, build a vector where dimension `w` holds:

```
tf(w, i) = count of word w in sentence i
```

(Raw term frequency, not log-normalized, as used in the original paper.)

**Step 4 — Compute IDF-modified cosine similarity between all sentence pairs**

For sentences `i` and `j`:

```
                 sum_{w in i ∩ j}  [ tf(w,i) * tf(w,j) * IDF(w)^2 ]
idf_cos(i,j) = -------------------------------------------------------
                sqrt( sum_{w in i} [tf(w,i)*IDF(w)]^2 )
              * sqrt( sum_{w in j} [tf(w,j)*IDF(w)]^2 )
```

This is standard cosine similarity where each term weight is `tf * idf` rather than raw tf. The IDF^2 in the numerator comes from multiplying the idf-weighted vectors of both sentences.

Only words shared between both sentences contribute to the numerator. If two sentences share no words, similarity = 0.

**Step 5 — Build the similarity matrix**

You now have an N×N matrix `C` where `C[i][j] = idf_cos(i, j)`. The matrix is symmetric because cosine similarity is symmetric. The diagonal is 1.0 (each sentence is identical to itself).

**Step 6 — Discrete LexRank vs Continuous LexRank (choose one)**

This is where the two variants diverge.

*Discrete LexRank* (threshold-based, recommended for CLI use):

Apply a threshold `t` (default 0.1 from the original paper) to binarize the similarity matrix:

```
B[i][j] = 1   if C[i][j] >= t
B[i][j] = 0   otherwise
```

Then build the stochastic matrix by normalizing each row by its degree (number of non-zero neighbors):

```
M[i][j] = B[i][j] / degree(i)
```

Where `degree(i) = sum_j B[i][j]`. If a sentence has no neighbors above threshold (degree = 0), assign uniform probability: `M[i][j] = 1/N` for all `j`.

*Continuous LexRank* (weighted):

Skip the threshold. Normalize the raw similarity matrix by row sums:

```
M[i][j] = C[i][j] / sum_k C[i][k]
```

Continuous LexRank preserves the gradation of similarity. Discrete is more robust against noisy text where many near-zero similarities would otherwise add noise. The original paper found discrete LexRank with threshold 0.1 performed best in practice.

**Step 7 — Apply power iteration to find the stationary distribution**

Initialize a uniform probability vector:

```
p[0] = [1/N, 1/N, ..., 1/N]
```

At each iteration `t`:

```
p[t+1] = M^T * p[t]
```

(Multiply the TRANSPOSE of the stochastic matrix by the current vector.)

Check for convergence:

```
error = sum_i | p[t+1][i] - p[t][i] |
if error < epsilon: stop
```

Typical epsilon: `1e-6`. Typical max iterations: 100. For N < 1000 sentences this converges in 10-30 iterations.

The resulting vector `p` is the stationary distribution — the LexRank score for each sentence.

**Step 8 — Select top-K sentences**

Sort sentences by their LexRank score descending. Take the top `K` (the `--sentences` parameter). To avoid redundancy in multi-document scenarios, apply Maximum Marginal Relevance: add a sentence to the summary only if its similarity to all already-selected sentences is below a redundancy threshold (e.g., 0.5).

**Step 9 — Reorder selected sentences**

Output sentences in their original document order, not sorted by score. This preserves narrative coherence.

### Key parameters

| Parameter | Default | Effect |
|-----------|---------|--------|
| `threshold` | 0.1 | Minimum cosine similarity to form an edge (discrete mode). Lower = more edges = more connected graph. Higher = sparser graph, only very similar sentences connected. Range: 0.0–1.0. |
| `continuous` | false | If true, use weighted edges instead of binary threshold |
| `epsilon` (convergence) | 1e-6 | Power iteration stops when L1 norm of score change < epsilon |
| `maxIterations` | 100 | Hard cap on power iteration steps |
| `damping` | 0.85 | In continuous mode, damping factor analogous to PageRank. Prevents getting trapped in sink nodes. Applied as: `p[t+1][i] = (1-d)/N + d * sum_j M[j][i] * p[t][j]` |
| `sentences` (K) | 5 | Number of sentences to extract |

### Pseudocode

```
function LexRank(text, K, threshold=0.1, epsilon=1e-6, maxIter=100):
    sentences = tokenize_sentences(text)
    N = len(sentences)

    // Build IDF
    idf = map[word -> log(N / sentence_frequency(word))]

    // Build TF vectors
    tf[i][w] = count of word w in sentence i

    // Build similarity matrix (IDF-modified cosine)
    for i in 0..N:
        for j in i..N:
            C[i][j] = idf_cosine(tf[i], tf[j], idf)
            C[j][i] = C[i][j]  // symmetric

    // Build stochastic matrix (discrete mode)
    for i in 0..N:
        degree_i = count of j where C[i][j] >= threshold
        if degree_i == 0:
            M[i][j] = 1/N  for all j  // dangling node
        else:
            M[i][j] = 1/degree_i  if C[i][j] >= threshold  else 0

    // Power iteration
    p = [1/N] * N
    for iter in 0..maxIter:
        p_new = M^T * p
        error = sum(|p_new[i] - p[i]| for i in 0..N)
        p = p_new
        if error < epsilon: break

    // Select top-K in original order
    ranked = sort sentences by p[i] descending
    selected = ranked[:K]
    return sort selected by original_position ascending
```

---

## TextRank Algorithm

### Conceptual foundation

TextRank (Mihalcea & Tarau, 2004) applies the PageRank algorithm directly to sentences. Where LexRank was specifically designed for multi-document summarization with IDF weighting, TextRank is a more general framework designed for single-document summarization.

The key difference in similarity: TextRank uses a word-overlap-based similarity normalized by sentence length (log-based), rather than IDF-modified cosine. This makes it cheaper to compute and more appropriate for single documents where IDF is less meaningful.

### Step-by-step

**Step 1 — Tokenize sentences**

Split text into sentences. Tokenize each into words, lowercase, remove stopwords.

**Step 2 — Compute pairwise sentence similarity**

For sentences `i` and `j`, similarity is the count of words they share divided by the sum of the logarithms of their lengths:

```
sim(i, j) = |{w : w in i AND w in j}|
            -------------------------
            log(|i|) + log(|j|)
```

Where `|i|` is the number of words in sentence `i`. The log normalization prevents long sentences from always winning due to having more words available to overlap with.

If `|i| <= 1` or `|j| <= 1`, set similarity to 0 to avoid log(0) or log(1)=0 division.

**Step 3 — Build the weighted similarity graph**

Create an N×N matrix `W` where `W[i][j] = sim(i, j)`. Unlike LexRank there is no threshold by default — all non-zero similarities form edges. The graph is directed (though similarity is symmetric, PageRank requires a directed graph convention).

**Step 4 — Convert to stochastic matrix**

Normalize each row by its row sum:

```
P[i][j] = W[i][j] / sum_k W[i][k]
```

If `sum_k W[i][k] == 0` (sentence has no overlap with any other), assign uniform: `P[i][j] = 1/N`.

**Step 5 — Apply PageRank with damping factor**

Initialize uniform scores:

```
score[i] = 1/N  for all i
```

At each iteration:

```
score_new[i] = (1 - d) / N  +  d * sum_j ( P[j][i] * score[j] )
```

Where `d` is the damping factor (default 0.85). The `(1-d)/N` term represents a random jump to any sentence with probability `1-d`, preventing convergence issues in disconnected or sparse graphs.

Check convergence:

```
delta = sum_i |score_new[i] - score[i]|
if delta < epsilon: stop
```

Typical epsilon: `1e-4`. Typical max iterations: 200.

**Step 6 — Select and reorder**

Same as LexRank: sort by score descending, take top-K, reorder to original document position.

### Key parameters

| Parameter | Default | Effect |
|-----------|---------|--------|
| `damping` (d) | 0.85 | Probability of following a similarity link vs. random jump. Range 0.8–0.9 recommended. Lower = more uniform scores, less differentiation. Higher = more concentrated on highly-connected sentences. |
| `epsilon` | 1e-4 | Convergence threshold. TextRank typically uses a looser epsilon than LexRank. |
| `maxIterations` | 200 | Hard cap. TextRank with weighted edges takes longer to converge than discrete LexRank. |
| `sentences` (K) | 5 | Number of sentences to extract |

Note: TextRank does NOT have a threshold parameter by default. All non-zero word overlaps create edges.

### Pseudocode

```
function TextRank(text, K, damping=0.85, epsilon=1e-4, maxIter=200):
    sentences = tokenize_sentences(text)
    N = len(sentences)
    words[i] = set of non-stopword tokens in sentence i

    // Build similarity matrix
    for i in 0..N:
        for j in i..N:
            overlap = |words[i] ∩ words[j]|
            if len(words[i]) <= 1 or len(words[j]) <= 1:
                W[i][j] = 0
            else:
                W[i][j] = overlap / (log(len(words[i])) + log(len(words[j])))
            W[j][i] = W[i][j]

    // Build stochastic matrix
    for i in 0..N:
        row_sum = sum(W[i][j] for j in 0..N)
        if row_sum == 0:
            P[i][j] = 1/N  for all j
        else:
            P[i][j] = W[i][j] / row_sum

    // PageRank iteration
    score = [1/N] * N
    for iter in 0..maxIter:
        score_new[i] = (1-damping)/N + damping * sum_j(P[j][i] * score[j])
        delta = sum(|score_new[i] - score[i]|)
        score = score_new
        if delta < epsilon: break

    // Select top-K in original order
    ranked = sort sentences by score[i] descending
    selected = ranked[:K]
    return sort selected by original_position ascending
```

---

## Comparison

### Similarity metric

| Aspect | LexRank | TextRank |
|--------|---------|---------|
| Similarity function | IDF-modified cosine similarity | Word overlap / log(len) |
| Word weighting | TF * IDF (rare words weighted higher) | Uniform (all shared words equal) |
| Normalization | L2 norm (cosine) | Log of sentence lengths |
| Threshold | Explicit threshold (discrete mode) | None by default |
| Graph type | Undirected (cosine is symmetric) | Directed (PageRank convention) |
| Damping | Optional (continuous mode) | Required |
| Designed for | Multi-document clusters | Single documents |
| Computational cost | Higher (IDF matrix, cosine on vectors) | Lower (word set intersection) |

### When to use LexRank

**Multi-document input or long documents with topic drift.** LexRank's IDF weighting means words that appear across many sentences are down-weighted. This helps when a document is heterogeneous and you want sentences from the "core" topic rather than from recurring boilerplate.

**Technical text.** IDF gives extra weight to rare technical terms shared between sentences, which is exactly what matters for developer documentation, API references, and technical articles.

**When redundancy control is needed.** The threshold creates a sparser graph that is less susceptible to clusters of near-duplicate sentences all voting for each other.

**Multi-document summarization (transcripts, aggregated docs).** LexRank was specifically designed and evaluated for this use case (DUC 2004).

**When the text is noisy** (e.g., auto-generated YouTube transcripts with filler phrases). The threshold filters out weak connections, making the result more robust to noise.

Recommended default for tldt: LexRank is the stronger choice for the primary use cases (YouTube transcripts, technical articles, developer docs).

### When to use TextRank

**Short single-document text** (400–2000 words, one coherent topic). The log-normalized word overlap works well when IDF is less informative because the corpus is small.

**Non-technical prose.** News articles, blog posts, narrative text where common words are intentionally repeated as rhetorical structure. IDF in LexRank would down-weight these intentional repetitions.

**Speed-critical paths.** TextRank's similarity function is cheaper: set intersection vs. vector dot products with IDF weights. For very large inputs (50k+ tokens) this matters.

**When you want predictable behavior.** TextRank has fewer parameters. The damping factor is well-understood from PageRank. LexRank's threshold requires tuning for different text types.

### Practical performance guidance

From research literature (Springer comparative analysis, DUC 2004 evaluations):

- LexRank consistently outperforms TextRank on multi-document and heterogeneous text (ROUGE metrics)
- TextRank is competitive on single short documents
- For developer CLI use (article summarization before pasting to AI), LexRank is the better default
- When in doubt: run both and let the user pick via `--algorithm`

---

## CLI Architecture

### Input pipeline

The input pipeline should be a single `io.Reader` handed to a `Summarizer` interface. Three entry points, one codepath:

```
stdin pipe       ─┐
-f file path     ─┼─> io.Reader ─> readInput() ─> string ─> Summarize()
positional arg   ─┘
```

Priority order: if `-f` is provided, use file. If positional arg is provided and no pipe, use arg. If stdin is a pipe (detected via `os.Stdin.Stat()`), use stdin.

```go
type Summarizer interface {
    Summarize(text string, sentences int, opts Options) (Summary, error)
}

type Options struct {
    Threshold   float64  // LexRank only, default 0.1
    Damping     float64  // TextRank and continuous LexRank, default 0.85
    Continuous  bool     // LexRank: use continuous (weighted) mode
    MaxIter     int      // default 100 (LexRank) / 200 (TextRank)
    Epsilon     float64  // convergence, default 1e-6 (LexRank) / 1e-4 (TextRank)
}

type Summary struct {
    Sentences      []string
    OriginalTokens int
    SummaryTokens  int
    Ratio          float64  // SummaryTokens / OriginalTokens
}
```

Algorithm selection via a constructor, not interface splitting:

```go
func NewSummarizer(algorithm string) (Summarizer, error)
```

### Output formatting

Default output: sentences joined with a space or newline, preceded by a stats line.

```
[tldt] 3847 tokens -> 312 tokens (8.1% of original)

First selected sentence. Second selected sentence. Third selected sentence.
```

With `--paragraphs N` flag: group the selected sentences into N paragraphs (see below).

With `--json` flag (optional future addition):

```json
{
  "original_tokens": 3847,
  "summary_tokens": 312,
  "ratio": 0.081,
  "sentences": ["...", "...", "..."]
}
```

### Paragraph grouping

The `--paragraphs N` flag groups the selected K sentences into N paragraphs. Two valid strategies:

**Strategy A — Position-based grouping (recommended, simpler):**

After selecting top-K sentences and reordering by original position, divide them evenly into N consecutive groups. Sentences keep their narrative order within each paragraph.

```
selected = [s2, s7, s12, s18, s24]  // 5 sentences, sorted by position
paragraphs = 2
group 1: [s2, s7, s12]
group 2: [s18, s24]
```

This works because original-position reordering means consecutive selected sentences are likely topically adjacent.

**Strategy B — Similarity-based clustering:**

After scoring, cluster the N highest-scored sentences by similarity using k-means or simple agglomerative clustering on their TF-IDF vectors. Assign each cluster as a paragraph.

This is more complex and not significantly better for CLI use. Implement Strategy A first.

Implementation:

```go
func groupIntoParagraphs(sentences []string, n int) [][]string {
    if n <= 0 || n >= len(sentences) {
        return [][]string{sentences}
    }
    groupSize := len(sentences) / n
    remainder := len(sentences) % n
    groups := make([][]string, n)
    idx := 0
    for i := 0; i < n; i++ {
        size := groupSize
        if i < remainder {
            size++
        }
        groups[i] = sentences[idx : idx+size]
        idx += size
    }
    return groups
}
```

Output: paragraphs separated by a blank line.

---

## Token Estimation

### Why estimate tokens

tldt's core value proposition is token savings. Users need to see the compression ratio to know whether the summary is worth using. The estimate does not need to be exact — it needs to be consistent and directionally accurate.

### The chars/4 rule

The common approximation: `tokens ≈ len(text) / 4`

This holds reasonably for English prose. It breaks down for:
- Source code (operators, identifiers: often closer to chars/3)
- Non-English text (especially CJK where 1 character can be 1+ tokens)
- Mixed content with many special characters

For tldt's primary use cases (English developer content: articles, transcripts, docs), chars/4 gives approximately ±15% error compared to actual tiktoken counts. This is acceptable for a "how much am I saving?" indicator.

### Recommended implementation

```go
// EstimateTokens returns a rough token count estimate.
// Uses chars/4 for English prose, with a floor of 1 per word as a sanity check.
func EstimateTokens(text string) int {
    byChars := len(text) / 4
    // Sanity floor: count whitespace-separated tokens
    words := len(strings.Fields(text))
    if byChars < words {
        return words  // very short/sparse text
    }
    return byChars
}
```

For the output display:

```go
original := EstimateTokens(inputText)
summary  := EstimateTokens(summaryText)
ratio    := float64(summary) / float64(original) * 100
fmt.Printf("[tldt] ~%d tokens -> ~%d tokens (%.1f%% of original)\n",
    original, summary, ratio)
```

The `~` prefix signals approximation to users.

### Accuracy note

For production use where exact token billing matters, the tiktoken library (or `go-tiktoken`) should be used. For tldt's purpose — showing developers the savings before pasting to an AI — the chars/4 estimate is sufficient and avoids any external dependency.

A future `--exact-tokens` flag could invoke a tiktoken-compatible library for users who want precision.

---

## Go Library Assessment

### Existing Go libraries for LexRank / TextRank

**github.com/didasy/tldr** (also at github.com/JesusIslam/tldr, 137 stars)
- Implements something it calls LexRank but uses Hamming distance or Jaccard coefficient as similarity metrics, not IDF-modified cosine. Uses PageRank externally via `github.com/alixaxel/pagerank`. This is NOT a correct LexRank implementation per the Erkan & Radev paper.
- The existing `summary.go` in tldt already uses this library.
- **Verdict:** Do not rely on this for correct LexRank. Useful only as a TextRank-adjacent baseline.

**github.com/DavidBelicza/TextRank** (223 stars)
- Legitimate TextRank implementation. Handles tokenization, stop words, multi-language. Has goroutine support.
- API is more complex than needed for a CLI tool.
- **Verdict:** Viable for TextRank backend to avoid reimplementing from scratch. Evaluate in Phase 1.

**Native implementation recommendation:**
Given that neither library provides correct LexRank (IDF-modified cosine + eigenvector centrality), and TextRank can be implemented in ~150 lines of Go, implementing both natively is the right call. Benefits: correctness, no hidden dependencies, full parameter exposure, testability.

The matrix operations required (N×N float64 matrices, vector multiplication, L1 norm) are straightforward with `gonum.org/v1/gonum/mat` or plain Go slices for N < 5000.

### Recommended dependency strategy

```
Core algorithms: implement natively (LexRank, TextRank)
Sentence tokenization: use "github.com/neurosnap/sentences" or simple rule-based
Stop words: embed a static list (no external dep)
Matrix ops: plain [][]float64 slices (avoid gonum for this scale)
Token estimation: chars/4, no external dep
```

---

## Sources

- Erkan, G. & Radev, D. (2004). LexRank: Graph-based Lexical Centrality as Salience in Text Summarization. JAIR. https://www.cs.cmu.edu/afs/cs/project/jair/pub/volume22/erkan04a-html/erkan04a.html
- Mihalcea, R. & Tarau, P. (2004). TextRank: Bringing Order into Texts. EMNLP. https://web.eecs.umich.edu/~mihalcea/papers/mihalcea.emnlp04.pdf
- LexRank algorithm step-by-step tutorial: https://maciejzalwert.medium.com/lexrank-algorithm-explained-a-step-by-step-tutorial-with-examples-3d3aa0297c57
- LexRank Python implementation (crabcamp): https://github.com/crabcamp/lexrank
- TextRank Python implementation (summanlp): https://github.com/summanlp/textrank
- LexRank vs TextRank comparison (OpenGenus): https://iq.opengenus.org/lexrank-text-summarization/
- TextRank overview (OpenGenus): https://iq.opengenus.org/textrank-for-text-summarization/
- Comparative analysis (Springer): https://link.springer.com/chapter/10.1007/978-981-97-0180-3_30
- Go TextRank library: https://github.com/DavidBelicza/TextRank
- Go tldr/LexRank library: https://github.com/didasy/tldr
- Token counting approximation: https://help.openai.com/en/articles/4936856-what-are-tokens-and-how-to-count-them
- Tiktoken accuracy analysis: https://galileo.ai/blog/tiktoken-guide-production-ai
