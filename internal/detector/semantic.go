package detector

import (
	"regexp"
	"strings"
	"unicode"
)

// Categories for the layers beyond pattern, encoding and outlier. Each is its own
// category because Recompute counts *distinct* categories for corroboration:
// folding two layers into one name would let a single layer's findings corroborate
// each other, which is exactly what CorroborationFloor exists to prevent.
const (
	// CategoryObfuscated marks a phrase that matched only after character
	// substitutions were folded away.
	CategoryObfuscated Category = "obfuscated"
	// CategoryRole marks chat-template and conversational role markers.
	CategoryRole Category = "role"
	// CategoryExfil marks link constructs shaped to carry data outward.
	CategoryExfil Category = "exfil"
	// CategoryPositional marks signals derived from placement rather than content.
	CategoryPositional Category = "positional"
	// CategoryScript marks a sentence whose dominant Unicode script differs from
	// the document's.
	CategoryScript Category = "script"
)

// --- Obfuscation folding (FR-7, FR-8) ---

// leetMap folds common character substitutions. Every entry maps one byte to one
// byte so the folded copy is the same length as the input and byte offsets still
// index the original text (FR-9).
//
// '1' folds to 'i' rather than 'l' because the injection corpus uses it that way
// ("1gn0r3"); a fold table cannot serve both readings at once.
var leetMap = [256]byte{
	'4': 'a', '@': 'a', '3': 'e', '1': 'i', '!': 'i', '|': 'l',
	'0': 'o', '5': 's', '$': 's', '7': 't', '+': 't', '8': 'b',
	'9': 'g', '6': 'g', '2': 'z',
}

// foldObfuscation returns a lowercased copy of text with leet substitutions
// resolved, and whether anything changed. The result is for matching only and is
// never emitted: findings quote the original bytes.
func foldObfuscation(text string) (string, bool) {
	return foldLower(asciiLower(text))
}

// foldLower applies leet substitutions to an already-lowercased string.
func foldLower(lower string) (string, bool) {
	b := []byte(lower)
	changed := false
	for i := range b {
		if r := leetMap[b[i]]; r != 0 {
			b[i] = r
			changed = true
		}
	}
	return string(b), changed
}

// obfuscatedScoreFactor scales a folded match below its literal equivalent.
// Folding never self-rejects — "1gn0r3" always resolves to something — so a match
// found only after folding is weaker evidence than one found literally.
const obfuscatedScoreFactor = 0.7

// DetectObfuscated reports injection phrases that match only after character
// substitutions are folded away. Matches that the literal pass already found are
// excluded: reporting them twice would let one phrase corroborate itself.
func DetectObfuscated(text string) []Finding {
	return detectObfuscatedIn(text, asciiLower(text))
}

func detectObfuscatedIn(text, lower string) []Finding {
	folded, changed := foldLower(lower)
	if !changed {
		return nil
	}

	// Collect folded matches first. On ordinary prose no pattern anchor survives
	// folding, so this loop skips every regex via the prefilter and returns
	// nothing — at which point the literal-exclusion pass below is never run.
	// Building the literal set eagerly would cost a second full pattern pass over
	// the whole document (measured ~42 ms at 256 KB) on every call.
	type match struct {
		offset, end int
		score       float64
	}
	var matches []match
	for _, p := range preparedPatterns {
		if !anchorsPresent(folded, p.anchors) {
			continue
		}
		for _, loc := range p.re.FindAllStringIndex(folded, -1) {
			matches = append(matches, match{loc[0], loc[1], p.confidence * obfuscatedScoreFactor})
		}
	}
	if len(matches) == 0 {
		return nil
	}

	// A folded match that the literal pass already found is not obfuscation; drop
	// it so a plain phrase does not corroborate itself across two layers.
	literal := make(map[int]bool)
	for _, f := range DetectPatterns(text) {
		literal[f.Offset] = true
	}

	var findings []Finding
	for _, m := range matches {
		if literal[m.offset] {
			continue
		}
		findings = append(findings, Finding{
			Category: CategoryObfuscated,
			Sentence: -1,
			Offset:   m.offset,
			Score:    m.score,
			Pattern:  "injection-obfuscated",
			// Excerpt quotes the original bytes, not the folded copy.
			Excerpt: truncateExcerpt(text[m.offset:m.end], 80, "…"),
		})
	}
	return findings
}

