---
phase: 02-algorithms
fixed_at: 2026-05-01T00:00:00Z
review_path: .planning/phases/02-algorithms/02-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 02: Code Review Fix Report

**Fixed at:** 2026-05-01T00:00:00Z
**Source review:** .planning/phases/02-algorithms/02-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6
- Fixed: 6
- Skipped: 0

## Fixed Issues

### CR-01: LexRank power iteration applies matrix in wrong orientation

**Files modified:** `internal/summarizer/lexrank.go`
**Commit:** 6071dda
**Applied fix:** Rewrote the inner loops of `powerIterate` from `next[i] += matrix[j][i]*p[j]`
(M^T * p, wrong orientation) to `next[j] += matrix[i][j]*p[i]` (p * M, correct row-stochastic
recurrence). The outer loop now iterates over `i` in `p` and the inner loop over `j` in `next`.

---

### CR-02: TextRank `wordOverlapSim` double-counts repeated words in s2

**Files modified:** `internal/summarizer/textrank.go`
**Commit:** 4b0cbb1
**Applied fix:** Replaced single-set lookup (counting every occurrence of s2 words found in
s1's set) with two independent sets `set1` and `set2`. The count loop iterates `set1` and
checks membership in `set2`, counting each distinct shared word exactly once, matching the
TextRank paper definition.

---

### WR-01: `groupIntoParagraphs` has no guard against n=0

**Files modified:** `cmd/tldt/main.go`
**Commit:** aef3b80
**Applied fix:** Added early return at the top of `groupIntoParagraphs`: if `n <= 0` or
`len(sentences) == 0`, returns `strings.Join(sentences, "\n")` immediately before the
integer division that would panic.

---

### WR-02: `normalizeWord` preserves trailing hyphens

**Files modified:** `internal/summarizer/tokenizer.go`
**Commit:** 28c4b5f
**Applied fix:** Extracted the `strings.Map` result into a local variable `mapped` and
added `return strings.TrimRight(mapped, "-")` so any trailing hyphen introduced by the
preceding-character rule is stripped before the token is returned.

---

### WR-03: Integration test ignores errors and will panic on nil receiver

**Files modified:** `internal/summarizer/integration_test.go`
**Commit:** 3a30d26
**Applied fix:** Replaced the `s, _ := New("lexrank")` and `r1, _ := s.Summarize(...)` calls
with explicit error checks using `t.Fatalf` for `New`, the first `Summarize`, and the second
`Summarize`. The `err` variable is now captured and checked for each call.

---

### WR-04: `trSelectTopN` sorts a sub-slice of `indices` in-place

**Files modified:** `internal/summarizer/textrank.go`
**Commit:** 012c0dc
**Applied fix:** Replaced `top := indices[:n]` with `top := make([]int, n)` followed by
`copy(top, indices[:n])`. `sort.Ints(top)` now operates on an independent copy, leaving the
score-sorted `indices` slice intact.

---

_Fixed: 2026-05-01T00:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
