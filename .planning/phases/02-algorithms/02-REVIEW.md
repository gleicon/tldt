---
phase: 02-algorithms
reviewed: 2026-05-01T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - cmd/tldt/main.go
  - internal/summarizer/graph.go
  - internal/summarizer/lexrank.go
  - internal/summarizer/lexrank_test.go
  - internal/summarizer/summarizer.go
  - internal/summarizer/textrank.go
  - internal/summarizer/textrank_test.go
  - internal/summarizer/tokenizer.go
  - internal/summarizer/tokenizer_test.go
  - internal/summarizer/integration_test.go
findings:
  critical: 2
  warning: 4
  info: 3
  total: 9
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-05-01T00:00:00Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

This phase implements LexRank and TextRank extractive summarization algorithms alongside the
pre-existing graph/PageRank wrapper. The code is generally well-structured with good test
coverage. Two blocker-level algorithmic correctness bugs were found: a matrix transposition
error in the LexRank power iteration that computes the wrong stationary distribution for
asymmetric transition matrices, and a word-overlap double-counting error in TextRank. Both
produce incorrect ranking output silently. Four warnings cover a missing zero-guard in a
public helper, a trailing-hyphen tokenization defect, a test that panics on error instead of
failing gracefully, and a fragile in-place mutation of a slice header. Three info items cover
minor test quality and code style concerns.

---

## Critical Issues

### CR-01: LexRank power iteration applies matrix in wrong orientation

**File:** `internal/summarizer/lexrank.go:179-183`

**Issue:** The power iteration in `powerIterate` computes:

```go
next[i] += matrix[j][i] * p[j]
```

This is `next = M^T * p` — it treats the matrix as column-stochastic and multiplies on the
left. The matrix `M` was built as a row-stochastic matrix (rows sum to 1 after
`rowNormalize`). The correct recurrence for finding the stationary distribution of a
row-stochastic matrix is `p_next = p * M`, i.e.:

```
next[j] = sum_i(p[i] * M[i][j])
```

which in index form is `next[j] += matrix[i][j] * p[i]`, not `next[i] += matrix[j][i] * p[j]`.

Although the cosine similarity values are symmetric (`sim[i][j] == sim[j][i]`), after
row-normalization the matrix is no longer symmetric in general:
`P[i][j] = sim[i][j] / row_sum[i]` vs `P[j][i] = sim[j][i] / row_sum[j]`.
When rows have different sums (which is the typical case), `P[i][j] != P[j][i]`, so the
transposed computation yields a different — and incorrect — stationary distribution.

The result is that LexRank scores sentences using the wrong eigenvector, silently producing
a different ranking than the algorithm specifies.

**Fix:**
```go
// Correct: next[j] = sum_i( p[i] * M[i][j] )
for iter := 0; iter < maxIter; iter++ {
    next := make([]float64, n)
    for i := range p {
        for j := range next {
            next[j] += matrix[i][j] * p[i]
        }
    }
    // ... convergence check unchanged ...
}
```

---

### CR-02: TextRank `wordOverlapSim` double-counts repeated words in s2

**File:** `internal/summarizer/textrank.go:27-31`

**Issue:** The overlap counter increments for every occurrence of a word in `s2` that
appears in `s1`, not just the first occurrence:

```go
common := 0
for _, w := range s2 {
    if set[w] {
        common++   // fires for every occurrence of w in s2
    }
}
```

If `s2 = ["cat", "cat", "dog"]` and `s1 = ["cat", "dog"]`, `common` = 3 even though only
2 distinct words overlap. The TextRank paper (Mihalcea & Tarau 2004) defines overlap as the
count of *distinct* words common to both sentences. Double-counting inflates similarity
scores for sentences with repeated vocabulary and produces incorrect rankings.

**Fix:**
```go
// Build a set for s2 as well; count distinct common words
set1 := make(map[string]bool, len(s1))
for _, w := range s1 {
    set1[w] = true
}
set2 := make(map[string]bool, len(s2))
for _, w := range s2 {
    set2[w] = true
}
common := 0
for w := range set1 {
    if set2[w] {
        common++
    }
}
```

---

## Warnings

### WR-01: `groupIntoParagraphs` has no guard against n=0 — division by zero on direct call

**File:** `cmd/tldt/main.go:91`

**Issue:** `size := len(sentences) / n` panics if `n == 0`. The call site guards with
`if *paragraphs > 0` (line 58), but `groupIntoParagraphs` is an exported-style helper
(lower-case, but used as a standalone function) without its own guard. Any future caller
that passes `n=0` — including tests added directly against this function — will cause a
runtime panic instead of a recoverable error.

**Fix:**
```go
func groupIntoParagraphs(sentences []string, n int) string {
    if n <= 0 || len(sentences) == 0 {
        return strings.Join(sentences, "\n")
    }
    // ... rest unchanged
}
```