// --- Role and chat-template markers (FR-21) ---

// roleMarkers are literal tokens that only appear when someone is trying to forge
// a conversation boundary. They are matched case-insensitively against the folded
// copy of the input; unlike the injection patterns these are fixed strings, so the
// layer costs one substring scan each and produces almost no false positives in
// prose.
var roleMarkers = []struct {
	literal string
	score   float64
}{
	{"<|im_start|>", 0.95},
	{"<|im_end|>", 0.95},
	{"<|endoftext|>", 0.90},
	{"<|system|>", 0.90},
	{"[inst]", 0.90},
	{"[/inst]", 0.90},
	{"<<sys>>", 0.90},
	{"### human:", 0.85},
	{"### assistant:", 0.85},
	{"### system:", 0.85},
	{"</system>", 0.90},
	{"</instructions>", 0.85},
	{"<function_calls>", 0.85},
	{"<function_results>", 0.85},
	{"</invoke>", 0.85},
	{"<invoke name=", 0.80},
	{"h:\nassistant:", 0.75},
}

// fabricatedTurnRE matches a forged conversational turn at the start of a line.
var fabricatedTurnRE = regexp.MustCompile(`(?m)^\s*(human|assistant|system)\s*:\s*\S`)

// DetectRoleMarkers reports chat-template tokens and forged conversation turns.
func DetectRoleMarkers(text string) []Finding {
	return detectRoleMarkersIn(text, asciiLower(text))
}

func detectRoleMarkersIn(text, lower string) []Finding {
	var findings []Finding

	for _, m := range roleMarkers {
		off := 0
		for {
			i := strings.Index(lower[off:], m.literal)
			if i < 0 {
				break
			}
			at := off + i
			findings = append(findings, Finding{
				Category: CategoryRole,
				Sentence: -1,
				Offset:   at,
				Score:    m.score,
				Pattern:  "role-marker",
				Excerpt:  truncateExcerpt(text[at:min(at+len(m.literal)+20, len(text))], 80, "…"),
			})
			off = at + len(m.literal)
		}
	}

	for _, loc := range fabricatedTurnRE.FindAllStringIndex(lower, -1) {
		findings = append(findings, Finding{
			Category: CategoryRole,
			Sentence: -1,
			Offset:   loc[0],
			Score:    0.70,
			Pattern:  "fabricated-turn",
			Excerpt:  truncateExcerpt(text[loc[0]:loc[1]], 80, "…"),
		})
	}
	return findings
}

// --- Markdown exfiltration (FR-22) ---

// mdLinkRE captures Markdown inline links and images: the optional '!' marks an
// image, group 2 is the target URL.
var mdLinkRE = regexp.MustCompile(`(!?)\[[^\]]*\]\(([^)\s]+)`)

// autolinkRE captures bare <http://...> autolinks.
var autolinkRE = regexp.MustCompile(`<(https?://[^>\s]+)>`)

// refLinkRE captures reference-style link definitions: [id]: http://...
var refLinkRE = regexp.MustCompile(`(?m)^\s*\[[^\]]+\]:\s*(https?://\S+)`)

// dataShapeRE matches a value that looks like carried data rather than a path: a
// base64-ish run, a percent-encoded block, or a template interpolation.
var dataShapeRE = regexp.MustCompile(`[A-Za-z0-9+/_-]{16,}={0,2}|(?:%[0-9a-fA-F]{2}){3,}|\{\{[^}]+\}\}|\$\{[^}]+\}`)

