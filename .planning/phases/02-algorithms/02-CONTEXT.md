# Phase 2: Algorithms - Context

**Gathered:** 2026-05-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement LexRank and TextRank natively in Go, expose them (plus the existing graph baseline) via `--algorithm` / `--sentences` / `--paragraphs` flags, output token compression stats to stderr, and ship a deterministic, fully-tested summarization pipeline.

No HTTP, no persistence, no LLM calls, no external NLP dependencies.

</domain>

<decisions>
## Implementation Decisions

### Algorithm Architecture
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

### Sentence Tokenization
- **D-04:** Regexp heuristic in `internal/summarizer/tokenizer.go` — shared by both LexRank and TextRank.
  ```go
  var sentenceEnd = regexp.MustCompile(`[.!?]["''"]?\s+[A-Z"'"]`)
  ```
  Split on sentence-ending punctuation followed by whitespace + capital letter. No external NLP library.

### Paragraph Grouping (`--paragraphs N`)
- **D-05:** `--paragraphs N` distributes output sentences into N paragraphs (evenly). Paragraphs separated by blank lines.
  - Example: `--sentences 6 --paragraphs 2` → 2 paragraphs of 3 sentences each.
- **D-06:** If `N > sentences`, cap paragraphs to sentence count silently (one sentence per paragraph). No error — consistent with SUM-04 behavior.
- **D-07:** Default `--paragraphs 0` means no grouping — flat output.

### Output Formatting
- **D-08:** Default plain-text output: **one sentence per line** (`\n` separator). Breaking change from Phase 1 space-join. Easier to pipe, count, post-process.
- **D-09:** Token compression stats go to **stderr, always** in Phase 2:
  ```
  ~12,400 → ~1,380 tokens (89% reduction)
  ```
  TTY detection (suppress stats when piped) is Phase 3 scope (CLI-05, CLI-06). Phase 2 always emits stats to stderr.
- **D-10:** Token estimate uses `len(text) / 4` heuristic, labeled as estimated (TOK-02).

### Claude's Discretion
- LexRank implementation details: TF-IDF weighting, cosine similarity matrix construction, power iteration convergence tolerance, and stable sort for determinism — researcher and planner decide the specifics.
- TextRank implementation details: word co-occurrence window size, PageRank damping factor, convergence tolerance — researcher and planner decide.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — SUM-01 through SUM-07, TOK-01 through TOK-03, TEST-01 through TEST-06 define what Phase 2 must deliver.
- `.planning/ROADMAP.md` — Phase 2 success criteria (all 4 items must pass).

### Phase 1 Foundation
- `.planning/phases/01-foundation/01-RESEARCH.md` — Algorithm library evaluation: why `didasy/tldr` was chosen, `JesusIslam/tldr` incompatibility, Cobra tradeoffs.
- `.planning/phases/01-foundation/01-PATTERNS.md` — Established patterns: flag parsing, stdin resolution, error handling, `internal/summarizer/` wrapper structure.

### Existing Code
- `internal/summarizer/graph.go` — Existing graph wrapper (implements `Summarize(text string, n int) ([]string, error)` — interface must match this signature).
- `cmd/tldt/main.go` — CLI entry point to extend with new flags.

No external specs beyond requirements above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/summarizer/graph.go`: `Graph` struct wraps `didasy/tldr`. New `LexRank` and `TextRank` structs follow same pattern — thin wrapper, no state, one exported method.
- `cmd/tldt/main.go` `resolveInput()`: No changes needed — already returns `string` that feeds into summarizer.
- `internal/summarizer/graph_test.go` + `integration_test.go`: Test patterns to follow for new algorithm unit tests.

### Established Patterns
- Error handling: `fmt.Fprintln(os.Stderr, err); os.Exit(1)` — all error exits.
- Input resolution: stdin → -f file → positional arg (precedence fixed, unchanged).
- Output: currently `fmt.Println(strings.Join(sentences, " "))` — Phase 2 changes to `strings.Join(sentences, "\n")`.
- No shared state: `Graph` creates `tldr.New()` per call. `LexRank` and `TextRank` must also be stateless per-call.

### Integration Points
- `cmd/tldt/main.go` calls `summarizer.Summarize(text, n)` today. Phase 2 changes this to `summarizer.New(algo)` → `s.Summarize(text, n)`.
- Token stats computed after `s.Summarize()` returns: `charsIn := len(text)`, `charsOut := len(strings.Join(sentences, " "))`, stats to stderr.
- Paragraph grouping is a post-processing step after `s.Summarize()` returns sentences — not inside the algorithms.

</code_context>

<specifics>
## Specific Ideas

- User asked about SymSpell for spell-aware tokenization — noted as deferred.
- User asked about hybrid/ensemble mode (mix LexRank + TextRank scores) — noted as deferred. Researcher should be aware this is on the user's mind for future phases.
- Output: user confirmed one-sentence-per-line is the right default even though it's a breaking change from Phase 1's space-join.

</specifics>

<deferred>
## Deferred Ideas

- **Hybrid/ensemble algorithm mode:** Combine LexRank + TextRank scores to improve accuracy. Own phase after both baseline implementations are solid.
- **SymSpell integration:** Spell correction + word normalization pre-processing pass before tokenization. Would improve TF-IDF quality on noisy transcripts. Own phase.
- **TTY detection for stats suppression:** `--paragraphs` stats suppression when stdout is piped — Phase 3 scope (CLI-05, CLI-06).

</deferred>

---

*Phase: 2-Algorithms*
*Context gathered: 2026-05-01*
