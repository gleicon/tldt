package summarizer

import (
	"fmt"
	"strings"
)

// ExplainInfo holds debug diagnostics from a summarization run.
// Printed to stderr when --explain is active.
type ExplainInfo struct {
	Algorithm string

	// Input / output counts
	InputSentences int
	SelectedN      int

	// LexRank only
	VocabSize int
	IDFMin    float64
	IDFMax    float64

	// TextRank only
	DampingFactor     float64
	SimilarityNonZero int
	SimilarityPairs   int
	SimilarityMax     float64
	SimilarityMean    float64

	// Both (0 for Graph — opaque library)
	Iterations int
	Converged  bool

	// Per-sentence scores in document order
	Scores []SentenceScore
}

// SentenceScore holds the centrality score and selection status for one sentence.
type SentenceScore struct {
	Index    int
	Score    float64
	Selected bool
	Rank     int    // 1-based rank by score (1 = highest)
	Preview  string // first 72 chars of sentence
}

// Explainer is an optional interface implemented by algorithms that can
// return per-run diagnostics alongside the summary.
type Explainer interface {
	SummarizeExplain(text string, n int) ([]string, *ExplainInfo, error)
}

// preview truncates s to maxLen characters for display.
func preview(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// PrintExplain writes a human-readable explain report to a string.
// Callers write it to stderr.
func (e *ExplainInfo) Format() string {
	var b strings.Builder

	fmt.Fprintf(&b, "\nexplain: algorithm=%s\n", e.Algorithm)
	fmt.Fprintf(&b, "  input_sentences: %d\n", e.InputSentences)
	fmt.Fprintf(&b, "  selected:        %d\n", e.SelectedN)

	switch e.Algorithm {
	case "lexrank":
		fmt.Fprintf(&b, "  vocab_size:      %d\n", e.VocabSize)
		fmt.Fprintf(&b, "  idf_range:       %.4f .. %.4f\n", e.IDFMin, e.IDFMax)
	case "textrank":
		fmt.Fprintf(&b, "  damping:         %.2f\n", e.DampingFactor)
		fmt.Fprintf(&b, "  nonzero_pairs:   %d / %d\n", e.SimilarityNonZero, e.SimilarityPairs)
		fmt.Fprintf(&b, "  max_similarity:  %.4f\n", e.SimilarityMax)
		fmt.Fprintf(&b, "  mean_similarity: %.4f\n", e.SimilarityMean)
	}

	if e.Iterations > 0 {
		conv := "yes"
		if !e.Converged {
			conv = fmt.Sprintf("no (hit max %d)", e.Iterations)
		}
		fmt.Fprintf(&b, "  iterations:      %d  converged: %s\n", e.Iterations, conv)
	}

	fmt.Fprintln(&b, "\nsentence scores (document order):")
	fmt.Fprintln(&b, "  idx  rank  score     sel  text")
	for _, s := range e.Scores {
		sel := "  "
		if s.Selected {
			sel = "* "
		}
		fmt.Fprintf(&b, "  %3d  %4d  %.6f  %s%s\n",
			s.Index, s.Rank, s.Score, sel, preview(s.Preview, 55))
	}
	return b.String()
}
