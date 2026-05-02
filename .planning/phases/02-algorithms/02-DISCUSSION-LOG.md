# Phase 2: Algorithms - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-01
**Phase:** 2-Algorithms
**Areas discussed:** Algorithm architecture, Sentence tokenization, Paragraph grouping, Output formatting

---

## Algorithm Architecture

### Package structure

| Option | Description | Selected |
|--------|-------------|----------|
| Extend internal/summarizer | lexrank.go + textrank.go + summarizer.go in existing package | ✓ |
| Sub-packages | internal/summarizer/lexrank/, internal/summarizer/textrank/ | |
| Single file | All three algorithms in algorithms.go | |

**User's choice:** Extend internal/summarizer (flat layout)

---

### Interface / dispatch

| Option | Description | Selected |
|--------|-------------|----------|
| Interface + registry | `Summarizer` interface, `New(algo string)` factory | ✓ |
| Plain dispatch function | `Summarize(algo, text, n)` with switch inside | |

**User's choice:** Interface + registry
**Notes:** User asked to investigate whether algorithms should be mixed to improve accuracy. Captured as deferred idea (hybrid/ensemble mode).

---

### CLI flag library

| Option | Description | Selected |
|--------|-------------|----------|
| Keep stdlib flag | Add --algorithm, --sentences, --paragraphs to existing setup | ✓ |
| Switch to Cobra | Add subcommand support and --help formatting | |

**User's choice:** Keep stdlib flag

---

## Sentence Tokenization

### Split strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Regexp heuristic | `[.!?]["''"]?\s+[A-Z]` — no deps, testable | ✓ |
| Newline-aware split | Split on \n first, then punctuation within lines | |
| Third-party library | jdkato/prose or neurosnap/sentences | |

**User's choice:** Regexp heuristic
**Notes:** User asked about SymSpell for spell correction + tokenization. Captured as deferred idea.

---

### Tokenizer location

| Option | Description | Selected |
|--------|-------------|----------|
| internal/summarizer/tokenizer.go | Shared by both algorithms | ✓ |
| Each algorithm owns its tokenizer | Allows divergence, duplicates code | |

**User's choice:** Shared tokenizer.go

---

## Paragraph Grouping

### Grouping semantic

| Option | Description | Selected |
|--------|-------------|----------|
| Produce N paragraphs | Distribute sentences evenly into N groups | ✓ |
| Group every N sentences | Each paragraph has N sentences; total varies | |

**User's choice:** Produce N paragraphs (evenly distributed)

---

### Overflow behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Cap to sentence count | If N > sentences, produce M paragraphs of 1 each. Silent. | ✓ |
| Error out | Return error if N > sentences | |
| One paragraph | Produce single paragraph when N > sentences | |

**User's choice:** Silent cap — consistent with SUM-04

---

## Output Formatting

### Sentence separator

| Option | Description | Selected |
|--------|-------------|----------|
| One sentence per line | \n between sentences. Pipe-friendly. | ✓ |
| Space-joined (Phase 1 behavior) | All sentences in one paragraph | |

**User's choice:** One sentence per line (breaking change from Phase 1)

---

### Token stats placement

| Option | Description | Selected |
|--------|-------------|----------|
| stderr, always | Always emit stats to stderr. TTY suppression is Phase 3. | ✓ |
| stderr, only TTY | Suppress when stdout piped (Phase 3 concern) | |

**User's choice:** stderr always in Phase 2

---

## Claude's Discretion

- LexRank internals: TF-IDF weighting approach, cosine similarity matrix construction, power iteration convergence tolerance, stable sort mechanism for determinism
- TextRank internals: word co-occurrence window size, PageRank damping factor and convergence tolerance
- Token heuristic format string (e.g., `~12,400 → ~1,380 tokens (89% reduction)`)

## Deferred Ideas

- **Hybrid/ensemble algorithm mode:** Mix LexRank + TextRank scores for better accuracy. User specifically asked about this. Own future phase.
- **SymSpell integration:** Spell correction + word normalization before tokenization. Improves TF-IDF quality on noisy transcripts. Own future phase.
- **TTY detection for stats suppression:** Phase 3 (CLI-05, CLI-06).
