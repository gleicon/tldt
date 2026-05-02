# Phase 2: Algorithms - Pattern Map

**Mapped:** 2026-05-01
**Files analyzed:** 9 (6 new, 2 extended, 1 unchanged)
**Analogs found:** 9 / 9

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/summarizer/summarizer.go` | registry/interface | request-response | `internal/summarizer/graph.go` | role-match |
| `internal/summarizer/tokenizer.go` | utility | transform | `github.com/didasy/tldr@v0.7.0/tldr.go` `TokenizeSentences` | role-match |
| `internal/summarizer/lexrank.go` | service | transform | `internal/summarizer/graph.go` | role-match |
| `internal/summarizer/textrank.go` | service | transform | `internal/summarizer/graph.go` | role-match |
| `internal/summarizer/lexrank_test.go` | test | — | `internal/summarizer/graph_test.go` | exact |
| `internal/summarizer/textrank_test.go` | test | — | `internal/summarizer/graph_test.go` | exact |
| `internal/summarizer/tokenizer_test.go` | test | — | `internal/summarizer/graph_test.go` | exact |
| `internal/summarizer/integration_test.go` | test (extend) | — | `internal/summarizer/integration_test.go` | exact (self) |
| `cmd/tldt/main.go` | CLI entry point (extend) | request-response | `cmd/tldt/main.go` | exact (self) |

---

## Pattern Assignments

### `internal/summarizer/summarizer.go` (registry/interface, request-response)

**Analog:** `internal/summarizer/graph.go` (lines 1–14) — same package, same one-function pattern; we extend to a full interface + registry.

**Package declaration pattern** (`graph.go` line 1):
```go
package summarizer
```

**Import pattern** — graph.go uses a single import; summarizer.go needs only `fmt`:
```go
package summarizer

import "fmt"
```

**Summarizer interface + registry** — derived directly from D-02 (locked decision):
```go
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

**Graph struct wrapper** — wraps the existing package-level `Summarize()` in `graph.go` to satisfy the interface. Add to `graph.go` (not summarizer.go) so the struct lives next to its implementation:
```go
// Graph wraps the package-level Summarize() to satisfy the Summarizer interface.
type Graph struct{}

func (g *Graph) Summarize(text string, n int) ([]string, error) {
    return Summarize(text, n)
}
```

---

### `internal/summarizer/tokenizer.go` (utility, transform)

**Analog:** `github.com/didasy/tldr@v0.7.0/tldr.go` — `createSentences` and `createDictionary` methods (lines 353–403) show the word normalization pattern. The sentence tokenizer approach is locked by D-04.

**Package and import pattern** (matches all files in package):
```go
package summarizer

import (
    "regexp"
    "strings"
)
```

**Sentence tokenizer — D-04 locked pattern:**
```go
var sentenceEnd = regexp.MustCompile(`[.!?]["'"]?\s+[A-Z]`)

// TokenizeSentences splits text into sentences using a regexp heuristic.
// Sentences are returned trimmed, in original order.
// Returns nil for empty input.
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

**Word normalization pattern** — copy from `tldr.go` `createDictionary` (lines 374–403): lowercase + strip non-alphanumeric (keep hyphens between digits/letters):
```go
// normalizeWord lowercases and strips non-alphanumeric characters from a word.
// Hyphens between digit/letter sequences are preserved.
func normalizeWord(word string) string {
    word = strings.ToLower(word)
    var prev rune
    return strings.Map(func(r rune) rune {
        if r == '-' && (unicode.IsDigit(prev) || unicode.IsLetter(prev)) {
            prev = r
            return r
        }
        if !unicode.IsDigit(r) && !unicode.IsLetter(r) {
            return -1
        }
        prev = r
        return r
    }, word)
}

// tokenizeWords returns the normalized, non-empty words for a sentence.
func tokenizeWords(sentence string) []string {
    raw := strings.Fields(sentence)
    out := make([]string, 0, len(raw))
    for _, w := range raw {
        if n := normalizeWord(w); n != "" {
            out = append(out, n)
        }
    }
    return out
}
```

Additional import needed: `"unicode"` (used by `normalizeWord`).

---

### `internal/summarizer/lexrank.go` (service, transform)

**Analog:** `internal/summarizer/graph.go` (lines 1–14) — same struct pattern: zero-field struct, one exported method `Summarize(text string, n int) ([]string, error)`.

**Package and import pattern:**
```go
package summarizer

