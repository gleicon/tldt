package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gleicon/tldt/internal/summarizer"
)

const defaultSentences = 5

func main() {
	filePath := flag.String("f", "", "input file path")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: tldt [-f file] [text...]")
		fmt.Fprintln(os.Stderr, "       cat file.txt | tldt")
		flag.PrintDefaults()
		os.Exit(1)
	}
	flag.Parse()

	text, err := resolveInput(flag.Args(), *filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sentences, err := summarizer.Summarize(text, defaultSentences)
	if err != nil {
		fmt.Fprintln(os.Stderr, "summarization failed:", err)
		os.Exit(1)
	}

	fmt.Println(strings.Join(sentences, " "))
}

// resolveInput determines the text source using explicit precedence:
//  1. stdin pipe (stdin is not a TTY)
//  2. -f file flag
//  3. positional arguments joined with spaces
//  4. error (no input provided)
func resolveInput(args []string, filePath string) (string, error) {
	// 1. stdin pipe: check if stdin is a pipe/redirect (not a TTY)
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	}
	// 2. -f flag
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("reading file %q: %w", filePath, err)
		}
		return string(data), nil
	}
	// 3. positional argument
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	return "", fmt.Errorf("no input: provide text via stdin, -f file, or positional argument")
}
