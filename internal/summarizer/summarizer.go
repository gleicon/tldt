package summarizer

import "fmt"

// Summarizer is the common interface for all extractive summarization algorithms.
// Summarize returns up to n sentences from text in original document order.
// If n > sentence count, all sentences are returned (SUM-04).
type Summarizer interface {
	Summarize(text string, n int) ([]string, error)
}

// New returns a Summarizer for the named algorithm.
// Valid names: "lexrank", "textrank", "graph", "ensemble".
func New(algo string) (Summarizer, error) {
	switch algo {
	case "lexrank":
		return &LexRank{}, nil
	case "textrank":
		return &TextRank{}, nil
	case "graph":
		return &Graph{}, nil
	case "ensemble":
		return &Ensemble{}, nil
	default:
		return nil, fmt.Errorf("unknown algorithm: %s", algo)
	}
}
