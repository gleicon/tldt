package summarizer

import "github.com/didasy/tldr"

// Graph implements Summarizer using the PageRank-based graph algorithm
// from github.com/didasy/tldr (didasy/tldr v0.7.0).
//
// Note: a new *tldr.Bag is created per call. The Bag type is not thread-safe;
// do not share instances across goroutines.
type Graph struct{}

func (g *Graph) Summarize(text string, n int) ([]string, error) {
	bag := tldr.New()
	return bag.Summarize(text, n)
}
