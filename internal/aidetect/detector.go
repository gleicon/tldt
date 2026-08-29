// Package aidetect implements AI-generated content detection combining two
// complementary approaches:
//
//  1. Excess-vocabulary method (Kobak et al. 2024, arXiv:2406.07016):
//     detects words statistically overrepresented in LLM output vs. human text.
//     density  = fraction of sentences containing ≥1 excess marker
//     variety  = fraction of the marker vocabulary observed in the text
//     score    = 0.6*density + 0.4*variety
//
//  2. Linguistic/stylometric signals (classical NLP + stylometry literature,
//     complementing neural approaches reviewed in arXiv:2402.14873):
//     - Sentence length regularity (coefficient of variation)
//     - Compression ratio (gzip; model-free perplexity proxy)
//     - Discourse connector density (sentence-initial transition phrases)
//     - Type-token ratio (vocabulary diversity)
//     - Hapax ratio (rare word fraction)
//
// The CombinedScore() method blends both layers when ≥5 sentences are present.
// Thresholds apply to CombinedScore (and to Score when sentences < 5):
//
//	≥ 0.70  → likely AI-generated
//	≥ 0.40  → possibly AI-generated
//	< 0.40  → likely human-written
//
// Wordlists are language-specific JSON files embedded into the binary.
// Supported languages: "en", "pt-BR", "es".
package aidetect

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// Result holds the output of AI content detection for one text.
type Result struct {
	Score        float64           // excess-vocabulary score: 0.6*density + 0.4*variety, plus the monotonic phrase signal (capped at 1.0)
	Density      float64           // fraction of sentences with ≥1 word marker
	Variety      float64           // fraction of word-marker vocabulary used
	Markers      []string          // unique word markers found, sorted
	Phrases      []string          // unique phrase/template tells matched, sorted
	PhraseSignal float64           // additive phrase contribution folded into Score (0 when none matched)
	Lang         string            // language code used
	Sentences    int               // total sentences analysed
	Linguistic   LinguisticSignals // structural/stylometric signals (populated when Sentences ≥ 3)
}

// CombinedScore blends the Kobak excess-vocabulary score with structural
// linguistic signals when at least 5 sentences are present. Below that
// threshold the linguistic signals are statistically unreliable, so the
// Kobak score is returned unchanged.
func (r Result) CombinedScore() float64 {
	if r.Sentences < 5 {
		return r.Score
	}
	return 0.65*r.Score + 0.35*r.Linguistic.Score
}

// Verdict returns a human-readable interpretation of CombinedScore.
func (r Result) Verdict() string {
	switch s := r.CombinedScore(); {
	case s >= 0.70:
		return "likely AI-generated"
	case s >= 0.40:
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
	wordScore := 0.6*density + 0.4*variety

	// Phrase and template layer. Word density/variety above are deliberately left
	// untouched — adding phrases to the word vocabulary would grow the variety
	// denominator and silently lower every existing score. Instead the phrase
	// signal is strictly additive and monotonic: it can only raise the score, so
	// word-only text scores exactly as before and a matched tell is pure extra
	// evidence.
	phraseHits, templateHits := matchPhrases(text, wl)
	phraseSignal := phraseScore(len(phraseHits), len(templateHits))
	score := wordScore + phraseSignal
	if score > 1.0 {
		score = 1.0
	}

	markers := make([]string, 0, len(usedMarkers))
	for m := range usedMarkers {
		markers = append(markers, m)
	}
	sortStrings(markers)

	phrases := append(phraseHits, templateHits...)
	sortStrings(phrases)

	return Result{
		Score:        score,
		Density:      density,
		Variety:      variety,
		Markers:      markers,
		Phrases:      phrases,
		PhraseSignal: phraseSignal,
		Lang:         lang,
		Sentences:    len(sentences),
		Linguistic:   computeLinguistic(text, sentences),
	}, nil
}

// phraseWeight and templateWeight set how much each distinct tell contributes to
// the additive phrase signal. Templates weigh more: a structural tic like
// "not just X, but Y" is one of the strongest single markers reported (~6% of AI
// messages carry it), whereas a fixed phrase is weaker on its own.
const (
	phraseWeight    = 0.12
	templateWeight  = 0.25
	maxPhraseSignal = 0.35
)

// phraseScore combines distinct phrase and template hit counts into a bounded
// additive signal. The cap keeps a phrase-heavy text from pinning the score at
// 1.0 on phrases alone while still letting one strong template reach the
// "possibly AI-generated" band without any word markers.
func phraseScore(phrases, templates int) float64 {
	s := phraseWeight*float64(phrases) + templateWeight*float64(templates)
	if s > maxPhraseSignal {
		return maxPhraseSignal
	}
	return s
}

// matchPhrases scans the lowercased text for the wordlist's literal phrases and
// regex templates, returning the distinct phrases and distinct template pattern
// strings that matched. Matching runs on the raw text rather than the word
// tokens because these tells contain apostrophes, spaces, and punctuation the
// word tokenizer strips.
func matchPhrases(text string, wl wordlist) (phrases, templates []string) {
	lower := strings.ToLower(text)

	seenP := make(map[string]bool)
	for _, p := range wl.Phrases {
		lp := strings.ToLower(strings.TrimSpace(p))
		if lp == "" || seenP[lp] {
			continue
		}
		if strings.Contains(lower, lp) {
			seenP[lp] = true
			phrases = append(phrases, lp)
		}
	}

	for _, t := range wl.Templates {
		re := compileTemplate(t)
		if re == nil {
			continue
		}
		// Report the matched text, not the pattern, so the finding reads as the
		// actual tell ("not just fast but reliable") rather than a regex. Each
		// pattern still counts once regardless of how many times it matches.
		if m := re.FindString(lower); m != "" {
			templates = append(templates, strings.TrimSpace(m))
		}
	}
	return phrases, templates
}

// templateCache memoizes compiled template regexes across Detect calls. A failed
// compile is cached as nil so a malformed pattern is skipped rather than
// retried and never aborts detection.
var (
	templateCache   = map[string]*regexp.Regexp{}
	templateCacheMu sync.Mutex
)

func compileTemplate(pattern string) *regexp.Regexp {
	templateCacheMu.Lock()
	defer templateCacheMu.Unlock()
	if re, ok := templateCache[pattern]; ok {
		return re
	}
	// Patterns are authored lowercase-insensitive; match against lowered text, so
	// no (?i) is required, but add it defensively for any literal upper in a
	// pattern.
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		templateCache[pattern] = nil
		return nil
	}
	templateCache[pattern] = re
	return re
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
