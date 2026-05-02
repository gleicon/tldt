package summarizer

import (
	"math"
	"sort"
)

const lexrankEpsilon = 0.0001
const lexrankMaxIter = 1000

// LexRank implements Summarizer using IDF-modified cosine similarity
// and power iteration (Erkan & Dragomir 2004).
type LexRank struct{}

// Summarize returns the top n sentences from text ranked by eigenvector centrality
// using IDF-modified cosine similarity. Sentences are returned in document order.
// Returns nil, nil for empty input. Caps n to sentence count silently (SUM-04).
func (l *LexRank) Summarize(text string, n int) ([]string, error) {
	sentences := TokenizeSentences(text)
	if len(sentences) == 0 {
		return nil, nil
	}
	if n > len(sentences) {
		n = len(sentences)
	}

	// Tokenize each sentence into normalized words
	wordLists := make([][]string, len(sentences))
	for i, s := range sentences {
		wordLists[i] = tokenizeWords(s)
	}

	// Build sorted vocabulary and IDF weights
	vocab, idf := buildVocabAndIDF(wordLists)

	// Build word → index map for TF vector construction
	wordIdx := make(map[string]int, len(vocab))
	for i, w := range vocab {
		wordIdx[w] = i
	}

	// Build TF-IDF vectors for each sentence
	vocabSize := len(vocab)
	vectors := make([][]float64, len(sentences))
	for i, words := range wordLists {
		vectors[i] = buildTFVector(words, wordIdx, vocabSize)
	}

	// Build n×n cosine similarity matrix (continuous — no threshold)
	n2 := len(sentences)
	matrix := make([][]float64, n2)
	for i := range matrix {
		matrix[i] = make([]float64, n2)
		for j := range matrix[i] {
			matrix[i][j] = idfCosine(vectors[i], vectors[j], idf)
		}
	}

	// Row-normalize to stochastic matrix
	rowNormalize(matrix)

	// Power iteration to find stationary distribution (eigenvector centrality)
	scores := powerIterate(matrix, lexrankEpsilon, lexrankMaxIter)

	return selectTopN(scores, n, sentences), nil
}

// buildVocabAndIDF computes the sorted vocabulary and parallel IDF weights
// for a list of tokenized sentences. Uses single-document IDF:
//
//	IDF(w) = log(N / df(w))
//
// where N is the number of sentences and df(w) is the number of sentences
// containing word w. Vocabulary is sorted alphabetically for determinism.
//
// Source: Erkan & Dragomir 2004 (LexRank paper)
func buildVocabAndIDF(sentences [][]string) ([]string, []float64) {
	N := len(sentences)
	df := make(map[string]int)
	for _, words := range sentences {
		seen := make(map[string]bool)
		for _, w := range words {
			if !seen[w] {
				df[w]++
				seen[w] = true
			}
		}
	}

	// Extract and sort vocabulary for deterministic indexing
	vocab := make([]string, 0, len(df))
	for w := range df {
		vocab = append(vocab, w)
	}
	sort.Strings(vocab)

	idf := make([]float64, len(vocab))
	for i, w := range vocab {
		idf[i] = math.Log(float64(N) / float64(df[w]))
	}
	return vocab, idf
}

// buildTFVector builds a term-frequency vector for a sentence.
// Each dimension corresponds to a vocabulary word (indexed by wordIdx).
// TF(w) = count(w in sentence) / total words in sentence.
func buildTFVector(words []string, wordIdx map[string]int, vocabSize int) []float64 {
	if len(words) == 0 {
		return make([]float64, vocabSize)
	}
	counts := make([]int, vocabSize)
	for _, w := range words {
		if idx, ok := wordIdx[w]; ok {
			counts[idx]++
		}
	}
	total := float64(len(words))
	v := make([]float64, vocabSize)
	for i, c := range counts {
		v[i] = float64(c) / total
	}
	return v
}

// idfCosine computes the IDF-modified cosine similarity between two TF vectors.
// Formula: sum(idf[i]^2 * v1[i] * v2[i]) / (sqrt(sum(idf[i]^2*v1[i]^2)) * sqrt(sum(idf[i]^2*v2[i]^2)))
// Returns 0.0 if either vector has zero IDF-weighted norm (avoids NaN).
//
// Source: Erkan & Dragomir 2004 (LexRank paper)
func idfCosine(v1, v2, idf []float64) float64 {
	dot, n1, n2 := 0.0, 0.0, 0.0
	for i := range v1 {
		w := idf[i] * idf[i]
		dot += w * v1[i] * v2[i]
		n1 += w * v1[i] * v1[i]
		n2 += w * v2[i] * v2[i]
	}
	if n1 == 0 || n2 == 0 {
		return 0
	}
	return dot / (math.Sqrt(n1) * math.Sqrt(n2))
}

// rowNormalize normalizes each row of matrix to sum to 1.0.
// Rows that sum to 0 are replaced with uniform probability (1/n).
func rowNormalize(matrix [][]float64) {
	n := len(matrix)
	for i := range matrix {
		sum := 0.0
		for _, v := range matrix[i] {
			sum += v
		}
		if sum > 0 {
			for j := range matrix[i] {
				matrix[i][j] /= sum
			}
		} else {
			// Dangling row: assign uniform probability
			uniform := 1.0 / float64(n)
			for j := range matrix[i] {
				matrix[i][j] = uniform
			}
		}
	}
}

// powerIterate returns the stationary distribution of a row-stochastic matrix
// using the power method. Converges when L1 difference < epsilon or maxIter reached.
//
// Source: standard power method; matches didasy/tldr DEFAULT_TOLERANCE=0.0001
func powerIterate(matrix [][]float64, epsilon float64, maxIter int) []float64 {
	n := len(matrix)
	p := make([]float64, n)
	for i := range p {
		p[i] = 1.0 / float64(n)
	}
	for iter := 0; iter < maxIter; iter++ {
		next := make([]float64, n)
		for i := range p {
			for j := range next {
				next[j] += matrix[i][j] * p[i]
			}
		}
		diff := 0.0
		for i := range p {
			diff += math.Abs(next[i] - p[i])
		}
		p = next
		if diff < epsilon {
			break
		}
	}
	return p
}

// scored is a pair of sentence index and its centrality score.
type scored struct {
	idx   int
	score float64
}

// selectTopN selects the top n sentences by score and returns them in document order (SUM-05).
// Uses sort.SliceStable for deterministic tie-breaking.
func selectTopN(scores []float64, n int, sentences []string) []string {
	ranked := make([]scored, len(scores))
	for i, s := range scores {
		ranked[i] = scored{i, s}
	}
	// Stable sort descending by score — deterministic tie-breaking
	sort.SliceStable(ranked, func(a, b int) bool {
		return ranked[a].score > ranked[b].score
	})
	if n > len(ranked) {
		n = len(ranked)
	}
	top := make([]int, n)
	for i := 0; i < n; i++ {
		top[i] = ranked[i].idx
	}
	// Restore document order (SUM-05)
	sort.Ints(top)
	result := make([]string, n)
	for i, idx := range top {
		result[i] = sentences[idx]
	}
	return result
}
