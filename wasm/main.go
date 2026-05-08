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
	Error      string           `json:"error,omitempty"`
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
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
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
	format := config.Get("format").String()
	if format == "" {
		format = "text"
	}
	verbose := config.Get("verbose").Bool()

	result := runSummarize(text, algorithm, sentences, sanitize, detectInjection, detectPII, format, verbose)
	return toJSValue(result)
}

func toJSValue(result JSResult) js.Value {
	jsonBytes, _ := json.Marshal(result)
	return js.Global().Get("JSON").Call("parse", string(jsonBytes))
}

func runSummarize(text, algorithm string, sentences int, sanitize, detectInjection, detectPII bool, format string, verbose bool) JSResult {
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
			severity := "medium"
			if finding.Category == "pattern" {
				severity = "high"
			}
			result.Detections = append(result.Detections, DetectionAlert{
				Type:     "injection",
				Severity: severity,
				Message:  fmt.Sprintf("%s: %s", finding.Category, finding.Excerpt),
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
