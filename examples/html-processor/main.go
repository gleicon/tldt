// HTML Processor Example
//
// This example demonstrates converting HTML content to Markdown and then
// summarizing it. Useful for processing web pages from curl or saved HTML files.
//
// Usage:
//
//	go run main.go -f article.html
//
//	cat article.html | go run main.go
//
//	curl -s https://example.com | go run main.go -url-mode
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/gleicon/tldt/pkg/tldt"
)

func main() {
	fileFlag := flag.String("f", "", "Read HTML from file")
	urlMode := flag.Bool("url-mode", false, "Expect URL input (fetch and process)")
	sentences := flag.Int("sentences", 5, "Number of summary sentences")
	extractContent := flag.Bool("extract", true, "Use readability to extract main content")
	includeTitle := flag.Bool("title", true, "Include article title")
	flag.Parse()

	var htmlContent string
	var err error

	if *urlMode {
		// Read URL from stdin or args
		var url string
		if len(flag.Args()) > 0 {
			url = flag.Args()[0]
		} else {
			data, _ := io.ReadAll(os.Stdin)
			url = string(data)
		}
		if url == "" {
			log.Fatal("No URL provided")
		}

		// Fetch the URL
		fmt.Printf("Fetching: %s\n", url)
		result, err := tldt.Fetch(url, tldt.FetchOptions{})
		if err != nil {
			log.Fatalf("Fetch failed: %v", err)
		}
		htmlContent = result.Text
	} else if *fileFlag != "" {
		// Read from file
		data, err := os.ReadFile(*fileFlag)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}
		htmlContent = string(data)
	} else {
		// Read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("Failed to read stdin: %v", err)
		}
		htmlContent = string(data)
	}

	if htmlContent == "" {
		log.Fatal("No HTML content provided")
	}

	fmt.Printf("Input HTML: %d bytes\n", len(htmlContent))

	// Convert HTML to Markdown
	markdown, err := tldt.ConvertHTML(htmlContent, tldt.HTMLConvertOptions{
		ExtractContent: *extractContent,
		IncludeTitle:   *includeTitle,
	})
	if err != nil {
		log.Fatalf("HTML conversion failed: %v", err)
	}

	fmt.Printf("Markdown output: %d bytes (%.1f%% reduction)\n",
		len(markdown),
		100.0*(1.0-float64(len(markdown))/float64(len(htmlContent))))
	fmt.Println()

	// Summarize the Markdown
	result, err := tldt.Summarize(markdown, tldt.SummarizeOptions{
		Algorithm: "ensemble",
		Sentences: *sentences,
	})
	if err != nil {
		log.Fatalf("Summarization failed: %v", err)
	}

	fmt.Printf("Summary (%d sentences):\n", *sentences)
	fmt.Println("---")
	fmt.Println(result.Summary)
	fmt.Println("---")
	fmt.Printf("\nOriginal: ~%d tokens\n", result.TokensIn)
	fmt.Printf("Summary: ~%d tokens (%d%% reduction)\n", result.TokensOut, result.Reduction)
}