import (
    "math"
    "sort"
)
```

**Struct declaration pattern** — copy stateless zero-struct from `graph.go`:
```go
// LexRank implements Summarizer using IDF-modified cosine similarity
// and power iteration (Erkan & Dragomir 2004).
// LexRank is stateless; a single instance may be reused across calls.
type LexRank struct{}
```

**Summarize method signature** — must exactly match existing `graph.go` package-level function signature:
```go
func (l *LexRank) Summarize(text string, n int) ([]string, error) {
    sentences := TokenizeSentences(text)
    if len(sentences) == 0 {
        return nil, nil
    }
    if n > len(sentences) {
        n = len(sentences) // SUM-04: silent cap
    }
    // ... algorithm steps ...
    return result, nil
}
```

**Deterministic vocabulary construction pattern** (from RESEARCH.md Code Examples):
```go
func buildVocabAndIDF(sentences [][]string) ([]string, []float64) {
    N := len(sentences)
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
    // Sort once — never range over df in scoring path
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

**IDF-modified cosine similarity** (from RESEARCH.md Code Examples):
```go
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

**Row normalization pattern** (from RESEARCH.md):
```go
func rowNormalize(matrix [][]float64) {
    n := len(matrix)
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
            // dangling row: uniform probability
            for j := range matrix[i] {
                matrix[i][j] = 1.0 / float64(n)
            }
        }
    }
}
```

**Power iteration** (from RESEARCH.md Code Examples, matches `didasy/tldr` DEFAULT_TOLERANCE=0.0001):
```go
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
```

**Top-N selection — document order restoration pattern** (from RESEARCH.md):
```go
type scored struct {
    idx   int
    score float64
}

func selectTopN(scores []float64, n int, sentences []string) []string {
    ranked := make([]scored, len(scores))
    for i, s := range scores {
        ranked[i] = scored{i, s}
    }
    // STABLE sort: deterministic tie-breaking
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
    sort.Ints(top) // restore document order (SUM-05)
    result := make([]string, n)
    for i, idx := range top {
        result[i] = sentences[idx]
    }
    return result
}
```

---

### `internal/summarizer/textrank.go` (service, transform)

**Analog:** `internal/summarizer/lexrank.go` (will be created in same phase) — same struct/method pattern. Secondary analog: `internal/summarizer/graph.go`.

**Package and import pattern:**
```go
package summarizer

import (
    "math"
    "sort"
)
```

**Struct declaration pattern** — identical to LexRank:
```go
// TextRank implements Summarizer using word-overlap similarity
// and PageRank-style power iteration (Mihalcea & Tarau 2004).
// TextRank is stateless; a single instance may be reused across calls.
type TextRank struct{}
```

**Summarize method — same signature as LexRank and graph.go:**
```go
func (t *TextRank) Summarize(text string, n int) ([]string, error) {
    sentences := TokenizeSentences(text)
    if len(sentences) == 0 {
        return nil, nil
    }
    if n > len(sentences) {
        n = len(sentences) // SUM-04: silent cap
    }
    // ... algorithm steps ...
    return result, nil
}
```

**Word overlap similarity** (from RESEARCH.md Code Examples — Mihalcea & Tarau 2004):
```go
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

**Power iteration with TextRank damping** — uses same `powerIterate` helper as LexRank (share or duplicate the function; both files are in the same package so a shared unexported function in a helper file or either file is fine). Apply damping in the iteration:
```go
// For TextRank, incorporate damping d=0.85 directly in the power step:
// score(i) = (1-d)/n  +  d * sum_j(matrix[j][i] * score[j])
const textRankDamping = 0.85

func powerIterateDamped(matrix [][]float64, damping, epsilon float64, maxIter int) []float64 {
    n := len(matrix)
    p := make([]float64, n)
    for i := range p {
        p[i] = 1.0 / float64(n)
    }
    base := (1.0 - damping) / float64(n)
    for iter := 0; iter < maxIter; iter++ {
        next := make([]float64, n)
        for i := range next {
            next[i] = base
            for j := range p {
                next[i] += damping * matrix[j][i] * p[j]
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
```

**Top-N selection** — use same `selectTopN` helper as defined for LexRank (shared unexported function in the same package).

---

### `internal/summarizer/lexrank_test.go` (test)

**Analog:** `internal/summarizer/graph_test.go` (lines 1–68) — same package, same `package summarizer` declaration (white-box tests), same constant test-text pattern, same `t.Fatalf` / `t.Errorf` style.

**Package and import pattern** (`graph_test.go` lines 1–6):
```go
package summarizer

import (
    "math"
    "testing"
)
```