---

### WR-02: `normalizeWord` preserves trailing hyphens

**File:** `internal/summarizer/tokenizer.go:48-58`

**Issue:** The hyphen-preservation rule fires when `prev` is a digit or letter — it checks
the *preceding* character, not the *following* one. A word ending with a hyphen (e.g.,
`"state-"`, `"co-"`) will have the trailing hyphen preserved because `prev` at that point
is the preceding letter. The resulting token `"state-"` is different from `"state"`,
causing two tokens that should be identical to not match during overlap/TF-IDF computation.

Example: `normalizeWord("state-")` returns `"state-"`, not `"state"`.

**Fix:** After the `strings.Map` call, strip any trailing hyphen:
```go
func normalizeWord(word string) string {
    word = strings.ToLower(word)
    var prev rune
    mapped := strings.Map(func(r rune) rune {
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
    return strings.TrimRight(mapped, "-")
}
```

---

### WR-03: Integration test ignores errors and will panic on nil receiver

**File:** `internal/summarizer/integration_test.go:235-236`

**Issue:**
```go
s, _ := New("lexrank")
r1, _ := s.Summarize(text, 3)
```

If `New` returns an error (e.g., due to a future refactor that changes the algorithm name),
`s` is `nil`. Calling `s.Summarize(...)` on a nil interface panics rather than producing a
test failure. The panic surfaces as a test crash, not a clean `FAIL` with a message,
making CI output harder to diagnose.

**Fix:**
```go
s, err := New("lexrank")
if err != nil {
    t.Fatalf("New(lexrank) error: %v", err)
}
r1, err := s.Summarize(text, 3)
if err != nil {
    t.Fatalf("first Summarize error: %v", err)
}
r2, err := s.Summarize(text, 3)
if err != nil {
    t.Fatalf("second Summarize error: %v", err)
}
```

---

### WR-04: `trSelectTopN` sorts a sub-slice of `indices` in-place, mutating caller-visible state

**File:** `internal/summarizer/textrank.go:106-107`

**Issue:**
```go
top := indices[:n]
sort.Ints(top)
```

`top` is a slice header pointing into the same backing array as `indices`. `sort.Ints(top)`
reorders the first `n` elements of `indices` in-place. Although `indices` is local to
`trSelectTopN` and is not used after this point, the pattern is fragile: any addition of
code between lines 106 and 110 that reads from `indices[0:n]` expecting score-sorted order
will get position-sorted data instead. A copy makes the intent explicit:

**Fix:**
```go
top := make([]int, n)
copy(top, indices[:n])
sort.Ints(top)
```

---

## Info

### IN-01: `tenSentenceText` / `threeSentenceText` test fixtures defined in non-reviewed `graph_test.go` but relied on by lexrank and textrank tests

**File:** `internal/summarizer/graph_test.go:9-23`

**Issue:** The shared test constants `tenSentenceText` and `threeSentenceText` live in
`graph_test.go` (within the `summarizer` package test scope), which is not obviously
associated with the LexRank or TextRank tests. If `graph_test.go` is ever removed or
renamed, both `lexrank_test.go` and `textrank_test.go` break at compile time with no
obvious reason. The fixtures should live in a dedicated `testdata_test.go` or
`fixtures_test.go` file.

**Fix:** Move the constants to `internal/summarizer/testfixtures_test.go`.

---

### IN-02: Token estimation uses integer division, losing fractional tokens — result is misleading

**File:** `cmd/tldt/main.go:48-49`

**Issue:**
```go
tokIn := charsIn / 4
tokOut := charsOut / 4
```

Integer division truncates. For a 6-character input `tokIn = 1`, but for a 7-character
input `tokIn = 1` as well. When `tokIn == tokOut == 0` (very short input), the reduction
formula returns 0% even though the input was compressed. The stat is printed to stderr and
labeled approximate (`~`), but the rounding makes it inaccurate for small inputs and could
mislead users into thinking nothing was summarized.

**Fix:** Use `(charsIn + 3) / 4` for ceiling division, or compute as float and round.

---

### IN-03: `sentenceEnd` regex requires next sentence to start with uppercase or quote — fails on numbered lists and lowercase continuations

**File:** `internal/summarizer/tokenizer.go:12`

**Issue:** The regex `[.!?]['"...]?\s+[A-Z'"]` will not split at sentence boundaries where
the following sentence starts with a digit (e.g., `"Done. 3 items remain."`) or a lowercase
letter (legitimate in some document styles). This is documented as a heuristic, but there
is no test or comment indicating the known limitation, which may surprise downstream users.

**Fix:** Add a comment enumerating the known non-splitting cases. Optionally extend the
character class to `[A-Z0-9'"]` to handle digit-starting sentences.

---

_Reviewed: 2026-05-01T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
