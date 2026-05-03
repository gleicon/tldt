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
}

// Report aggregates all findings from a full analysis pass.
type Report struct {
	Findings        []Finding
	MaxScore        float64
	Suspicious      bool  // MaxScore > DefaultDetectionThreshold
	QuarantinedIdxs []int // sentence indices excluded by DetectOutliers with quarantine=true
}

// DefaultDetectionThreshold is the score above which a report is marked Suspicious.
const DefaultDetectionThreshold = 0.70

// DefaultOutlierThreshold is the outlier_score above which a sentence is flagged.
// outlier_score(i) = 1 - mean(sim[i][j] for j ≠ i).
// Higher = lower similarity to neighbors = more off-topic.
const DefaultOutlierThreshold = 0.85

// --- Pattern detection ---

// patternDef pairs a human-readable name with a compiled regex and confidence score.
type patternDef struct {
	name       string
	re         *regexp.Regexp
	confidence float64
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
	},
	{
		name:       "direct-override",
		re:         regexp.MustCompile(`(?i)disregard\s+(all\s+)?(the\s+)?(previous|prior|above|following)`),
		confidence: 0.90,
	},
	{
		name:       "direct-override",
		re:         regexp.MustCompile(`(?i)forget\s+(all\s+)?(previous|prior|above|your)\s+(instructions?|context|conversation)`),
		confidence: 0.90,
	},
	// Role injection
	{
		name:       "role-injection",
		re:         regexp.MustCompile(`(?i)you\s+are\s+now\s+(a|an)\s+\w+`),
		confidence: 0.80,
	},
	{
		name:       "role-injection",
		re:         regexp.MustCompile(`(?i)(act|behave|pretend|respond)\s+as\s+(if\s+)?(you\s+(are|were|have|had))`),
		confidence: 0.75,
	},
	{
		name:       "role-injection",
		re:         regexp.MustCompile(`(?i)your\s+(new\s+)?(role|persona|character|identity)\s+is`),
		confidence: 0.80,
	},
	// Delimiter injection (model-specific special tokens)
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`(?i)<\s*/?\s*(system|instructions?|prompt|context)\s*>`),
		confidence: 0.85,
	},
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`(?i)---+\s*(begin|end|start)\s+(system\s+)?prompt\s*---+`),
		confidence: 0.90,
	},
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`\[/?INST\]`),
		confidence: 0.85,
	},
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`\|im_(start|end)\|`),
		confidence: 0.90,
	},
	{
		name:       "delimiter-injection",
		re:         regexp.MustCompile(`(?i)###\s*(instruction|system|prompt|context|task)`),
		confidence: 0.80,
	},
	// Conversational role prefixes (context-dependent — moderate confidence)
	{
		name:       "role-prefix",
		re:         regexp.MustCompile(`(?m)^(System|Assistant|Human|User)\s*:\s*.{10,}`),
		confidence: 0.65,
	},
	// Jailbreak patterns
	{
		name:       "jailbreak",
		re:         regexp.MustCompile(`(?i)\bDAN\b.{0,30}(mode|enabled|activated|persona)`),
		confidence: 0.85,
	},
	{
		name:       "jailbreak",
		re:         regexp.MustCompile(`(?i)(developer|jailbreak|unrestricted|unfiltered)\s+mode`),
		confidence: 0.80,
	},
	{
		name:       "jailbreak",
		re:         regexp.MustCompile(`(?i)pretend\s+(you\s+have\s+no|there\s+are\s+no)\s+(restrictions?|guidelines?|rules?|limits?)`),
		confidence: 0.85,
	},
	// Exfiltration patterns
	{
		name:       "exfiltration",
		re:         regexp.MustCompile(`(?i)(repeat|print|output|reveal|show|display)\s+(everything|all(thing)?s?)?\s*(above|before|prior|from\s+the\s+(beginning|start))`),
		confidence: 0.80,
	},
	{
		name:       "exfiltration",
		re:         regexp.MustCompile(`(?i)(what\s+(are|were)\s+your\s+(instructions?|system\s+prompt|guidelines?))`),
		confidence: 0.75,
	},
	{
		name:       "exfiltration",
		re:         regexp.MustCompile(`(?i)(print|output|show|display|repeat|reveal)\s+your\s+(system\s+)?(prompt|instructions?)`),
		confidence: 0.85,
	},
}

