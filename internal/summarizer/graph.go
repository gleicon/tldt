package summarizer

import "github.com/didasy/tldr"

// Summarize returns up to n sentences from text using the graph/PageRank algorithm.
// Sentences are returned in original document order, not score order.
// This is a thin wrapper around github.com/didasy/tldr v0.7.0.
//
// Note: a new *tldr.Bag is created per call. The Bag type is not thread-safe;
// do not share instances across goroutines (relevant for Phase 2 parallel processing).
func Summarize(text string, n int) ([]string, error) {
	bag := tldr.New()
	return bag.Summarize(text, n)
}
