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
// Uses a distinct name to avoid conflicts if LexRank runs in parallel.
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
// Returns sentence importance scores after convergence or maxIter iterations.
func powerIterateDamped(matrix [][]float64, damping, epsilon float64, maxIter int) []float64 {
	n := len(matrix)
	scores := make([]float64, n)
	for i := range scores {
		scores[i] = 1.0 / float64(n)
	}
	base := (1.0 - damping) / float64(n)
	next := make([]float64, n)
	for iter := 0; iter < maxIter; iter++ {
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
			break
		}
	}
	return scores
}

// trSelectTopN selects the top-n sentences by score and returns them in document order.
// Uses a distinct name to avoid conflicts if LexRank runs in parallel.
func trSelectTopN(scores []float64, n int, sentences []string) []string {
	indices := make([]int, len(sentences))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(a, b int) bool {
		return scores[indices[a]] > scores[indices[b]]
	})
	top := indices[:n]
	sort.Ints(top)
	result := make([]string, n)
	for i, idx := range top {
		result[i] = sentences[idx]
	}
	return result
}

// Summarize implements the Summarizer interface using TextRank.
// Returns up to n sentences from text in original document order.
// Returns nil, nil for empty input. Caps n to sentence count (SUM-04).
func (t *TextRank) Summarize(text string, n int) ([]string, error) {
	sentences := TokenizeSentences(text)
	if len(sentences) == 0 {
		return nil, nil
	}
	if n > len(sentences) {
		n = len(sentences)
	}

	// Tokenize each sentence into words
	words := make([][]string, len(sentences))
	for i, s := range sentences {
		words[i] = tokenizeWords(s)
	}

	// Build n*n word overlap similarity matrix
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
	scores := powerIterateDamped(matrix, textRankDamping, textRankEpsilon, textRankMaxIter)
	return trSelectTopN(scores, n, sentences), nil
}