// carriesData reports whether a URL's query, fragment, or path carries something
// shaped like encoded or templated data.
//
// Keying on structure rather than on a host allowlist is deliberate: an allowlist
// has to be maintained, defaults badly (an empty list flags every external link in
// an ordinary README), and does not describe the actual risk. A link carrying an
// encoded blob is suspicious wherever it points; a plain documentation link is not
// suspicious even pointing somewhere unfamiliar.
func carriesData(raw string) (string, bool) {
	i := strings.IndexAny(raw, "?#")
	tail := ""
	if i >= 0 {
		tail = raw[i+1:]
	}
	if m := dataShapeRE.FindString(tail); m != "" {
		return m, true
	}
	// Also consider a path segment, which is how a bare-path beacon is built.
	if i < 0 {
		if slash := strings.LastIndex(raw, "/"); slash >= 0 {
			if m := dataShapeRE.FindString(raw[slash+1:]); m != "" {
				return m, true
			}
		}
	}
	return "", false
}

// DetectExfil reports link constructs shaped to carry data to a remote host.
func DetectExfil(text string) []Finding {
	var findings []Finding

	add := func(offset int, url, payload string, isImage bool) {
		score := 0.70
		pattern := "exfil-link"
		if isImage {
			// An image renders automatically, so the request fires without the
			// reader clicking anything.
			score = 0.85
			pattern = "exfil-image"
		}
		findings = append(findings, Finding{
			Category: CategoryExfil,
			Sentence: -1,
			Offset:   offset,
			Score:    score,
			Pattern:  pattern,
			Excerpt:  truncateExcerpt(url+" [carries: "+payload+"]", 80, "…"),
		})
	}

	for _, m := range mdLinkRE.FindAllStringSubmatchIndex(text, -1) {
		target := text[m[4]:m[5]]
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			continue
		}
		if payload, ok := carriesData(target); ok {
			add(m[0], target, payload, text[m[2]:m[3]] == "!")
		}
	}
	for _, m := range autolinkRE.FindAllStringSubmatchIndex(text, -1) {
		target := text[m[2]:m[3]]
		if payload, ok := carriesData(target); ok {
			add(m[0], target, payload, false)
		}
	}
	for _, m := range refLinkRE.FindAllStringSubmatchIndex(text, -1) {
		target := text[m[2]:m[3]]
		if payload, ok := carriesData(target); ok {
			add(m[0], target, payload, false)
		}
	}
	return findings
}

// --- Positional and structural heuristics (FR-23) ---

const (
	// gapRunes is the run of blank space after which following text is treated as
	// deliberately separated from the document body.
	gapRunes = 200
	// tailFraction is the trailing portion of a document treated as the tail.
	tailFraction = 0.9
	// minRepeatBlocks is how many repeated turn-shaped blocks constitute a
	// many-shot pattern.
	minRepeatBlocks = 5
)

var whitespaceGapRE = regexp.MustCompile(`(?:[ \t]*\n){4,}[ \t]*|[ \t]{200,}`)

// DetectPositional reports signals derived from where text sits rather than what
// it says. These are weak priors by construction: they score below
// DefaultDetectionThreshold and can only mark a report suspicious by corroborating
// a finding from another layer.
func DetectPositional(text string) []Finding {
	return detectPositionalIn(text, asciiLower(text))
}

func detectPositionalIn(text, lower string) []Finding {
	var findings []Finding

	// Instructions after a large whitespace gap: content pushed out of sight.
	for _, loc := range whitespaceGapRE.FindAllStringIndex(text, -1) {
		after := text[loc[1]:]
		if len(after) == 0 {
			continue
		}
		if hits := DetectPatterns(after); len(hits) > 0 {
			findings = append(findings, Finding{
				Category: CategoryPositional,
				Sentence: -1,
				Offset:   loc[1],
				Score:    0.60,
				Pattern:  "post-gap-instruction",
				Excerpt:  truncateExcerpt(strings.TrimSpace(after), 80, "…"),
			})
			break
		}
	}

	// Instructions in the document tail, where a reader has stopped looking.
	if len(text) > 500 {
		tailStart := int(float64(len(text)) * tailFraction)
		for _, f := range DetectPatterns(text[tailStart:]) {
			findings = append(findings, Finding{
				Category: CategoryPositional,
				Sentence: -1,
				Offset:   tailStart + f.Offset,
				Score:    0.55,
				Pattern:  "tail-instruction",
				Excerpt:  f.Excerpt,
			})
			break
		}
	}

	// Many-shot repetition: a long run of forged turns conditioning the model.
	if n := len(fabricatedTurnRE.FindAllStringIndex(lower, -1)); n >= minRepeatBlocks {
		findings = append(findings, Finding{
			Category: CategoryPositional,
			Sentence: -1,
			Offset:   0,
			Score:    0.65,
			Pattern:  "many-shot-repetition",
			Excerpt:  itoaPositive(n) + " forged conversational turns",
		})
	}
	return findings
}

