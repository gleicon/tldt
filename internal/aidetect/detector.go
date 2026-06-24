// Package aidetect implements AI-generated content detection using the
// excess-vocabulary method from:
//
//	Kobak et al. (2024). "Delving into ChatGPT usage in academic writing
//	through excess vocabulary." arXiv:2406.07016.
//
// The method computes two signals over a tokenized text:
//
//	density  = fraction of sentences containing ≥1 excess marker
//	variety  = fraction of the marker vocabulary observed in the text
//	score    = 0.6*density + 0.4*variety
//
// Thresholds (calibrated on academic English in the paper):
//
//	score ≥ 0.70  → likely AI-generated
//	score ≥ 0.40  → possibly AI-generated
//	score < 0.40  → likely human-written
//
// The wordlists are language-specific JSON files embedded into the binary.
// Supported languages: "en", "pt-BR", "es".
package aidetect

import (
	"strings"
	"unicode"
)

// Result holds the output of AI content detection for one text.
type Result struct {
	Score     float64  // composite [0,1]: 0.6*density + 0.4*variety
	Density   float64  // fraction of sentences with ≥1 marker
	Variety   float64  // fraction of marker vocabulary used
	Markers   []string // unique markers found, sorted
	Lang      string   // language code used
	Sentences int      // total sentences analysed
}

// Verdict returns a human-readable interpretation of the score.
func (r Result) Verdict() string {
	switch {
	case r.Score >= 0.70:
		return "likely AI-generated"
	case r.Score >= 0.40:
		return "possibly AI-generated"
	default:
		return "likely human-written"
	}
}

// Detect scores text for AI-generated content using the excess-vocabulary
// method from Kobak et al. (2024). lang must be one of "en", "pt-BR", "es".
// An empty lang defaults to "en". Override is an optional path to a directory
// containing custom <lang>.json wordlist files; empty string uses embedded lists.
func Detect(text, lang, overrideDir string) (Result, error) {
	if lang == "" {
		lang = "en"
	}
	wl, err := loadWordlist(lang, overrideDir)
	if err != nil {
		return Result{}, err
	}

	allMarkers := make(map[string]bool, len(wl.Rare)+len(wl.Common))
	for _, w := range wl.Rare {
		allMarkers[strings.ToLower(w)] = true
	}
	for _, w := range wl.Common {
		allMarkers[strings.ToLower(w)] = true
	}

	sentences := tokenizeSentences(text)
	if len(sentences) == 0 {
		return Result{Lang: lang, Sentences: 0}, nil
	}

	// Track per-sentence hits and vocabulary coverage.
	sentencesWithMarker := 0
	usedMarkers := make(map[string]bool)

	for _, sent := range sentences {
		words := tokenizeWords(sent)
		hit := false
		for _, w := range words {
			if allMarkers[w] {
				usedMarkers[w] = true
				hit = true
			}
		}
		if hit {
			sentencesWithMarker++
		}
	}

	density := float64(sentencesWithMarker) / float64(len(sentences))
	variety := float64(len(usedMarkers)) / float64(len(allMarkers))
	score := 0.6*density + 0.4*variety

	markers := make([]string, 0, len(usedMarkers))
	for m := range usedMarkers {
		markers = append(markers, m)
	}
	sortStrings(markers)

	return Result{
		Score:     score,
		Density:   density,
		Variety:   variety,
		Markers:   markers,
		Lang:      lang,
		Sentences: len(sentences),
	}, nil
}

// tokenizeSentences splits text into sentences using punctuation boundaries.
// This is a lightweight splitter — the paper operates at sentence granularity
// but does not require a sophisticated NLP tokenizer.
func tokenizeSentences(text string) []string {
	var sentences []string
	var cur strings.Builder
	runes := []rune(text)
	for i, r := range runes {
		cur.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			// Look ahead: if the next non-space char is uppercase or EOF, split.
			rest := strings.TrimLeftFunc(string(runes[i+1:]), unicode.IsSpace)
			if rest == "" || (len(rest) > 0 && unicode.IsUpper([]rune(rest)[0])) {
				s := strings.TrimSpace(cur.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				cur.Reset()
			}
		}
	}
	if tail := strings.TrimSpace(cur.String()); tail != "" {
		sentences = append(sentences, tail)
	}
	return sentences
}

// tokenizeWords lower-cases text and splits on non-letter/non-hyphen boundaries,
// preserving hyphenated compounds (e.g., "cutting-edge").
func tokenizeWords(s string) []string {
	s = strings.ToLower(s)
	var words []string
	var cur strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || r == '-' {
			cur.WriteRune(r)
		} else {
			if w := cur.String(); w != "" && w != "-" {
				words = append(words, strings.Trim(w, "-"))
			}
			cur.Reset()
		}
	}
	if w := cur.String(); w != "" && w != "-" {
		words = append(words, strings.Trim(w, "-"))
	}
	return words
}

// sortStrings is a simple in-place insertion sort for small slices.
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		key := ss[i]
		j := i - 1
		for j >= 0 && ss[j] > key {
			ss[j+1] = ss[j]
			j--
		}
		ss[j+1] = key
	}
}
