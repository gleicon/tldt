// Basic example demonstrates simple text summarization using tldt.
//
// Usage:
//
//	go run main.go "Your long text here that needs summarization..."
//
// Or with a file:
//
//	go run main.go -f article.txt
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gleicon/tldt/pkg/tldt"
)

func main() {
	fileFlag := flag.String("f", "", "Read text from file")
	algorithm := flag.String("algorithm", "lexrank", "Algorithm: lexrank|textrank|graph|ensemble")
	sentences := flag.Int("sentences", 3, "Number of sentences in summary")
	flag.Parse()

	var text string
	var err error

	if *fileFlag != "" {
		data, err := os.ReadFile(*fileFlag)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}
		text = string(data)
	} else if len(flag.Args()) > 0 {
		text = strings.Join(flag.Args(), " ")
	} else {
		// Default demo text
		text = `The Go programming language is an open-source project that makes programmers more productive. 
Go is expressive, concise, clean, and efficient. Its concurrency mechanisms make it easy to write programs 
that get the most out of multicore and networked machines, while its novel type system enables flexible 
and modular program construction. Go compiles quickly to machine code yet has the convenience of 
garbage collection and the power of run-time reflection. It's a fast, statically typed, compiled 
language that feels like a dynamically typed, interpreted language. Go was designed at Google in 2007 
to improve programming productivity in an era of multicore, networked machines and large codebases. 
The designers were primarily motivated by their shared dislike of C++. The language was publicly 
announced in November 2009 and version 1.0 was released in March 2012. Go is widely used in production 
at Google and in many other organizations and open-source projects.`
	}

	result, err := tldt.Summarize(text, tldt.SummarizeOptions{
		Algorithm: *algorithm,
		Sentences: *sentences,
	})
	if err != nil {
		log.Fatalf("Summarization failed: %v", err)
	}

	fmt.Printf("Algorithm: %s\n", *algorithm)
	fmt.Printf("Original: ~%d tokens\n", result.TokensIn)
	fmt.Printf("Summary: ~%d tokens (%d%% reduction)\n", result.TokensOut, result.Reduction)
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Println(result.Summary)
}