**Constant test data pattern** (`graph_test.go` lines 8–18):
```go
const tenSentenceText = `...`   // multi-sentence for full algorithm path
const threeSentenceText = `...` // 3 sentences for edge cases
```

**Unit test structure** — copy error-check-first pattern from `graph_test.go` lines 25–33:
```go
func TestLexRank_TFIDFVectors(t *testing.T) {
    // known input → expected IDF values (TEST-01)
    sentences := [][]string{{"the", "cat"}, {"the", "dog"}}
    vocab, idf := buildVocabAndIDF(sentences)
    // "the" appears in both sentences: IDF = log(2/2) = 0
    // "cat" appears in 1: IDF = log(2/1) = log(2) ≈ 0.693
    wantIdx := 0 // "cat" is first alphabetically
    _ = vocab
    if math.Abs(idf[wantIdx]-math.Log(2)) > 0.001 {
        t.Errorf("idf[cat] = %f, want ~%f", idf[wantIdx], math.Log(2))
    }
}

func TestLexRank_CosineIdentical(t *testing.T) {
    // identical vectors → similarity = 1.0 (TEST-02)
    v := []float64{1.0, 0.5}
    idf := []float64{1.0, 1.0}
    got := idfCosine(v, v, idf)
    if math.Abs(got-1.0) > 0.001 {
        t.Errorf("idfCosine(v,v) = %f, want 1.0", got)
    }
}

func TestLexRank_CosineOrthogonal(t *testing.T) {
    // orthogonal vectors → similarity = 0.0 (TEST-02)
    v1 := []float64{1.0, 0.0}
    v2 := []float64{0.0, 1.0}
    idf := []float64{1.0, 1.0}
    got := idfCosine(v1, v2, idf)
    if math.Abs(got) > 0.001 {
        t.Errorf("idfCosine(orthogonal) = %f, want 0.0", got)
    }
}
```

**Power iteration unit test pattern** (TEST-03, from RESEARCH.md Code Examples):
```go
func TestPowerIterate_UniformMatrix(t *testing.T) {
    n := 3
    m := make([][]float64, n)
    for i := range m {
        m[i] = []float64{1.0 / 3, 1.0 / 3, 1.0 / 3}
    }
    got := powerIterate(m, 0.0001, 1000)
    for i, v := range got {
        if math.Abs(v-1.0/3) > 0.001 {
            t.Errorf("scores[%d] = %f, want ~0.333", i, v)
        }
    }
}
```

**Determinism test pattern** (TEST-06):
```go
func TestLexRank_Deterministic(t *testing.T) {
    lr := &LexRank{}
    r1, err1 := lr.Summarize(tenSentenceText, 3)
    r2, err2 := lr.Summarize(tenSentenceText, 3)
    if err1 != nil || err2 != nil {
        t.Fatalf("unexpected errors: %v, %v", err1, err2)
    }
    for i := range r1 {
        if r1[i] != r2[i] {
            t.Errorf("non-deterministic output at index %d: %q vs %q", i, r1[i], r2[i])
        }
    }
}
```

---

### `internal/summarizer/textrank_test.go` (test)

**Analog:** `internal/summarizer/lexrank_test.go` (same phase, same pattern) and `internal/summarizer/graph_test.go`.

**Package and import pattern** — identical to lexrank_test.go:
```go
package summarizer

import (
    "math"
    "testing"
)
```

**Word overlap unit test** (TEST-04 supporting):
```go
func TestWordOverlapSim_CommonWords(t *testing.T) {
    s1 := []string{"the", "cat", "sat"}
    s2 := []string{"the", "cat", "ran"}
    got := wordOverlapSim(s1, s2)
    // 2 common words / (log(3)+log(3)) = 2 / (2*1.099) ≈ 0.91
    if got <= 0 || got > 1.0 {
        t.Errorf("wordOverlapSim = %f, want (0,1]", got)
    }
}

func TestWordOverlapSim_NoOverlap(t *testing.T) {
    s1 := []string{"cat", "sat"}
    s2 := []string{"dog", "ran"}
    got := wordOverlapSim(s1, s2)
    if got != 0.0 {
        t.Errorf("wordOverlapSim(disjoint) = %f, want 0.0", got)
    }
}

func TestWordOverlapSim_SingleWord(t *testing.T) {
    // edge case: len <= 1 → 0 (avoid log(1)=0 division by zero)
    got := wordOverlapSim([]string{"cat"}, []string{"cat", "dog"})
    if got != 0.0 {
        t.Errorf("wordOverlapSim(single) = %f, want 0.0", got)
    }
}
```

