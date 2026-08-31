//go:build js && wasm

// Package main provides WASM entry point for browser demo.
// Exports summarize() function callable from JavaScript.
package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	tldt "github.com/gleicon/tldt/pkg/tldt"
)

// JSResult holds all output for JavaScript
type JSResult struct {
	Summary    string           `json:"summary"`
	RawOutput  string           `json:"rawOutput"`
	Metrics    *Metrics         `json:"metrics,omitempty"`
	Detections []DetectionAlert `json:"detections"`
	AI         *AIResult        `json:"ai,omitempty"`
	Error      string           `json:"error,omitempty"`
}

// AIResult holds the AI-generated-content detection summary for the demo UI.
type AIResult struct {
	Score   float64  `json:"score"`   // combined score, 0..1
	Verdict string   `json:"verdict"` // human-readable band
	Lang    string   `json:"lang"`    // wordlist language used
	Markers []string `json:"markers"` // single-word excess-vocabulary hits
	Phrases []string `json:"phrases"` // multi-word phrase/template tells
}

type Metrics struct {
	InputTokens    int     `json:"inputTokens"`
	OutputTokens   int     `json:"outputTokens"`
	TokensSaved    int     `json:"tokensSaved"`
	SavingsPercent float64 `json:"savingsPercent"`
	SentenceCount  int     `json:"sentenceCount"`
	Algorithm      string  `json:"algorithm"`
}

type DetectionAlert struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Provenance string `json:"provenance,omitempty"` // encoding chain for a decoded payload, e.g. "base64>hex"
}

func main() {
	c := make(chan struct{}, 0)
	js.Global().Set("tldtSummarize", js.FuncOf(summarizeWrapper))
	<-c
}

func summarizeWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return toJSValue(JSResult{Error: "missing config argument"})
	}

	config := args[0]
	text := config.Get("text").String()
	algorithm := config.Get("algorithm").String()
	if algorithm == "" {
		algorithm = "lexrank"
	}
	sentences := config.Get("sentences").Int()
	if sentences == 0 {
		sentences = 5
	}
	sanitize := config.Get("sanitize").Bool()
	detectInjection := config.Get("detectInjection").Bool()
	detectPII := config.Get("detectPII").Bool()
	detectAI := config.Get("detectAI").Bool()
	lang := config.Get("lang").String()
	if lang == "" {
		lang = "en"
	}
	format := config.Get("format").String()
	if format == "" {
		format = "text"
	}
	verbose := config.Get("verbose").Bool()

	result := runSummarize(text, algorithm, sentences, sanitize, detectInjection, detectPII, detectAI, lang, format, verbose)
	return toJSValue(result)
}

func toJSValue(result JSResult) js.Value {
	jsonBytes, _ := json.Marshal(result)
	return js.Global().Get("JSON").Call("parse", string(jsonBytes))
}

