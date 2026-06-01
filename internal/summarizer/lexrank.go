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

// MatrixSummarizer is an optional interface implemented by LexRank.
// SummarizeWithMatrix returns the top n sentences and the raw (pre-normalization)
// IDF-cosine similarity matrix, which callers can use for outlier detection.
// The matrix has dimensions len(sentences)×len(sentences).
type MatrixSummarizer interface {
	SummarizeWithMatrix(text string, n int) ([]string, [][]float64, error)
}

// Summarize returns the top n sentences from text ranked by eigenvector centrality
// using IDF-modified cosine similarity. Sentences are returned in document order.
// Returns nil, nil for empty input. Caps n to sentence count silently.
func (l *LexRank) Summarize(text string, n int) ([]string, error) {
	result, _, err := l.SummarizeWithMatrix(text, n)
	return result, err
}

// SummarizeWithMatrix runs the full LexRank algorithm and additionally returns
// the raw IDF-cosine similarity matrix (before row normalization). The matrix
// can be passed to detector.DetectOutliers for statistical injection detection.
// Matrix values are in [0, 1] where 1.0 = identical sentence content.
func (l *LexRank) SummarizeWithMatrix(text string, n int) ([]string, [][]float64, error) {
	sentences := TokenizeSentences(text)
	if len(sentences) == 0 {
		return nil, nil, nil
	}
	if n > len(sentences) {
		n = len(sentences)
	}
	c := lexrankCompute(sentences, true)
	return selectTopN(c.scores, n, sentences), c.rawMatrix, nil
}

// lexrankComputed carries the result of one LexRank scoring pass plus the
// intermediates different callers need (the raw similarity matrix for outlier
// detection; vocab/IDF and convergence stats for --explain).
type lexrankComputed struct {
	scores    []float64
	rawMatrix [][]float64 // nil unless keepRawMatrix was set
	vocabSize int
	idfMin    float64
	idfMax    float64
	iters     int
	converged bool
}

// lexrankCompute runs the LexRank pipeline (IDF-cosine similarity → row
// normalization → power iteration) over sentences. keepRawMatrix snapshots the
// pre-normalization similarity matrix; callers that don't need it pay no extra
// allocation. sentences must be non-empty.
func lexrankCompute(sentences []string, keepRawMatrix bool) lexrankComputed {
	wordLists := make([][]string, len(sentences))
	for i, s := range sentences {
		wordLists[i] = tokenizeWords(s)
	}

	vocab, idf := buildVocabAndIDF(wordLists)
	c := lexrankComputed{vocabSize: len(vocab)}
	if len(idf) > 0 {
		c.idfMin, c.idfMax = idf[0], idf[0]
		for _, v := range idf {
			if v < c.idfMin {
				c.idfMin = v
			}
			if v > c.idfMax {
				c.idfMax = v
			}
		}
	}

	wordIdx := make(map[string]int, len(vocab))
	for i, w := range vocab {
		wordIdx[w] = i
	}
	vocabSize := len(vocab)
	vectors := make([][]float64, len(sentences))
	for i, words := range wordLists {
		vectors[i] = buildTFVector(words, wordIdx, vocabSize)
	}

	// Build n×n cosine similarity matrix (continuous — no threshold), optionally
	// snapshotting raw values before row normalization for outlier detection.
	n2 := len(sentences)
	matrix := make([][]float64, n2)
	if keepRawMatrix {
		c.rawMatrix = make([][]float64, n2)
	}
	for i := range matrix {
		matrix[i] = make([]float64, n2)
		if keepRawMatrix {
			c.rawMatrix[i] = make([]float64, n2)
		}
		for j := range matrix[i] {
			v := idfCosine(vectors[i], vectors[j], idf)
			matrix[i][j] = v
			if keepRawMatrix {
				c.rawMatrix[i][j] = v
			}
		}
	}

	rowNormalize(matrix)
	c.scores, c.iters, c.converged = powerIterate(matrix, lexrankEpsilon, lexrankMaxIter)
	return c
}

// SummarizeExplain implements Explainer. Same algorithm as Summarize but
// also collects and returns diagnostic information for --explain output.
func (l *LexRank) SummarizeExplain(text string, n int) ([]string, *ExplainInfo, error) {
	sentences := TokenizeSentences(text)
	info := &ExplainInfo{Algorithm: "lexrank", InputSentences: len(sentences)}
	if len(sentences) == 0 {
		return nil, info, nil
	}
	if n > len(sentences) {
		n = len(sentences)
	}
	info.SelectedN = n

	c := lexrankCompute(sentences, false)
	info.VocabSize = c.vocabSize
	info.IDFMin, info.IDFMax = c.idfMin, c.idfMax
	info.Iterations = c.iters
	info.Converged = c.converged
	info.Scores = buildSentenceScores(sentences, c.scores, n)

	return selectTopN(c.scores, n, sentences), info, nil
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
// Returns (scores, iterations, converged).
//
// Source: standard power method; matches didasy/tldr DEFAULT_TOLERANCE=0.0001
func powerIterate(matrix [][]float64, epsilon float64, maxIter int) ([]float64, int, bool) {
	n := len(matrix)
	p := make([]float64, n)
	for i := range p {
		p[i] = 1.0 / float64(n)
	}
	for iter := range maxIter {
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
			return p, iter + 1, true
		}
	}
	return p, maxIter, false
}

// scored is a pair of sentence index and its centrality score.
type scored struct {
	idx   int
	score float64
}

// selectTopN selects the top n sentences by score and returns them in document order.
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
	if n < 0 {
		n = 0
	}
	top := make([]int, n)
	for i := 0; i < n; i++ {
		top[i] = ranked[i].idx
	}
	// Restore document order
	sort.Ints(top)
	result := make([]string, n)
	for i, idx := range top {
		result[i] = sentences[idx]
	}
	return result
}
