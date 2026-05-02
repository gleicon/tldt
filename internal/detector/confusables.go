package detector

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"
)

//go:embed data/confusables.txt
var confusablesRaw string

// confusableMap maps source rune → target string (the ASCII/Latin equivalent it resembles).
// Only populated for entries where the target is ASCII or basic Latin (U+0000–U+024F),
// making the source a candidate cross-script homoglyph.
var confusableMap map[rune]string
var confusableOnce sync.Once

// loadConfusables parses confusables.txt and builds confusableMap.
// Called once via sync.Once. Format per line (non-comment, non-blank):
//
//	source_hex ; target_hex [target_hex ...] ; type # comment
//
// We index source → target only when source > U+024F and all target codepoints ≤ U+024F
// (i.e., source is non-basic-Latin but looks like basic Latin or ASCII).
func loadConfusables() {
	confusableMap = make(map[rune]string, 1024)
	for _, line := range strings.Split(confusablesRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Split on " ; " — file uses tab-separated fields with semicolon
		fields := strings.Split(line, ";")
		if len(fields) < 2 {
			continue
		}
		srcHex := strings.TrimSpace(fields[0])
		srcCP, err := strconv.ParseInt(srcHex, 16, 32)
		if err != nil || srcCP <= 0x024F {
			// Source is ASCII/basic-Latin itself — not a cross-script homoglyph
			continue
		}

		// Target field may be multiple space-separated hex codepoints
		tgtField := strings.TrimSpace(fields[1])
		// Strip inline comment if present (some lines have # mid-field)
		if idx := strings.Index(tgtField, "#"); idx >= 0 {
			tgtField = strings.TrimSpace(tgtField[:idx])
		}
		tgtParts := strings.Fields(tgtField)
		var target strings.Builder
		allBasicLatin := true
		for _, part := range tgtParts {
			cp, err := strconv.ParseInt(part, 16, 32)
			if err != nil {
				allBasicLatin = false
				break
			}
			if cp > 0x024F {
				allBasicLatin = false
				break
			}
			target.WriteRune(rune(cp))
		}
		if !allBasicLatin || target.Len() == 0 {
			continue
		}

		confusableMap[rune(srcCP)] = target.String()
	}
}

// DetectConfusables scans text for runes that are cross-script homoglyphs of ASCII/Latin
// characters as defined by UTS#39 confusables.txt. Returns one Finding per detected rune.
//
// Example: Cyrillic 'а' (U+0430) looks identical to Latin 'a' (U+0061). A document
// containing "аdmin" with a Cyrillic 'а' would bypass naive pattern matching on "admin".
//
// Only non-basic-Latin sources (codepoint > U+024F) that map to basic Latin targets
// (U+0000–U+024F) are reported. NFKC normalization does not collapse these — they require
// a confusables lookup.
//
// Score is fixed at 0.80: high enough to warrant review, not as certain as a regex match.
func DetectConfusables(text string) []Finding {
	confusableOnce.Do(loadConfusables)

	var findings []Finding
	offset := 0
	for _, r := range text {
		if target, ok := confusableMap[r]; ok {
			excerpt := string(r) + " → " + target
			findings = append(findings, Finding{
				Category: CategoryEncoding,
				Sentence: -1,
				Offset:   offset,
				Score:    0.80,
				Pattern:  "confusable-homoglyph",
				Excerpt:  excerpt,
			})
		}
		offset += len(string(r))
	}
	return findings
}