func runSummarize(text, algorithm string, sentences int, sanitize, detectInjection, detectPII, detectAI bool, lang, format string, verbose bool) JSResult {
	result := JSResult{
		Detections: []DetectionAlert{},
	}

	if text == "" {
		result.Error = "no input text provided"
		return result
	}

	processedText := text

	// Apply sanitization if requested
	if sanitize {
		cleaned, report, err := tldt.Sanitize(text)
		if err != nil {
			result.Error = fmt.Sprintf("sanitize error: %v", err)
			return result
		}
		processedText = cleaned
		if report.RemovedCount > 0 {
			result.Detections = append(result.Detections, DetectionAlert{
				Type:     "sanitized",
				Severity: "low",
				Message:  fmt.Sprintf("%d invisible Unicode chars removed", report.RemovedCount),
			})
		}
	}

	// Run injection detection if requested
	if detectInjection {
		detectOpts := tldt.DetectOptions{}
		detectResult, err := tldt.Detect(processedText, detectOpts)
		if err != nil {
			result.Error = fmt.Sprintf("detect error: %v", err)
			return result
		}
		for _, finding := range detectResult.Report.Findings {
			// Outlier findings are a summarization signal on a different scale,
			// not an injection threat — keep them out of the demo alert list.
			if finding.Category == tldt.CategoryOutlier {
				continue
			}
			result.Detections = append(result.Detections, DetectionAlert{
				Type:       "injection",
				Severity:   severityFor(finding.Category, finding.Score),
				Message:    fmt.Sprintf("%s: %s", categoryLabel(finding.Category), finding.Excerpt),
				Provenance: finding.Provenance,
			})
		}
		if detectResult.Report.Suspicious {
			result.Detections = append(result.Detections, DetectionAlert{
				Type:     "injection",
				Severity: "high",
				Message:  "input flagged as suspicious",
			})
		}
	}

	// Run PII detection if requested
	if detectPII {
		findings := tldt.DetectPII(processedText)
		for _, f := range findings {
			result.Detections = append(result.Detections, DetectionAlert{
				Type:     "pii",
				Severity: "high",
				Message:  fmt.Sprintf("%s: %s (line %d)", f.Pattern, f.Excerpt, f.Line),
			})
		}
	}

	// Run AI-generated-content detection if requested
	if detectAI {
		if r, err := tldt.DetectAI(processedText, lang, ""); err == nil {
			result.AI = &AIResult{
				Score:   r.CombinedScore(),
				Verdict: r.Verdict(),
				Lang:    r.Lang,
				Markers: r.Markers,
				Phrases: r.Phrases,
			}
		}
	}

	// Summarize
	summarizeOpts := tldt.SummarizeOptions{
		Algorithm: algorithm,
		Sentences: sentences,
	}
	sumResult, err := tldt.Summarize(processedText, summarizeOpts)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Summary = sumResult.Summary
	result.RawOutput = formatOutput(sumResult.Summary, format)

	// Calculate metrics
	if verbose {
		result.Metrics = &Metrics{
			InputTokens:    sumResult.TokensIn,
			OutputTokens:   sumResult.TokensOut,
			TokensSaved:    sumResult.TokensIn - sumResult.TokensOut,
			SavingsPercent: float64(sumResult.Reduction),
			SentenceCount:  sentences,
			Algorithm:      algorithm,
		}
	}

	return result
}

// severityFor maps a finding category and score to a demo severity band.
func severityFor(category tldt.Category, score float64) string {
	switch category {
	case tldt.CategoryPattern, tldt.CategoryRole, tldt.CategoryEncoding:
		if score >= 0.85 {
			return "high"
		}
		return "medium"
	case tldt.CategoryExfil:
		return "high"
	default:
		return "medium"
	}
}

// categoryLabel gives the demo a human-readable name for each finding category.
func categoryLabel(category tldt.Category) string {
	switch category {
	case tldt.CategoryPattern:
		return "injection pattern"
	case tldt.CategoryEncoding:
		return "encoded payload"
	case tldt.CategoryRole:
		return "role marker"
	case tldt.CategoryObfuscated:
		return "obfuscated phrase"
	case tldt.CategoryExfil:
		return "exfiltration link"
	case tldt.CategoryPositional:
		return "positional signal"
	case tldt.CategoryScript:
		return "script mismatch"
	default:
		return string(category)
	}
}

func formatOutput(summary string, format string) string {
	sentences := []string{}
	// Split summary into sentences for structured formats
	// This is a simple split; the actual sentences are joined in the result
	if summary != "" {
		// Use basic split - the summary is already processed
		sentences = append(sentences, summary)
	}

	switch format {
	case "json":
		data := map[string]interface{}{"summary": sentences}
		b, _ := json.MarshalIndent(data, "", "  ")
		return string(b)
	case "markdown":
		result := "## Summary\n\n"
		// For markdown, we need individual sentences
		// The summary is already one string, so we present it as is
		result += summary + "\n"
		return result
	default:
		return summary
	}
}
