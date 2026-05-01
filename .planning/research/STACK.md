# Stack Research — Go NLP / Summarization Libraries

**Project:** tldt — CLI tool for extractive text summarization (YouTube transcripts, articles)
**Researched:** 2026-05-01
**Overall confidence:** HIGH for library evaluation, MEDIUM for LexRank scratch implementation scope

---

## Existing Library Evaluation

### JesusIslam/tldr

- **Module path:** `github.com/JesusIslam/tldr` (original import path; repository was transferred to didasy)
- **Algorithm:** LexRank — graph-based sentence ranking. NOT TextRank. Two weighing steps (Jaccard or Hamming distance) followed by a ranking step (PageRank via `github.com/alixaxel/pagerank`, or centrality). Default is Hamming + PageRank.
- **API:**
  ```go
  bag := tldr.New()
  summary, err := bag.Summarize(body, numSentences)
  // extended config:
  bag.Set(maxChars, damping, tolerance, threshold, sentenceThreshold, algorithm, weighing)
  ```
- **Thread safety:** NOT thread safe. The struct holds mutable state. Concurrent use requires a new instance per goroutine or external locking.
- **Maintenance status:** The repository at `github.com/JesusIslam/tldr` redirects to `github.com/didasy/tldr`. Last commit was October 2025 (documentation + test coverage updates, performance optimizations). Actively maintained by a new maintainer (didasy).
- **Stars/forks:** 137 stars, 19 forks (both repos combined).
- **Go module:** Has a valid `go.mod`. Version tagged at v0.7.0.
- **Dependencies:** `github.com/alixaxel/pagerank` (for the PageRank ranking path), `github.com/onsi/ginkgo` and `github.com/onsi/gomega` (test-only).
- **Performance:** ~885–962 ns/op on AMD Ryzen, ~7 µs/op on older i3. Well within acceptable range for CLI batch processing.
- **Verdict:** This IS the canonical library. The current project already uses it. The import path `github.com/JesusIslam/tldr` still resolves (GitHub redirect) but the canonical path is now `github.com/didasy/tldr`.

### didasy/tldr

This IS `JesusIslam/tldr`. The repository transferred ownership from JesusIslam to didasy. They are the same codebase. The October 2025 commits updated README references to reflect the new owner. `github.com/JesusIslam/tldr` is a redirect/alias to `github.com/didasy/tldr`. The module API, algorithm, and behavior are identical. The project's import path in go.mod should be updated from `JesusIslam` to `didasy` to track the canonical location.

### Other candidates

**DavidBelicza/TextRank** (`github.com/DavidBelicza/TextRank`)
- Algorithm: TextRank (NOT LexRank). Graph-based using relation-weight and chain-based weighting, not cosine similarity.
- Minimum Go version: 1.8.
- Latest version: v2 (stable, Go modules supported).
- Stars: 223 stars, 23 forks. Last commit June 2025.
- Thread safety: Supports goroutines (multi-threaded by design).
- API: More verbose — requires creating a TextRank object, defining parsing rules, populating text, running Rank(), then using finder functions. Significantly higher API complexity than didasy/tldr.
- Differentiator: Phrase extraction in addition to sentence extraction. Multi-language via configurable stop word lists.
- Verdict: More complex API for roughly equivalent sentence-extraction quality. TextRank and LexRank produce similar quality summaries on general text. Not recommended as a replacement — more suited to keyword/phrase extraction use cases.

**crabcamp/lexrank** (`github.com/crabcamp/lexrank`)
- This is a Python library (99.1% Python). Not a Go library. Disqualified.

**bguvenc/LexRank** (`github.com/bguvenc/LexRank`)
- Python implementation. Not Go. Disqualified.

**james-bowman/nlp** (`github.com/james-bowman/nlp`)
- Not a summarization library. Provides LSA/LSI, LDA, TF-IDF, Random Indexing, SimHash. The LSA approach could theoretically produce summary-like output but is not extractive summarization. Useful as a primitives library.
- 471 stars, well-maintained, built on Gonum. MEDIUM confidence on maintenance status (last update not confirmed).

**No other native Go LexRank library exists** beyond the didasy/tldr lineage. The search across pkg.go.dev, GitHub topics `lexrank` and `summarizer`, and general web search found zero other implementations.

---

## Recommendation

### Use `github.com/didasy/tldr` (updated import path)

**Do not implement LexRank from scratch.** The didasy/tldr library is the only Go LexRank implementation in existence, it is actively maintained (October 2025), and it already works in the project. The current `summary.go` file uses `github.com/JesusIslam/tldr` which resolves via redirect, but the import path should be updated to `github.com/didasy/tldr` to avoid depending on a GitHub redirect.

**Why not scratch:**
1. LexRank requires: sentence tokenization, TF-IDF vectorization, IDF computation across the full corpus, cosine similarity matrix construction (O(n²) sentence pairs), either power iteration for eigenvector centrality or a PageRank computation, and result deduplication. This is 400–800 lines of correct numerical code.
2. The existing library does all of this, benchmarks at sub-microsecond per operation, and was recently optimized (October 2025 "performance optimization" commits).
3. For the target use case (YouTube transcripts, articles of 1,000–50,000 words), the library's correctness and maintenance track record outweigh any theoretical customization benefit.

**Why not DavidBelicza/TextRank:**
- TextRank and LexRank produce comparable quality on general text. There is no quality reason to switch.
- The API is more complex with more setup steps.
- The current code is already working with LexRank. Switching to TextRank is a lateral move with extra work.

