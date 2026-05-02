package summarizer

import (
	"regexp"
	"strings"
	"unicode"
)

// sentenceEnd matches the boundary between two sentences: punctuation followed by
// whitespace and an uppercase letter (or opening quote). The match position is used
// to split text into individual sentences.
var sentenceEnd = regexp.MustCompile(`[.!?]['"` + "\u2019" + `]?\s+[A-Z'"` + "\u201C" + `]`)

// TokenizeSentences splits text into sentences using a regexp heuristic.
// Sentences are returned trimmed, in original order.
// Returns nil for empty or whitespace-only input.
func TokenizeSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sentences []string
	for {
		loc := sentenceEnd.FindStringIndex(text)
		if loc == nil {
			break
		}
		// boundary is at loc[0]+1 (after the punctuation, before the space)
		boundary := loc[0] + 1
		sentence := strings.TrimSpace(text[:boundary])
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
		text = strings.TrimSpace(text[boundary:])
	}
	// last remaining text is the final sentence
	if text != "" {
		sentences = append(sentences, text)
	}
	return sentences
}

// normalizeWord lowercases and strips non-alphanumeric characters from a word.
// Hyphens between digit/letter sequences are preserved.
func normalizeWord(word string) string {
	word = strings.ToLower(word)
	var prev rune
	mapped := strings.Map(func(r rune) rune {
		if r == '-' && (unicode.IsDigit(prev) || unicode.IsLetter(prev)) {
			prev = r
			return r
		}
		if !unicode.IsDigit(r) && !unicode.IsLetter(r) {
			return -1
		}
		prev = r
		return r
	}, word)
	return strings.TrimRight(mapped, "-")
}

// tokenizeWords returns the normalized, non-empty words for a sentence.
func tokenizeWords(sentence string) []string {
	raw := strings.Fields(sentence)
	out := make([]string, 0, len(raw))
	for _, w := range raw {
		if n := normalizeWord(w); n != "" {
			out = append(out, n)
		}
	}
	return out
}
