package summarizer

import (
	"math"
	"sort"
)

// TextRank implements Summarizer using word-overlap similarity
// and PageRank-style power iteration (Mihalcea & Tarau 2004).
type TextRank struct{}

const textRankDamping = 0.85
const textRankEpsilon = 0.0001
const textRankMaxIter = 1000

// wordOverlapSim computes the word-overlap similarity between two tokenized sentences.
// Returns 0.0 if either slice has len <= 1 (log(1)=0 causes division by zero).
// Formula: |common| / (log(|s1|) + log(|s2|))
func wordOverlapSim(s1, s2 []string) float64 {
	if len(s1) <= 1 || len(s2) <= 1 {
		return 0.0
	}
	set1 := make(map[string]bool, len(s1))
	for _, w := range s1 {
		set1[w] = true
	}
	set2 := make(map[string]bool, len(s2))
	for _, w := range s2 {
		set2[w] = true
	}
	common := 0
	for w := range set1 {
		if set2[w] {
			common++
		}
	}
	if common == 0 {
		return 0.0
	}
	return float64(common) / (math.Log(float64(len(s1))) + math.Log(float64(len(s2))))
}

// trRowNormalize normalizes each row of matrix to sum to 1.0.
// Zero rows are replaced with a uniform distribution (1/n).
func trRowNormalize(matrix [][]float64) {
	n := len(matrix)
	for i := range matrix {
		sum := 0.0
		for _, v := range matrix[i] {
			sum += v
		}
		if sum == 0 {
			for j := range matrix[i] {
				matrix[i][j] = 1.0 / float64(n)
			}
		} else {
			for j := range matrix[i] {
				matrix[i][j] /= sum
			}
		}
	}
}

// powerIterateDamped performs damped PageRank-style power iteration.
// Returns (scores, iterations, converged) after convergence or maxIter iterations.
func powerIterateDamped(matrix [][]float64, damping, epsilon float64, maxIter int) ([]float64, int, bool) {
	n := len(matrix)
	scores := make([]float64, n)
	for i := range scores {
		scores[i] = 1.0 / float64(n)
	}
	base := (1.0 - damping) / float64(n)
	next := make([]float64, n)
	for iter := range maxIter {
		for i := range next {
			sum := 0.0
			for j := range matrix {
				sum += matrix[j][i] * scores[j]
			}
			next[i] = base + damping*sum
		}
		// Check L1 convergence
		diff := 0.0
		for i := range scores {
			d := next[i] - scores[i]
			if d < 0 {
				d = -d
			}
			diff += d
		}
		copy(scores, next)
		if diff < epsilon {
			return scores, iter + 1, true
		}
	}
	return scores, maxIter, false
}

// trSelectTopN selects the top-n sentences by score and returns them in document order.
func trSelectTopN(scores []float64, n int, sentences []string) []string {
	// Clamp n to [0, len] so the function never panics on out-of-range counts.
	if n < 0 {
		n = 0
	}
	if n > len(sentences) {
		n = len(sentences)
	}
	indices := make([]int, len(sentences))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(a, b int) bool {
		return scores[indices[a]] > scores[indices[b]]
	})
	top := make([]int, n)
	copy(top, indices[:n])
	sort.Ints(top)
	result := make([]string, n)
	for i, idx := range top {
		result[i] = sentences[idx]
	}
	return result
}

// SummarizeExplain implements Explainer. Same algorithm as Summarize but
// also collects and returns diagnostic information for --explain output.
func (t *TextRank) SummarizeExplain(text string, n int) ([]string, *ExplainInfo, error) {
	sentences := TokenizeSentences(text)
	info := &ExplainInfo{
		Algorithm:      "textrank",
		InputSentences: len(sentences),
		DampingFactor:  textRankDamping,
	}
	if len(sentences) == 0 {
		return nil, info, nil
	}
	if n > len(sentences) {
		n = len(sentences)
	}
	info.SelectedN = n

	c := textrankCompute(sentences)
	info.SimilarityPairs = c.simPairs
	info.SimilarityNonZero = c.simNonZero
	info.SimilarityMax = c.simMax
	info.SimilarityMean = c.simMean
	info.Iterations = c.iters
	info.Converged = c.converged
	info.Scores = buildSentenceScores(sentences, c.scores, n)

	return trSelectTopN(c.scores, n, sentences), info, nil
}

// Summarize implements the Summarizer interface using TextRank.
// Returns up to n sentences from text in original document order.
// Returns nil, nil for empty input. Caps n to sentence count.
func (t *TextRank) Summarize(text string, n int) ([]string, error) {
	sentences := TokenizeSentences(text)
	if len(sentences) == 0 {
		return nil, nil
	}
	if n > len(sentences) {
		n = len(sentences)
	}
	c := textrankCompute(sentences)
	return trSelectTopN(c.scores, n, sentences), nil
}

// textrankComputed carries the result of one TextRank scoring pass plus the
// word-overlap similarity statistics --explain reports (accumulated while the
// matrix is built, so non-explain callers pay no extra pass).
type textrankComputed struct {
	scores     []float64
	iters      int
	converged  bool
	simPairs   int
	simNonZero int
	simMax     float64
	simMean    float64
}

// textrankCompute runs the TextRank pipeline (word-overlap similarity → row
// normalization → damped power iteration) over sentences, which must be
// non-empty.
func textrankCompute(sentences []string) textrankComputed {
	words := make([][]string, len(sentences))
	for i, s := range sentences {
		words[i] = tokenizeWords(s)
	}

	size := len(sentences)
	c := textrankComputed{simPairs: size * (size - 1)}
	matrix := make([][]float64, size)
	for i := range matrix {
		matrix[i] = make([]float64, size)
		for j := range matrix[i] {
			if i == j {
				continue
			}
			v := wordOverlapSim(words[i], words[j])
			matrix[i][j] = v
			if v > 0 {
				c.simNonZero++
				if v > c.simMax {
					c.simMax = v
				}
				c.simMean += v
			}
		}
	}
	if c.simNonZero > 0 {
		c.simMean /= float64(c.simNonZero)
	}

	trRowNormalize(matrix)
	c.scores, c.iters, c.converged = powerIterateDamped(matrix, textRankDamping, textRankEpsilon, textRankMaxIter)
	return c
}