**Action items:**
1. Update `summary.go` import from `github.com/JesusIslam/tldr` to `github.com/didasy/tldr`.
2. Update `go.mod` accordingly.
3. Consider exposing `bag.Set()` configuration in the HTTP handler or CLI to allow callers to tune `algorithm` (centrality vs pagerank) and `weighing` (hamming vs jaccard) for different text types.
4. Address thread safety: the current HTTP handler creates a new `bag` per request (via `tldr.New()`), which is correct. Do not share a single instance across requests.

---

## Go NLP Primitives Available

### Tokenization

**Sentence tokenization:**
- `github.com/neurosnap/sentences` — Pure Go, NLTK punkt-based, zero dependencies, multilingual (13 languages), 98.95% accuracy on Brown Corpus, 467 stars. Best option for sentence boundary detection if needed outside of didasy/tldr.
- `github.com/clipperhouse/uax29` — Unicode Standard Annex #29 compliant segmentation for words, sentences, phrases, graphemes. Stable at v1.16.0. Explicitly designed for "TF-IDF, BM25, embeddings" use cases. Best option for UAX-29 standards compliance.
- `github.com/jdkato/prose/v2` — Archived (May 2023, read-only). Do NOT use for new code. 3.1k stars but unmaintained.

**Word tokenization:**
- `github.com/clipperhouse/uax29/words` — Unicode-correct word segmentation. Handles apostrophes, hyphens, contractions properly per spec. Recommended.
- Standard library `strings.Fields` — Adequate for simple ASCII tokenization in performance-critical paths. Not Unicode-safe for all languages.

### TF-IDF

- `github.com/go-nlp/tfidf` — Lingo-friendly API. `New()`, `Add(doc)`, `CalculateIDF()`, `Score(term, doc)`. Thread-safe (mutex). Stable v1. Integrates with tensor operations for cosine similarity. **Best pick** for standalone TF-IDF.
- `github.com/dkgv/go-tf-idf` — Smaller implementation, includes cosine similarity comparison. Good for embedded use.
- `github.com/wilcosheh/tfidf` — Includes cosine similarity computation, Chinese tokenizer support. More complex.
- `github.com/james-bowman/nlp` — Production-grade TF-IDF as part of LSA pipeline. Higher dependency weight (requires Gonum). Use if already pulling in Gonum for matrix operations.

### Cosine similarity

For LexRank from scratch, cosine similarity between sentence TF-IDF vectors is the core computation. Options:

- **Manual implementation** (recommended for this use case): cosine similarity between two float64 slices is ~10 lines of Go. No external dependency needed. For n sentences, build an n×n matrix with nested loops. For typical transcript sizes (100–500 sentences), O(n²) is fast enough.
- `github.com/go-nlp/tfidf` — Includes cosine similarity examples in documentation.
- `github.com/rioloc/tfidf-go/similarity` — `CosineSimilarity` struct with configurable tokenizer and vectorizer.
- `gonum.org/v1/gonum/mat` — Full dense/sparse matrix support. For large documents (1000+ sentences), sparse matrix representation of the similarity graph reduces memory. `gonum.org/v1/gonum/graph/network` includes `PageRank()` function directly usable on a similarity graph.

**If implementing LexRank from scratch, the minimal dependencies are:**
1. Sentence tokenizer: `github.com/neurosnap/sentences` or `github.com/clipperhouse/uax29`
2. Word tokenizer: `github.com/clipperhouse/uax29/words` or `strings.Fields`
3. TF-IDF: `github.com/go-nlp/tfidf` or manual implementation (~50 lines)
4. Cosine similarity: manual implementation (~10 lines)
5. PageRank/eigenvector: `github.com/alixaxel/pagerank` (same dependency didasy/tldr already uses) or `gonum.org/v1/gonum/graph/network` (PageRank function available)

---

## Dependencies to add to go.mod

**Recommended: update import path only (minimal change)**

```
# Remove or rename:
github.com/JesusIslam/tldr

# Add:
github.com/didasy/tldr v0.7.0
```

**If exposing algorithm configuration, no additional dependencies needed.** The didasy/tldr package already exposes `Set()` for algorithm and weighing selection.

**If adding a dedicated sentence tokenizer for pre-processing or chunking:**

```
github.com/neurosnap/sentences v1.0.6    # NLTK punkt-based, zero deps, multilingual
# OR
github.com/clipperhouse/uax29 v1.16.0   # UAX-29 Unicode standard, words + sentences
```

**If implementing LexRank from scratch (not recommended but fully feasible):**

```
github.com/go-nlp/tfidf v1.0.1          # TF-IDF computation
github.com/alixaxel/pagerank v0.0.0-...  # PageRank on similarity graph
github.com/neurosnap/sentences v1.0.6    # Sentence boundary detection
github.com/clipperhouse/uax29 v1.16.0   # Unicode word tokenization
```

The total from-scratch implementation requires approximately 400–600 lines of Go and 3–4 days of work to match the quality of didasy/tldr. The library approach is strictly preferred.

---

## Sources

- https://github.com/didasy/tldr (canonical repo, last commit October 2025)
- https://github.com/JesusIslam/tldr (redirect, same repo)
- https://pkg.go.dev/github.com/didasy/tldr
- https://pkg.go.dev/github.com/JesusIslam/tldr
- https://github.com/DavidBelicza/TextRank (TextRank alternative, June 2025)
- https://pkg.go.dev/github.com/DavidBelicza/TextRank
- https://pkg.go.dev/github.com/go-nlp/tfidf
- https://github.com/james-bowman/nlp
- https://pkg.go.dev/github.com/alixaxel/pagerank
- https://pkg.go.dev/gonum.org/v1/gonum/graph/network
- https://github.com/neurosnap/sentences
- https://github.com/clipperhouse/uax29
- https://www.cs.cmu.edu/afs/cs/project/jair/pub/volume22/erkan04a-html/erkan04a.html (original LexRank paper)
