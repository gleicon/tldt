package summarizer

// LexRank implements Summarizer using IDF-modified cosine similarity
// and power iteration (Erkan & Dragomir 2004).
type LexRank struct{}

func (l *LexRank) Summarize(text string, n int) ([]string, error) {
	panic("lexrank: not yet implemented — see Plan 02")
}
