// OpenAPI Client Example
//
// This example demonstrates a real-world use case: fetching OpenAPI/Swagger
// documentation from a URL, processing it through tldt's security pipeline,
// and generating a condensed summary suitable for LLM context.
//
// This pattern is useful when:
//   - You need to understand a new API quickly
//   - API documentation is too long for your LLM's context window
//   - You want to sanitize documentation before processing
//
// Usage:
//
//	go run main.go -url https://petstore.swagger.io/v2/swagger.json -sentences 5
//
//	go run main.go -url https://petstore.swagger.io/v2/swagger.json \
//	    -sanitize -detect-pii -sanitize-pii -sentences 7
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gleicon/tldt/pkg/tldt"
)

// OpenAPI represents a simplified OpenAPI/Swagger document structure
type OpenAPI struct {
	Swagger     string                 `json:"swagger"`
	OpenAPI     string                 `json:"openapi"`
	Info        Info                   `json:"info"`
	Host        string                 `json:"host"`
	BasePath    string                 `json:"basePath"`
	Paths       map[string]interface{} `json:"paths"`
	Definitions map[string]interface{} `json:"definitions"`
}

type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

func main() {
	url := flag.String("url", "https://petstore.swagger.io/v2/swagger.json", "OpenAPI/Swagger JSON URL")
	sanitize := flag.Bool("sanitize", true, "Strip invisible Unicode")
	detectPII := flag.Bool("detect-pii", true, "Detect PII in documentation")
	sanitizePII := flag.Bool("sanitize-pii", false, "Redact PII (use if docs may contain secrets)")
	_ = flag.Bool("detect-injection", true, "Detect prompt injection patterns")
	sentences := flag.Int("sentences", 5, "Number of summary sentences")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP timeout")
	outputJSON := flag.Bool("json", false, "Output as JSON")
	flag.Parse()

	fmt.Printf("Fetching OpenAPI documentation from: %s\n", *url)
	fmt.Println()

	// Fetch the OpenAPI document
	fetchResult, err := tldt.Fetch(*url, tldt.FetchOptions{
		Timeout:  *timeout,
		MaxBytes: 10 * 1024 * 1024, // 10MB max
	})
	if err != nil {
		if errors.Is(err, tldt.ErrSSRFBlocked) {
			log.Fatalf("SSRF protection triggered: %v", err)
		}
		if errors.Is(err, tldt.ErrRedirectLimit) {
			log.Fatalf("Too many redirects: %v", err)
		}
		log.Fatalf("Failed to fetch URL: %v", err)
	}

	fmt.Printf("Fetched: %d bytes\n", len(fetchResult.Text))
	fmt.Printf("Content-Type: %s\n", fetchResult.ContentType)
	fmt.Printf("Status: %d\n", fetchResult.StatusCode)
	if fetchResult.FinalURL != *url {
		fmt.Printf("Final URL: %s\n", fetchResult.FinalURL)
	}
	fmt.Println()

	// Parse as OpenAPI to extract metadata
	var apiDoc OpenAPI
	if err := json.Unmarshal([]byte(fetchResult.Text), &apiDoc); err == nil {
		fmt.Printf("API Title: %s\n", apiDoc.Info.Title)
		fmt.Printf("Version: %s\n", apiDoc.Info.Version)
		if apiDoc.Host != "" {
			fmt.Printf("Host: %s\n", apiDoc.Host)
		}
		if apiDoc.BasePath != "" {
			fmt.Printf("Base Path: %s\n", apiDoc.BasePath)
		}
		fmt.Printf("Endpoints: %d\n", len(apiDoc.Paths))
		fmt.Println()
	}

	// Process through tldt pipeline
	fmt.Println("Processing through tldt pipeline...")
	result, err := tldt.Pipeline(fetchResult.Text, tldt.PipelineOptions{
		Sanitize:    *sanitize,
		DetectPII:   *detectPII,
		SanitizePII: *sanitizePII,
		Detect: tldt.DetectOptions{
			OutlierThreshold: 0.85,
		},
		Summarize: tldt.SummarizeOptions{
			Algorithm: "ensemble",
			Sentences: *sentences,
		},
	})
	if err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}

	if *outputJSON {
		// Output structured JSON
		output := struct {
			APITitle      string              `json:"api_title,omitempty"`
			APIVersion    string              `json:"api_version,omitempty"`
			OriginalSize  int                 `json:"original_size_bytes"`
			SummaryTokens int                 `json:"summary_tokens"`
			Reduction     int                 `json:"reduction_percent"`
			Warnings      []string            `json:"warnings,omitempty"`
			PIIFindings   []tldt.PIIFinding   `json:"pii_findings,omitempty"`
			Redactions    int                 `json:"redactions"`
			Summary       string              `json:"summary"`
		}{
			APITitle:      apiDoc.Info.Title,
			APIVersion:    apiDoc.Info.Version,
			OriginalSize:  len(fetchResult.Text),
			SummaryTokens: result.TokensOut,
			Reduction:     result.Reduction,
			Warnings:      result.Warnings,
			PIIFindings:   result.PIIFindings,
			Redactions:    result.Redactions,
			Summary:       result.Summary,
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(output)
	} else {
		// Human-readable output
		fmt.Printf("Original: ~%d tokens\n", result.TokensIn)
		fmt.Printf("Summary: ~%d tokens (%d%% reduction)\n", result.TokensOut, result.Reduction)
		fmt.Println()

		if len(result.Warnings) > 0 {
			fmt.Println("=== Security Warnings ===")
			for _, w := range result.Warnings {
				fmt.Printf("- %s\n", w)
			}
			fmt.Println()
		}

		if len(result.PIIFindings) > 0 {
			fmt.Println("=== PII Detected ===")
			for _, f := range result.PIIFindings {
				fmt.Printf("- [%s] Line %d: %s\n", f.Pattern, f.Line, f.Excerpt)
			}
			fmt.Println()
		}

		if result.Redactions > 0 {
			fmt.Printf("=== Redactions: %d ===\n", result.Redactions)
			fmt.Println()
		}

		fmt.Println("=== API Summary (Ready for LLM Context) ===")
		fmt.Println(result.Summary)
	}
}