// DetectPatterns scans text for known injection phrases.
// Returns one Finding per pattern match. Text is not modified.
func DetectPatterns(text string) []Finding {
	var findings []Finding
	for _, p := range injectionPatterns {
		matches := p.re.FindAllStringIndex(text, -1)
		for _, loc := range matches {
			start, end := loc[0], loc[1]
			excerpt := text[start:end]
			if len(excerpt) > 80 {
				excerpt = excerpt[:80] + "…"
			}
			findings = append(findings, Finding{
				Category: CategoryPattern,
				Sentence: -1,
				Offset:   start,
				Score:    p.confidence,
				Pattern:  p.name,
				Excerpt:  excerpt,
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

// DetectEncoding scans text for base64 payloads, hex-encoded data, and
// abnormally high control character density.
func DetectEncoding(text string) []Finding {
	var findings []Finding

	// Base64 detection: match → validate length divisible by 4 → check entropy → try decode
	for _, loc := range base64RE.FindAllStringIndex(text, -1) {
		candidate := text[loc[0]:loc[1]]
		// Base64 strings have length divisible by 4 (with padding) and high entropy
		padded := candidate
		for len(padded)%4 != 0 {
			padded += "="
		}
		_, err := base64.StdEncoding.DecodeString(padded)
		entropy := shannonEntropy(candidate)
		if err == nil && entropy > 4.5 {
			excerpt := candidate
			if len(excerpt) > 80 {
				excerpt = excerpt[:80] + "…"
			}
			findings = append(findings, Finding{
				Category: CategoryEncoding,
				Sentence: -1,
				Offset:   loc[0],
				Score:    0.75,
				Pattern:  "base64",
				Excerpt:  excerpt,
			})
		}
	}

	// Hex escape detection: \x41\x42... sequences
	for _, loc := range hexEscapeRE.FindAllStringIndex(text, -1) {
		excerpt := text[loc[0]:loc[1]]
		if len(excerpt) > 80 {
			excerpt = excerpt[:80] + "…"
		}
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
			if len(excerpt) > 80 {
				excerpt = excerpt[:80] + "…"
			}
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
	for i := 0; i < n; i++ {
		if len(simMatrix[i]) != n {
			continue
		}
		var sum float64
		count := 0
		for j := 0; j < n; j++ {
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
			if len(excerpt) > 80 {
				excerpt = excerpt[:80] + "…"
			}
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
type piiDef struct {
	name string
	re   *regexp.Regexp
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
		name: "api-key",
		re:   regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`),
	},
	{
		name: "api-key",
		re:   regexp.MustCompile(`\bAIza[A-Za-z0-9_\-]{35,}\b`),
	},
	{
		name: "api-key",
		re:   regexp.MustCompile(`\bAKIA[A-Za-z0-9]{16,}\b`),
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
	// Credit card numbers — 13-16 consecutive digits
	{
		name: "credit-card",
		re:   regexp.MustCompile(`\b(?:\d[ \-]?){12,15}\d\b`),
	},
}

// DetectPII scans text for PII and secret patterns.
// Returns one Finding per match. Text is not modified.
// Excerpts for long values are truncated to first 12 chars + "..." in the Excerpt field.
func DetectPII(text string) []Finding {
	var findings []Finding
	lines := strings.Split(text, "\n")
	for lineIdx, line := range lines {
		for _, p := range piiPatterns {
			matches := p.re.FindAllStringIndex(line, -1)
			for _, loc := range matches {
				start, end := loc[0], loc[1]
				raw := line[start:end]
				excerpt := raw
				if len(excerpt) > 12 {
					excerpt = excerpt[:12] + "..."
				}
				findings = append(findings, Finding{
					Category: CategoryPII,
					Sentence: lineIdx + 1, // 1-based line number
					Offset:   start,
					Score:    0.95,
					Pattern:  p.name,
					Excerpt:  excerpt,
				})
			}
		}
	}
	return findings
}

// SanitizePII replaces PII matches in text with [REDACTED:<type>] placeholders.
// Returns the redacted string and a []Finding slice in a single pass.
// The original text is never stored or logged — only the redacted form is returned.
// Replacement format: [REDACTED:email], [REDACTED:api-key], [REDACTED:jwt], [REDACTED:credit-card].
func SanitizePII(text string) (string, []Finding) {
	findings := DetectPII(text)
	if len(findings) == 0 {
		return text, nil
	}
	redacted := text
	for _, p := range piiPatterns {
		replacement := "[REDACTED:" + p.name + "]"
		redacted = p.re.ReplaceAllString(redacted, replacement)
	}
	return redacted, findings
}

// --- Combined analysis ---

// Analyze runs pattern, encoding, and confusable-homoglyph detectors against text
// and returns a combined Report. Outlier detection requires a precomputed similarity
// matrix and is handled separately (DetectOutliers) because it requires the LexRank matrix.
func Analyze(text string) Report {
	var allFindings []Finding

	allFindings = append(allFindings, DetectPatterns(text)...)
	allFindings = append(allFindings, DetectEncoding(text)...)
	allFindings = append(allFindings, DetectConfusables(text)...)

	var maxScore float64
	for _, f := range allFindings {
		if f.Score > maxScore {
			maxScore = f.Score
		}
	}

	return Report{
		Findings:   allFindings,
		MaxScore:   maxScore,
		Suspicious: maxScore > DefaultDetectionThreshold,
	}
}