**Determinism test** — same pattern as lexrank_test.go, using `&TextRank{}`.

---

### `internal/summarizer/tokenizer_test.go` (test)

**Analog:** `internal/summarizer/graph_test.go` (lines 1–6, package/import, test function structure).

**Package and import pattern:**
```go
package summarizer

import (
    "testing"
)
```

**Edge case test pattern** (TEST-05):
```go
func TestTokenizeSentences_Empty(t *testing.T) {
    got := TokenizeSentences("")
    if got != nil {
        t.Errorf("TokenizeSentences(\"\") = %v, want nil", got)
    }
}

func TestTokenizeSentences_Single(t *testing.T) {
    got := TokenizeSentences("Just one sentence.")
    if len(got) != 1 {
        t.Errorf("got %d sentences, want 1", len(got))
    }
}

func TestTokenizeSentences_MultiSentence(t *testing.T) {
    text := "First sentence. Second sentence. Third sentence."
    got := TokenizeSentences(text)
    if len(got) < 2 {
        t.Errorf("got %d sentences from 3-sentence input, want >=2", len(got))
    }
}

func TestTokenizeSentences_Unicode(t *testing.T) {
    // Unicode content should not panic (TEST-05)
    text := "Héllo world. Wörld is great. End here."
    got := TokenizeSentences(text)
    if len(got) == 0 {
        t.Error("TokenizeSentences returned empty for unicode input")
    }
}
```

---

### `internal/summarizer/integration_test.go` (extend existing)

**Analog:** `internal/summarizer/integration_test.go` (lines 1–85) — self-analog; extend by adding new test functions using the same `readTestFile` / `repoRoot` helpers.

**Existing helper pattern to reuse** (lines 12–33):
```go
// repoRoot and readTestFile are already defined; do not redefine.
// Add new test functions using the same helpers:

func TestNew_LexRank_WikipediaEn(t *testing.T) {
    text := readTestFile(t, "wikipedia_en.txt")
    s, err := New("lexrank")
    if err != nil {
        t.Fatalf("New(lexrank) error: %v", err)
    }
    result, err := s.Summarize(text, 5)
    if err != nil {
        t.Fatalf("LexRank.Summarize error: %v", err)
    }
    if len(result) == 0 {
        t.Fatal("LexRank returned empty slice")
    }
}
```

**Error check pattern** (copy from lines 36–44):
```go
if err != nil {
    t.Fatalf("Summarize(%s) returned error: %v", fixture, err)
}
if len(result) == 0 {
    t.Fatal("Summarize returned empty slice")
}
```

**TEST-04 matrix:** Add one test function per algorithm × fixture combination: `New("lexrank")` and `New("textrank")` called with `wikipedia_en.txt`, `youtube_transcript.txt`, `longform_3000.txt`, `edge_short.txt`.

---

### `cmd/tldt/main.go` (extend existing)

**Analog:** `cmd/tldt/main.go` (lines 1–68) — self-analog; extend by adding flags, replacing the direct `summarizer.Summarize()` call, and adding token stats + paragraph grouping.

**Existing flag pattern to follow** (`main.go` lines 16–23):
```go
filePath := flag.String("f", "", "input file path")
flag.Usage = func() {
    fmt.Fprintln(os.Stderr, "Usage: tldt [-f file] [text...]")
    fmt.Fprintln(os.Stderr, "       cat file.txt | tldt")
    flag.PrintDefaults()
    os.Exit(1)
}
flag.Parse()
```

**New flags to add after `filePath` declaration** (D-03 locked):
```go
algorithm  := flag.String("algorithm", "lexrank", "algorithm: lexrank|textrank|graph")
sentences  := flag.Int("sentences", 5, "number of output sentences")
paragraphs := flag.Int("paragraphs", 0, "group sentences into N paragraphs (0 = off)")
```

**Error exit pattern** (`main.go` lines 26–34) — unchanged, copy for new error sites:
```go
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```

**Replace direct summarizer call** (current `main.go` lines 31–37):
```go
// BEFORE (Phase 1):
sentences, err := summarizer.Summarize(text, defaultSentences)

// AFTER (Phase 2):
s, err := summarizer.New(*algorithm)
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
result, err := s.Summarize(text, *sentences)
if err != nil {
    fmt.Fprintln(os.Stderr, "summarization failed:", err)
    os.Exit(1)
}
```

