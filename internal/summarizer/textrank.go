package summarizer

// TextRank implements Summarizer using word-overlap similarity
// and PageRank-style power iteration (Mihalcea & Tarau 2004).
type TextRank struct{}

func (t *TextRank) Summarize(text string, n int) ([]string, error) {
	panic("textrank: not yet implemented — see Plan 03")
}
