package summarizer

// Ensemble combines LexRank and TextRank by averaging per-sentence scores,
// then returning top-N sentences in document order.
type Ensemble struct{}

// Summarize runs LexRank and TextRank independently, averages their per-sentence
// scores, and returns the top n sentences in document order.
// Returns nil, nil for empty input. Caps n to sentence count (SUM-04).
func (e *Ensemble) Summarize(text string, n int) ([]string, error) {
	sentences := TokenizeSentences(text)
	if len(sentences) == 0 {
		return nil, nil
	}
	if n > len(sentences) {
		n = len(sentences)
	}
	lr := lexrankScores(sentences)
	tr := textrankScores(sentences)
	combined := make([]float64, len(sentences))
	for i := range combined {
		combined[i] = (lr[i] + tr[i]) / 2.0
	}
	return selectTopN(combined, n, sentences), nil
}

// lexrankScores returns per-sentence eigenvector centrality scores (LexRank).
func lexrankScores(sentences []string) []float64 {
	wordLists := make([][]string, len(sentences))
	for i, s := range sentences {
		wordLists[i] = tokenizeWords(s)
	}
	vocab, idf := buildVocabAndIDF(wordLists)
	wordIdx := make(map[string]int, len(vocab))
	for i, w := range vocab {
		wordIdx[w] = i
	}
	vocabSize := len(vocab)
	vectors := make([][]float64, len(sentences))
	for i, words := range wordLists {
		vectors[i] = buildTFVector(words, wordIdx, vocabSize)
	}
	n2 := len(sentences)
	matrix := make([][]float64, n2)
	for i := range matrix {
		matrix[i] = make([]float64, n2)
		for j := range matrix[i] {
			matrix[i][j] = idfCosine(vectors[i], vectors[j], idf)
		}
	}
	rowNormalize(matrix)
	scores, _, _ := powerIterate(matrix, lexrankEpsilon, lexrankMaxIter)
	return scores
}

// textrankScores returns per-sentence PageRank scores (TextRank).
func textrankScores(sentences []string) []float64 {
	words := make([][]string, len(sentences))
	for i, s := range sentences {
		words[i] = tokenizeWords(s)
	}
	size := len(sentences)
	matrix := make([][]float64, size)
	for i := range matrix {
		matrix[i] = make([]float64, size)
		for j := range matrix[i] {
			if i != j {
				matrix[i][j] = wordOverlapSim(words[i], words[j])
			}
		}
	}
	trRowNormalize(matrix)
	scores, _, _ := powerIterateDamped(matrix, textRankDamping, textRankEpsilon, textRankMaxIter)
	return scores
}