**Token stats pattern** (D-09, D-10 — always to stderr):
```go
charsIn  := len(text)
charsOut := len(strings.Join(result, " "))
tokIn    := charsIn / 4
tokOut   := charsOut / 4
reduction := 0
if tokIn > 0 {
    reduction = int(float64(tokIn-tokOut) / float64(tokIn) * 100)
}
fmt.Fprintf(os.Stderr, "~%s → ~%s tokens (%d%% reduction)\n",
    formatTokens(tokIn), formatTokens(tokOut), reduction)
```

**formatTokens helper** (stdlib only, ~15 lines):
```go
func formatTokens(n int) string {
    s := strconv.Itoa(n)
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
```
Add `"strconv"` to the import block.

**Output change** (D-08 — breaking change, one sentence per line):
```go
// BEFORE (Phase 1):
fmt.Println(strings.Join(sentences, " "))

// AFTER (Phase 2):
if *paragraphs > 0 {
    fmt.Println(groupIntoParagraphs(result, *paragraphs))
} else {
    fmt.Println(strings.Join(result, "\n"))
}
```

**Paragraph grouping helper** (D-05, D-06, D-07):
```go
// groupIntoParagraphs distributes sentences into n evenly-sized paragraphs.
// If n > len(sentences), each sentence gets its own paragraph (D-06: silent cap).
// Paragraphs are separated by blank lines.
func groupIntoParagraphs(sentences []string, n int) string {
    if n > len(sentences) {
        n = len(sentences)
    }
    size := len(sentences) / n
    rem  := len(sentences) % n
    var b strings.Builder
    start := 0
    for i := 0; i < n; i++ {
        end := start + size
        if i < rem {
            end++
        }
        if i > 0 {
            b.WriteString("\n\n")
        }
        b.WriteString(strings.Join(sentences[start:end], "\n"))
        start = end
    }
    return b.String()
}
```

---

## Shared Patterns

### Error Exit
**Source:** `cmd/tldt/main.go` lines 26–29 and 32–35
**Apply to:** All error paths in `main.go` additions
```go
fmt.Fprintln(os.Stderr, err)
os.Exit(1)
```
For errors with context prefix:
```go
fmt.Fprintln(os.Stderr, "summarization failed:", err)
os.Exit(1)
```

### Stateless Struct — Zero Field, New Per Call
**Source:** `internal/summarizer/graph.go` lines 11–14
**Apply to:** `LexRank` struct, `TextRank` struct — no instance variables, all state is local to `Summarize()` call body.
```go
func Summarize(text string, n int) ([]string, error) {
    bag := tldr.New() // created per call, not stored
    return bag.Summarize(text, n)
}
```

### SUM-04 Silent Cap
**Source:** `github.com/didasy/tldr@v0.7.0/tldr.go` lines 147–150 (guard logic)
**Apply to:** `LexRank.Summarize`, `TextRank.Summarize`
```go
if n > len(sentences) {
    n = len(sentences) // silent cap, no error
}
```

### SUM-05 Document Order Restoration
**Source:** `github.com/didasy/tldr@v0.7.0/tldr.go` lines 153–157
**Apply to:** `LexRank.Summarize`, `TextRank.Summarize` — after selecting top-N indices, always `sort.Ints(indices)` before building result slice.
```go
sort.Ints(idx)
return bag.concatResult(idx), nil
```

### Deterministic Sort for Final Ranking
**Source:** RESEARCH.md Determinism Strategy (verified: pkg.go.dev/sort)
**Apply to:** `LexRank.Summarize`, `TextRank.Summarize` — final score sort must be stable.
```go
sort.SliceStable(ranked, func(a, b int) bool {
    return ranked[a].score > ranked[b].score
})
```

### White-Box Test Package Declaration
**Source:** `internal/summarizer/graph_test.go` line 1
**Apply to:** All test files in `internal/summarizer/` — use `package summarizer` (not `package summarizer_test`) to access unexported functions like `idfCosine`, `powerIterate`, `wordOverlapSim`, `buildVocabAndIDF`.
```go
package summarizer
```

### Test Fixture Helper
**Source:** `internal/summarizer/integration_test.go` lines 12–33
**Apply to:** Any new integration test that reads from `test-data/` — reuse the existing `repoRoot(t)` and `readTestFile(t, name)` helpers; do not redefine them.

---

## No Analog Found

All files have analogs. No entries in this section.

---

## Metadata

**Analog search scope:** `internal/summarizer/`, `cmd/tldt/`, `github.com/didasy/tldr@v0.7.0/`
**Files scanned:** 7 (graph.go, summarizer.go, graph_test.go, integration_test.go, main.go, tldr.go, go.mod)
**Pattern extraction date:** 2026-05-01
