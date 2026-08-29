// Package detector identifies prompt injection patterns in text before it enters
// an AI model's context. It implements four complementary detection layers:
//
//  1. Pattern matching — regex against taxonomized injection phrase categories
//  2. Encoding anomaly — base64 and hex payload detection via entropy analysis
//  3. Statistical outlier — cosine similarity scoring via the LexRank similarity matrix
//
// Detection is always advisory: findings are reported to stderr and never cause the
// tool to refuse or modify input without explicit --sanitize/--quarantine flags.
//
// False positive philosophy: prefer reporting to blocking. The tool's role is to
// surface suspicious content, not to act as a content policy enforcement layer.
package detector

import (
	"encoding/base64"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Category classifies the type of detected injection signal.
type Category string

const (
	CategoryPattern  Category = "pattern"  // textual injection phrase
	CategoryEncoding Category = "encoding" // obfuscated payload (base64, hex, ctrl-chars)
	CategoryOutlier  Category = "outlier"  // statistical off-topic sentence
)

// Finding describes a single injection signal.
type Finding struct {
	Category Category
	Sentence int     // index into sentence list; -1 if not sentence-scoped
	Offset   int     // byte offset in source text; -1 if not applicable
	Score    float64 // 0.0–1.0 confidence estimate
	Pattern  string  // pattern/detector name that triggered
	Excerpt  string  // up to 80 chars of matched/suspicious content

	// Provenance records how the matched text was recovered when it was not
	// present literally in the input: an encoding chain such as "base64" or
	// "percent>base64", or a hidden-surface source such as "pdf:annotation".
	// Empty for findings matched directly against the caller's text.
	Provenance string
}

// Report aggregates all findings from a full analysis pass.
type Report struct {
	Findings []Finding
	MaxScore float64

	// CorroboratingLayers counts distinct detection categories that produced a
	// finding at or above CorroborationFloor. Two or more is sufficient for
	// Suspicious on its own.
	CorroboratingLayers int

	// Suspicious is true when MaxScore exceeds DefaultDetectionThreshold, or when
	// CorroboratingLayers is at least 2.
	Suspicious bool
}

// DefaultDetectionThreshold is the score above which a report is marked Suspicious.
const DefaultDetectionThreshold = 0.70

// CorroborationFloor is the score at which a finding counts as corroborating
// evidence. Two findings from *distinct* layers at or above this floor mark a
// report suspicious even when neither reaches DefaultDetectionThreshold.
//
// Rationale: a pure max is sound only while every layer is high-precision. Once
// weak-prior layers exist (positional placement, script mismatch, obfuscated
// matches), three independent 0.6 signals are far more alarming than any one of
// them, yet max would report 0.6 and leave the verdict clean. Restricting
// compounding to distinct layers stops ten findings from a single weak layer
// manufacturing a verdict on their own.
//
// The value is calibrated against the false-positive corpus by
// TestCorroborationFloorCalibration rather than asserted here.
const CorroborationFloor = 0.5

// Recompute refreshes MaxScore and Suspicious from the current Findings slice.
// Callers that append findings after Analyze (notably the outlier pass, which
// needs a similarity matrix Analyze does not build) must call this to keep the
// verdict consistent with the evidence.
func (r *Report) Recompute() {
	var maxScore float64
	layers := make(map[Category]bool)
	for _, f := range r.Findings {
		// Outlier scores are a dissimilarity metric on a different scale — a
		// normal sentence scores ~0.97 — so they never contribute to MaxScore or
		// to corroboration. Including them would mark every document suspicious.
		if f.Category == CategoryOutlier {
			continue
		}
		if f.Score > maxScore {
			maxScore = f.Score
		}
		if f.Score >= CorroborationFloor {
			layers[f.Category] = true
		}
	}
	r.MaxScore = maxScore
	r.CorroboratingLayers = len(layers)
	r.Suspicious = maxScore > DefaultDetectionThreshold || len(layers) >= 2
}

// MaxOutlierSentences caps the number of sentences fed to the outlier pass.
//
// Outlier detection consumes an n×n LexRank similarity matrix, so its cost is
// quadratic in sentence count. Measured on an Apple M5, the uncapped pipeline ran
// 57.6 ms at 32 KB, 199 ms at 64 KB and 2873 ms at 256 KB — doubling ratios of
// 3.4–3.9x.
//
// The value is set from measurement rather than intuition: 208 sentences costs
// ~43 ms, and quadratic scaling puts 400 sentences at ~160 ms, which alone
// exceeds the whole-pipeline budget. 250 sentences costs ~62 ms and leaves room
// for the rest of the pipeline.
//
// This is unrelated to the CLI's 2000-sentence input cap, which bounds
// summarization rather than detection. 2000 sentences is 4M similarity
// computations and was never a latency bound for this pass.
const MaxOutlierSentences = 250

// SampleSentences reduces sentences to at most MaxOutlierSentences by taking an
// evenly spaced sample, and returns the sample alongside the original index of
// each retained sentence. Even spacing rather than truncation keeps coverage
// across the whole document: an injection planted in the final paragraph of a long
// file is exactly the case truncation would miss.
func SampleSentences(sentences []string) (sample []string, originalIndex []int) {
	if len(sentences) <= MaxOutlierSentences {
		idx := make([]int, len(sentences))
		for i := range sentences {
			idx[i] = i
		}
		return sentences, idx
	}
	sample = make([]string, 0, MaxOutlierSentences)
	originalIndex = make([]int, 0, MaxOutlierSentences)
	step := float64(len(sentences)) / float64(MaxOutlierSentences)
	for i := 0; i < MaxOutlierSentences; i++ {
		j := int(float64(i) * step)
		if j >= len(sentences) {
			j = len(sentences) - 1
		}
		sample = append(sample, sentences[j])
		originalIndex = append(originalIndex, j)
	}
	return sample, originalIndex
}

// DefaultOutlierThreshold is the outlier_score above which a sentence is flagged.
// outlier_score(i) = 1 - mean(sim[i][j] for j ≠ i).
// Higher = lower similarity to neighbors = more out-topic.
//
// Calibration: Normal text produces outlier scores around 0.96-0.99 due to
// TF-IDF cosine similarity properties. A threshold of 0.99 catches only
// sentences with mean similarity < 0.01 (extremely anomalous) while avoiding
// false positives on legitimate text.
const DefaultOutlierThreshold = 0.99

// --- Pattern detection ---

// patternDef pairs a human-readable name with a compiled regex and confidence score.
//
// anchors is a conjunction of disjunctions: every group must contribute at least
// one literal present in the input before re is executed. Anchors are necessary
// conditions for a match, never sufficient ones — a pattern whose anchors all hit
// still has to match its regex. Getting an anchor wrong in the other direction
// (listing a literal the regex does not require) silently disables the pattern,
// which is why TestPatternAnchorCoverage asserts every pattern against a known
// matching sample.
type patternDef struct {
	name       string
	re         *regexp.Regexp
	confidence float64
	anchors    [][]string
}

// preparedPattern resolves case folding once at init. Patterns written with the
// (?i) flag are recompiled without it and matched against an ASCII-lowercased copy
// of the input; case-sensitive patterns keep their original regex and match the
// original bytes. Folding once for the whole set costs one pass over the input
// instead of one per pattern.
type preparedPattern struct {
	patternDef
	folded bool // match against the lowercased copy rather than the original
}

// preparedPatterns is injectionPatterns with (?i) hoisted out of each regex.
var preparedPatterns = preparePatterns(injectionPatterns)

func preparePatterns(defs []patternDef) []preparedPattern {
	out := make([]preparedPattern, 0, len(defs))
	for _, d := range defs {
		src := d.re.String()
		if rest, ok := strings.CutPrefix(src, "(?i)"); ok {
			if c, bad := uppercaseLiteral(rest); bad {
				// Hoisting (?i) makes the pattern case-sensitive against
				// lowercased text, so an uppercase literal would silently stop
				// matching. Fail at init rather than lose a detector quietly.
				panic("detector: case-insensitive pattern " + d.name +
					" contains uppercase literal " + string(c) + "; write it lowercase")
			}
			d.re = regexp.MustCompile(rest)
			out = append(out, preparedPattern{patternDef: d, folded: true})
			continue
		}
		out = append(out, preparedPattern{patternDef: d})
	}
	return out
}

// uppercaseLiteral reports the first uppercase ASCII letter in a regex source
// that is not part of an escape sequence. Escapes are skipped because \\W, \\S and
// \\B mean the opposite of their lowercase forms.
func uppercaseLiteral(src string) (byte, bool) {
	for i := 0; i < len(src); i++ {
		if src[i] == '\\' {
			i++ // skip the escaped character, whatever its case
			continue
		}
		if src[i] >= 'A' && src[i] <= 'Z' {
			return src[i], true
		}
	}
	return 0, false
}

// asciiLower lowercases A-Z and leaves every other byte untouched. Unlike
// strings.ToLower it is length-preserving, so byte offsets into the result index
// the original text exactly — required because findings must report offsets into
// the caller's input (see FR-9). Every injection pattern is ASCII, so restricting
// the fold to ASCII loses nothing.
func asciiLower(s string) string {
	hasUpper := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		return s
	}
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// anchorsPresent reports whether every anchor group has at least one literal in
// lower. An empty anchor set means the pattern always runs.
func anchorsPresent(lower string, anchors [][]string) bool {
	for _, group := range anchors {
		hit := false
		for _, lit := range group {
			if strings.Contains(lower, lit) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// injectionPatterns is the canonical set of known injection patterns.
// Patterns are case-insensitive multi-word phrases to minimize false positives
// on single common words (e.g. "ignore" alone is not a signal).
var injectionPatterns = []patternDef{
	// Direct instruction override
	{
		name:       "direct-override",
		re:         regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+instructions?`),
		confidence: 0.95,
		anchors:    [][]string{{"ignore"}, {"instruction"}},
	},
	{
		name:       "direct-override",
		re:         regexp.MustCompile(`(?i)disregard\s+(all\s+)?(the\s+)?(previous|prior|above|following)`),
		confidence: 0.90,
		anchors:    [][]string{{"disregard"}},
	},
	{
		name:       "direct-override",
		re:         regexp.MustCompile(`(?i)forget\s+(all\s+)?(previous|prior|above|your)\s+(instructions?|context|conversation)`),
		confidence: 0.90,
		anchors:    [][]string{{"forget"}},
	},
	// Role injection
	{
		name:       "role-injection",
		re:         regexp.MustCompile(`(?i)you\s+are\s+now\s+(a|an)\s+\w+`),
		confidence: 0.80,
		anchors:    [][]string{{"you"}, {"are"}, {"now"}},
	},
	{
		name:       "role-injection",
		re:         regexp.MustCompile(`(?i)(act|behave|pretend|respond)\s+as\s+(if\s+)?(you\s+(are|were|have|had))`),
		confidence: 0.75,
		anchors:    [][]string{{"act", "behave", "pretend", "respond"}, {"as"}, {"you"}},
	},
	{
		name:       "role-injection",
		re:         regexp.MustCompile(`(?i)your\s+(new\s+)?(role|persona|character|identity)\s+is`),
		confidence: 0.80,
		anchors:    [][]string{{"your"}, {"role", "persona", "character", "identity"}, {"is"}},
	},
	// Delimiter injection (model-specific special tokens)
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`(?i)<\s*/?\s*(system|instructions?|prompt|context)\s*>`),
		confidence: 0.85,
		anchors:    [][]string{{"<"}, {"system", "instruction", "prompt", "context"}},
	},
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`(?i)---+\s*(begin|end|start)\s+(system\s+)?prompt\s*---+`),
		confidence: 0.90,
		anchors:    [][]string{{"---"}, {"begin", "end", "start"}, {"prompt"}},
	},
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`\[/?INST\]`),
		confidence: 0.85,
		anchors:    [][]string{{"inst]"}},
	},
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`\|im_(start|end)\|`),
		confidence: 0.90,
		anchors:    [][]string{{"|im_"}},
	},
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`(?i)###\s*(instruction|system|prompt|context|task)`),
		confidence: 0.80,
		anchors:    [][]string{{"###"}, {"instruction", "system", "prompt", "context", "task"}},
	},
	// Conversational role prefixes (context-dependent — moderate confidence)
	{
		name:       "role-prefix",
		re:         regexp.MustCompile(`(?m)^(System|Assistant|Human|User)\s*:\s*.{10,}`),
		confidence: 0.65,
		anchors:    [][]string{{"system", "assistant", "human", "user"}},
	},
	// Jailbreak patterns
	{
		name: "jailbreak",
		// Literal is written lowercase because (?i) is hoisted out at init and
		// the regex runs against an ASCII-lowercased copy; see preparePatterns.
		re:         regexp.MustCompile(`(?i)\bdan\b.{0,30}(mode|enabled|activated|persona)`),
		confidence: 0.85,
		anchors:    [][]string{{"dan"}, {"mode", "enabled", "activated", "persona"}},
	},
	{
		name:       "jailbreak",
		re:         regexp.MustCompile(`(?i)(developer|jailbreak|unrestricted|unfiltered)\s+mode`),
		confidence: 0.80,
		anchors:    [][]string{{"developer", "jailbreak", "unrestricted", "unfiltered"}, {"mode"}},
	},
	{
		name:       "jailbreak",
		re:         regexp.MustCompile(`(?i)pretend\s+(you\s+have\s+no|there\s+are\s+no)\s+(restrictions?|guidelines?|rules?|limits?)`),
		confidence: 0.85,
		anchors:    [][]string{{"pretend"}, {"no"}, {"restriction", "guideline", "rule", "limit"}},
	},
	// Exfiltration patterns
	{
		name:       "exfiltration",
		re:         regexp.MustCompile(`(?i)(repeat|print|output|reveal|show|display)\s+(everything|all(thing)?s?)?\s*(above|before|prior|from\s+the\s+(beginning|start))`),
		confidence: 0.80,
		anchors:    [][]string{{"repeat", "print", "output", "reveal", "show", "display"}, {"above", "before", "prior", "beginning", "start"}},
	},
	{
		name:       "exfiltration",
		re:         regexp.MustCompile(`(?i)(what\s+(are|were)\s+your\s+(instructions?|system\s+prompt|guidelines?))`),
		confidence: 0.75,
		anchors:    [][]string{{"what"}, {"your"}, {"instruction", "system", "guideline"}},
	},
	{
		name:       "exfiltration",
		re:         regexp.MustCompile(`(?i)(print|output|show|display|repeat|reveal)\s+your\s+(system\s+)?(prompt|instructions?)`),
		confidence: 0.85,
		anchors:    [][]string{{"print", "output", "show", "display", "repeat", "reveal"}, {"your"}, {"prompt", "instruction"}},
	},
	// Social engineering — behavioral manipulation via urgency, threats, or
	// covert instruction injection (e.g. hidden in HTML comments or metadata).
	{
		// Matches "append your username to your User-Agent header",
		// "add a custom header called X-Foo", etc.
		name:       "social-engineering",
		re:         regexp.MustCompile(`(?i)(append|add)\s+.{0,60}(user.?agent|custom\s+header)`),
		confidence: 0.75,
		anchors:    [][]string{{"append", "add"}, {"agent", "header"}},
	},
	{
		// Matches "you have only one attempt", "you only have one chance", etc.
		name:       "social-engineering",
		re:         regexp.MustCompile(`(?i)you\s+(have\s+only|only\s+have)\s+(one|1|a\s+single)\s+(attempt|chance|try|shot)`),
		confidence: 0.85,
		anchors:    [][]string{{"you"}, {"only"}, {"attempt", "chance", "try", "shot"}},
	},
	{
		// Matches "flagged as malicious", "IP banned", "have your IP banned", etc.
		name:       "social-engineering",
		re:         regexp.MustCompile(`(?i)(flagged?\s+as\s+malicious|ip\s+ban(ned)?|have\s+your\s+ip\s+ban)`),
		confidence: 0.80,
		anchors:    [][]string{{"malicious", "ban"}},
	},
}

// DetectPatterns scans text for known injection phrases.
// Returns one Finding per pattern match. Text is not modified.
//
// The input is ASCII-lowercased once and each pattern is gated behind a literal
// anchor check before its regex runs. On text containing no anchor the regex pass
// is skipped entirely; see BenchmarkPatternPass for the measured effect.
func DetectPatterns(text string) []Finding {
	return detectPatternsIn(text, "")
}

// detectPatternsIn is DetectPatterns with optional provenance stamped onto every
// finding. provenance is empty for direct scans and carries an encoding chain
// (for example "base64") when the text was recovered by the decoder.
func detectPatternsIn(text, provenance string) []Finding {
	return detectPatternsWithLower(text, asciiLower(text), provenance)
}

func detectPatternsWithLower(text, lower, provenance string) []Finding {
	var findings []Finding
	for _, p := range preparedPatterns {
		if !anchorsPresent(lower, p.anchors) {
			continue
		}
		subject := text
		if p.folded {
			subject = lower
		}
		for _, loc := range p.re.FindAllStringIndex(subject, -1) {
			start, end := loc[0], loc[1]
			// Excerpt always quotes the original bytes, never the folded copy.
			excerpt := truncateExcerpt(text[start:end], 80, "…")
			findings = append(findings, Finding{
				Category:   CategoryPattern,
				Sentence:   -1,
				Offset:     start,
				Score:      p.confidence,
				Pattern:    p.name,
				Excerpt:    excerpt,
				Provenance: provenance,
			})
		}
	}
	return findings
}

// --- Encoding anomaly detection ---

// shannonEntropy computes the per-character Shannon entropy of s (bits/char).
// High entropy suggests dense/encoded content rather than natural language.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	total := 0
	for _, r := range s {
		freq[r]++
		total++
	}
	var entropy float64
	for _, count := range freq {
		p := float64(count) / float64(total)
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// base64RE matches candidate base64 tokens: alphabet chars, min 20 chars, valid padding.
var base64RE = regexp.MustCompile(`[A-Za-z0-9+/]{20,}={0,2}`)

// hexEscapeRE matches \x-escaped hex sequences (4+ bytes).
var hexEscapeRE = regexp.MustCompile(`(?:\\x[0-9a-fA-F]{2}){4,}`)

// hexStringRE matches raw hex strings (32+ chars = 16+ bytes).
var hexStringRE = regexp.MustCompile(`\b[0-9a-fA-F]{32,}\b`)

// highEntropyBase64 returns the byte ranges of base64-shaped tokens in text that
// decode cleanly and exceed the secret-material entropy threshold (random key
// blobs, not prose). Shared by the encoding advisory (DetectEncoding) and PII
// redaction (scanPII) so both agree on what counts as a likely-secret blob.
func highEntropyBase64(text string) [][2]int {
	var ranges [][2]int
	for _, loc := range base64RE.FindAllStringIndex(text, -1) {
		candidate := text[loc[0]:loc[1]]
		// Re-pad to a multiple of 4 before decoding. Strip any captured padding
		// first so a token that already ends in '=' is padded correctly rather
		// than over-padded into an invalid string (which would skip a real secret).
		body := strings.TrimRight(candidate, "=")
		padded := body + strings.Repeat("=", (4-len(body)%4)%4)
		if _, err := base64.StdEncoding.DecodeString(padded); err != nil {
			continue
		}
		if shannonEntropy(candidate) > 4.5 {
			ranges = append(ranges, [2]int{loc[0], loc[1]})
		}
	}
	return ranges
}

// DetectEncoding scans text for base64 payloads, hex-encoded data, and
// abnormally high control character density.
func DetectEncoding(text string) []Finding {
	var findings []Finding

	// Base64 detection: high-entropy, cleanly-decodable blobs.
	for _, r := range highEntropyBase64(text) {
		excerpt := truncateExcerpt(text[r[0]:r[1]], 80, "…")
		findings = append(findings, Finding{
			Category: CategoryEncoding,
			Sentence: -1,
			Offset:   r[0],
			Score:    0.75,
			Pattern:  "base64",
			Excerpt:  excerpt,
		})
	}

	// Hex escape detection: \x41\x42... sequences
	for _, loc := range hexEscapeRE.FindAllStringIndex(text, -1) {
		excerpt := text[loc[0]:loc[1]]
		excerpt = truncateExcerpt(excerpt, 80, "…")
		findings = append(findings, Finding{
			Category: CategoryEncoding,
			Sentence: -1,
			Offset:   loc[0],
			Score:    0.80,
			Pattern:  "hex-escape",
			Excerpt:  excerpt,
		})
	}

	// Raw hex string detection
	for _, loc := range hexStringRE.FindAllStringIndex(text, -1) {
		candidate := text[loc[0]:loc[1]]
		entropy := shannonEntropy(candidate)
		// Legitimate hex strings (UUIDs, hashes) have entropy > 3.0
		// English text has ~4.5 bits/char but hex alphabet is only 16 chars → ~4.0 max
		// Use length to differentiate: UUIDs = 32–36 chars; injection = longer
		if entropy > 3.5 && len(candidate) > 40 {
			excerpt := candidate
			excerpt = truncateExcerpt(excerpt, 80, "…")
			findings = append(findings, Finding{
				Category: CategoryEncoding,
				Sentence: -1,
				Offset:   loc[0],
				Score:    0.65,
				Pattern:  "hex-string",
				Excerpt:  excerpt,
			})
		}
	}

	// Control character density
	var controlCount, total int
	for _, r := range text {
		total++
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
			controlCount++
		}
	}
	if total > 0 {
		density := float64(controlCount) / float64(total)
		if density > 0.01 {
			findings = append(findings, Finding{
				Category: CategoryEncoding,
				Sentence: -1,
				Offset:   -1,
				Score:    math.Min(density*50, 0.90), // scale: 2% → 1.0 capped at 0.90
				Pattern:  "ctrl-char-density",
				Excerpt:  strings.Repeat("?", min(controlCount, 10)) + " (control chars)",
			})
		}
	}

	return findings
}

// --- Outlier detection ---

// DetectOutliers computes per-sentence outlier scores from the LexRank similarity
// matrix and returns findings for sentences above threshold.
//
// outlier_score(i) = 1 - mean(simMatrix[i][j] for j ≠ i)
//
// A score near 1.0 means the sentence shares very little vocabulary/semantic content
// with its document neighbors — a strong indicator of off-topic injection.
//
// simMatrix must be square (n×n) and match len(sentences).
// threshold: sentences with outlier_score > threshold are returned as findings.
func DetectOutliers(sentences []string, simMatrix [][]float64, threshold float64) []Finding {
	n := len(sentences)
	if n == 0 || len(simMatrix) != n {
		return nil
	}

	var findings []Finding
	for i := range n {
		if len(simMatrix[i]) != n {
			continue
		}
		var sum float64
		count := 0
		for j := range n {
			if i != j {
				sum += simMatrix[i][j]
				count++
			}
		}
		if count == 0 {
			continue // single sentence — can't compute outlier score
		}
		meanSim := sum / float64(count)
		outlierScore := 1.0 - meanSim

		if outlierScore > threshold {
			excerpt := sentences[i]
			excerpt = truncateExcerpt(excerpt, 80, "…")
			findings = append(findings, Finding{
				Category: CategoryOutlier,
				Sentence: i,
				Offset:   -1,
				Score:    outlierScore,
				Pattern:  "cosine-outlier",
				Excerpt:  excerpt,
			})
		}
	}
	return findings
}

// --- PII detection ---

// CategoryPII classifies findings from DetectPII and SanitizePII.
const CategoryPII Category = "pii"

// piiDef pairs a human-readable name with a compiled regex for PII pattern matching.
// validate, when set, filters matches: a match is kept only if validate returns true
// (used to apply a Luhn checksum to credit-card candidates). multiline patterns are
// scanned against the full text rather than line-by-line, for secrets that span lines.
type piiDef struct {
	name      string
	re        *regexp.Regexp
	validate  func(string) bool
	multiline bool
}

// piiPatterns is the canonical set of PII and secret patterns.
// Ordered from most-specific (AKIA, AIza) to least-specific (generic digit runs).
var piiPatterns = []piiDef{
	// API Keys — specific prefixes first to avoid ambiguous matches
	{
		name: "api-key",
		re:   regexp.MustCompile(`Bearer\s+[A-Za-z0-9._~+/\-]+=*`),
	},
	{
		// Allow _ and - so modern OpenAI keys (sk-proj-…) match, not just legacy sk-.
		name: "api-key",
		re:   regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{20,}\b`),
	},
	{
		name: "api-key",
		re:   regexp.MustCompile(`\bAIza[A-Za-z0-9_\-]{35,}\b`),
	},
	{
		name: "api-key",
		re:   regexp.MustCompile(`\bAKIA[A-Za-z0-9]{16,}\b`),
	},
	// GitHub tokens — classic (ghp_/gho_/ghs_/ghu_/ghr_) and fine-grained (github_pat_)
	{
		name: "github-token",
		re:   regexp.MustCompile(`\bgh[opsur]_[A-Za-z0-9]{36,}\b`),
	},
	{
		name: "github-token",
		re:   regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{50,}\b`),
	},
	// Slack tokens — distinct xox[abprs]-/xapp- prefix, very low false-positive surface.
	{
		name: "slack-token",
		re:   regexp.MustCompile(`\b(?:xox[abprs]|xapp)-[A-Za-z0-9-]{10,}\b`),
	},
	// Private keys — PEM blocks span multiple lines (scanned against full text).
	{
		name:      "private-key",
		re:        regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`),
		multiline: true,
	},
	// JWT — three base64url segments separated by dots, minimum 10 chars per segment
	{
		name: "jwt",
		re:   regexp.MustCompile(`\b[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b`),
	},
	// Email addresses
	{
		name: "email",
		re:   regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	},
	// US Social Security numbers
	{
		name: "ssn",
		re:   regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	},
	// Credit card numbers — 13-16 digit runs that pass the Luhn checksum.
	{
		name:     "credit-card",
		re:       regexp.MustCompile(`\b(?:\d[ \-]?){12,15}\d\b`),
		validate: luhnValid,
	},
}

// luhnValid reports whether the digits in s satisfy the Luhn checksum and form a
// plausible card length (13-16 digits). Non-digit characters (spaces, hyphens)
// are ignored. Used to reject digit runs that merely look card-shaped.
func luhnValid(s string) bool {
	var digits []int
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 13 || len(digits) > 16 {
		return false
	}
	sum := 0
	parity := len(digits) % 2
	for i, d := range digits {
		if i%2 == parity {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

// DetectPII scans text for PII and secret patterns.
// Returns one Finding per match. Text is not modified.
// Excerpts for long values are truncated to first 12 chars + "..." in the Excerpt field.
func DetectPII(text string) []Finding {
	var findings []Finding
	lines := strings.Split(text, "\n")
	for lineIdx, line := range lines {
		for _, p := range piiPatterns {
			if p.multiline {
				continue // handled in the full-text pass below
			}
			matches := p.re.FindAllStringIndex(line, -1)
			for _, loc := range matches {
				start, end := loc[0], loc[1]
				raw := line[start:end]
				if p.validate != nil && !p.validate(raw) {
					continue
				}
				findings = append(findings, Finding{
					Category: CategoryPII,
					Sentence: lineIdx + 1, // 1-based line number
					Offset:   start,
					Score:    0.95,
					Pattern:  p.name,
					Excerpt:  excerptOf(raw),
				})
			}
		}
	}
	// Multiline secrets (PEM blocks) span lines, so scan the full text and derive
	// the line number from the match offset.
	for _, p := range piiPatterns {
		if !p.multiline {
			continue
		}
		for _, loc := range p.re.FindAllStringIndex(text, -1) {
			raw := text[loc[0]:loc[1]]
			if p.validate != nil && !p.validate(raw) {
				continue
			}
			findings = append(findings, Finding{
				Category: CategoryPII,
				Sentence: strings.Count(text[:loc[0]], "\n") + 1,
				Offset:   loc[0] - strings.LastIndex(text[:loc[0]], "\n") - 1,
				Score:    0.95,
				Pattern:  p.name,
				Excerpt:  excerptOf(raw),
			})
		}
	}
	return findings
}

// truncateExcerpt returns s limited to maxRunes runes, appending ellipsis when
// truncated. It cuts on a rune boundary so the result is always valid UTF-8.
func truncateExcerpt(s string, maxRunes int, ellipsis string) string {
	count := 0
	for i := range s {
		if count == maxRunes {
			return s[:i] + ellipsis
		}
		count++
	}
	return s
}

// excerptOf returns a short, display-safe excerpt of a matched value: the first
// 12 runes followed by "..." when longer.
func excerptOf(raw string) string {
	return truncateExcerpt(raw, 12, "...")
}

// piiSpan is a matched PII region as an absolute byte range into the source text.
type piiSpan struct {
	start, end int
	name       string
	raw        string
}

// scanPII finds every PII match across all patterns as absolute byte spans,
// applying each pattern's validator. Patterns are scanned over the full text;
// none anchor or use newline separators, so this matches the per-line scan in
// DetectPII while also covering multi-line secrets (PEM blocks).
func scanPII(text string) []piiSpan {
	var spans []piiSpan
	for _, p := range piiPatterns {
		for _, loc := range p.re.FindAllStringIndex(text, -1) {
			raw := text[loc[0]:loc[1]]
			if p.validate != nil && !p.validate(raw) {
				continue
			}
			spans = append(spans, piiSpan{loc[0], loc[1], p.name, raw})
		}
	}
	// High-entropy base64 secrets (AWS secret keys, random API tokens, opaque
	// blobs) have no fixed prefix, so redact the spans the encoding detector
	// already flags — otherwise --sanitize-pii would leak them. Overlap
	// resolution in SanitizePII lets a more-specific pattern (jwt, api-key, …)
	// win when both match the same region.
	for _, r := range highEntropyBase64(text) {
		spans = append(spans, piiSpan{r[0], r[1], "secret", text[r[0]:r[1]]})
	}
	return spans
}

// SanitizePII replaces PII matches in text with [REDACTED:<type>] placeholders.
// It scans once, resolves overlapping matches (earliest start wins; longest at
// the same start), then redacts in a single pass — so the returned findings
// correspond exactly to the redactions applied. The original text is never
// stored or logged; only the redacted form is returned.
func SanitizePII(text string) (string, []Finding) {
	spans := scanPII(text)
	if len(spans) == 0 {
		return text, nil
	}
	// Earliest start first; longer span first on ties.
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end > spans[j].end
	})

	var (
		redacted strings.Builder
		findings []Finding
		prev     int
	)
	for _, s := range spans {
		if s.start < prev {
			continue // overlaps an already-redacted span — drop it
		}
		redacted.WriteString(text[prev:s.start])
		redacted.WriteString("[REDACTED:")
		redacted.WriteString(s.name)
		redacted.WriteString("]")
		prev = s.end
		findings = append(findings, Finding{
			Category: CategoryPII,
			Sentence: strings.Count(text[:s.start], "\n") + 1,
			Offset:   s.start,
			Score:    0.95,
			Pattern:  s.name,
			Excerpt:  excerptOf(s.raw),
		})
	}
	redacted.WriteString(text[prev:])
	return redacted.String(), findings
}

// --- Combined analysis ---

// Analyze runs pattern, encoding, and confusable-homoglyph detectors against text
// and returns a combined Report. Outlier detection requires a precomputed similarity
// matrix and is handled separately (DetectOutliers) because it requires the LexRank matrix.
// Layers selects which detection layers run. The zero value runs nothing; use
// DefaultLayers or HookLayers rather than constructing one field by field.
type Layers struct {
	Patterns    bool
	Encoding    bool
	Confusables bool
	Decode      bool
	Role        bool
	Obfuscated  bool
	Exfil       bool
	Positional  bool
	Script      bool
}

// DefaultLayers enables every layer. This is the CLI default: a developer running
// tldt over a document explicitly is paying attention to the output, so breadth is
// worth more than precision.
func DefaultLayers() Layers {
	return Layers{
		Patterns: true, Encoding: true, Confusables: true, Decode: true,
		Role: true, Obfuscated: true, Exfil: true, Positional: true, Script: true,
	}
}

// HookLayers enables only the high-precision layers.
//
// Hook mode fires on every agent prompt, so a false positive injects an advisory
// into a real user's turn again and again. That cost is paid constantly while the
// false-negative cost is paid rarely, and a noisy advisory is one the user learns
// to ignore — which would cost the precise layers their value too. The weak-prior
// layers (positional placement, script mismatch, obfuscation folding) and the
// flag-gated exfil layer stay off unless explicitly enabled.
func HookLayers() Layers {
	return Layers{
		Patterns: true, Encoding: true, Confusables: true, Decode: true, Role: true,
	}
}

// Analyze runs every detection layer over text. Equivalent to
// AnalyzeWith(text, DefaultLayers()).
func Analyze(text string) Report {
	return AnalyzeWith(text, DefaultLayers())
}

// AnalyzeWith runs the selected detection layers over text.
//
// Sentence-scoped layers are not run here: the outlier pass needs a similarity
// matrix and script mismatch needs a sentence split, both of which live above this
// package. See tldt.Detect.
func AnalyzeWith(text string, layers Layers) Report {
	var allFindings []Finding

	// The ASCII-lowercased copy is shared across every layer that needs it.
	// Computing it once rather than per layer removes several 256 KB allocations
	// per call — measured ~44 ms of GC churn at 256 KB with all layers enabled.
	lower := asciiLower(text)

	if layers.Patterns {
		allFindings = append(allFindings, detectPatternsWithLower(text, lower, "")...)
	}
	if layers.Encoding {
		allFindings = append(allFindings, DetectEncoding(text)...)
	}
	if layers.Confusables {
		allFindings = append(allFindings, DetectConfusables(text)...)
	}
	if layers.Decode {
		allFindings = append(allFindings, DetectDecoded(text)...)
	}
	if layers.Role {
		allFindings = append(allFindings, detectRoleMarkersIn(text, lower)...)
	}
	if layers.Obfuscated {
		allFindings = append(allFindings, detectObfuscatedIn(text, lower)...)
	}
	if layers.Exfil {
		allFindings = append(allFindings, DetectExfil(text)...)
	}
	if layers.Positional {
		allFindings = append(allFindings, detectPositionalIn(text, lower)...)
	}

	report := Report{Findings: allFindings}
	report.Recompute()
	return report
}