func itoaPositive(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// --- Script-run mismatch (FR-24) ---

// scriptOf classifies a rune into a coarse script bucket. Only scripts that carry
// their own alphabet are distinguished; punctuation, digits and spaces are
// neutral and return "".
func scriptOf(r rune) string {
	switch {
	case !unicode.IsLetter(r):
		return ""
	case unicode.Is(unicode.Latin, r):
		return "Latin"
	case unicode.Is(unicode.Cyrillic, r):
		return "Cyrillic"
	case unicode.Is(unicode.Greek, r):
		return "Greek"
	case unicode.Is(unicode.Arabic, r):
		return "Arabic"
	case unicode.Is(unicode.Hebrew, r):
		return "Hebrew"
	case unicode.Is(unicode.Han, r):
		return "Han"
	case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
		return "Kana"
	case unicode.Is(unicode.Hangul, r):
		return "Hangul"
	case unicode.Is(unicode.Devanagari, r):
		return "Devanagari"
	case unicode.Is(unicode.Thai, r):
		return "Thai"
	}
	return "Other"
}

// dominantScript returns the most frequent script in s and its share of letters.
func dominantScript(s string) (string, float64) {
	counts := map[string]int{}
	total := 0
	for _, r := range s {
		if sc := scriptOf(r); sc != "" {
			counts[sc]++
			total++
		}
	}
	if total == 0 {
		return "", 0
	}
	best, bestN := "", 0
	for sc, n := range counts {
		if n > bestN {
			best, bestN = sc, n
		}
	}
	return best, float64(bestN) / float64(total)
}

// minScriptLetters is the shortest sentence worth classifying. Below it, a couple
// of borrowed words dominate the count and the signal is noise.
const minScriptLetters = 20

// DetectScriptMismatch reports sentences whose dominant Unicode script differs
// from the document's.
//
// This is deliberately script detection, not language detection. The repository
// ships no language model: internal/aidetect's wordlists are excess-vocabulary
// markers from Kobak et al., which identify AI-generated text rather than
// language. Script runs are computable from unicode range tables alone and catch
// the actual evasion — a payload written in an alphabet the pattern set does not
// cover — without claiming a capability the data cannot support. A same-script
// cross-language payload (Portuguese inside English prose) is not detected.
func DetectScriptMismatch(sentences []string, fullText string) []Finding {
	docScript, docShare := dominantScript(fullText)
	if docScript == "" || docShare < 0.5 {
		return nil
	}

	var findings []Finding
	for i, sent := range sentences {
		letters := 0
		for _, r := range sent {
			if scriptOf(r) != "" {
				letters++
			}
		}
		if letters < minScriptLetters {
			continue
		}
		sc, share := dominantScript(sent)
		if sc == "" || sc == docScript || share < 0.6 {
			continue
		}
		findings = append(findings, Finding{
			Category: CategoryScript,
			Sentence: i,
			Offset:   -1,
			Score:    0.60,
			Pattern:  "script-mismatch",
			Excerpt:  truncateExcerpt(sent, 80, "…"),
			// Provenance names both scripts so the reader can judge the finding
			// without re-deriving it.
			Provenance: docScript + "->" + sc,
		})
	}
	return findings
}
