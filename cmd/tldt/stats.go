package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	usagelog "github.com/gleicon/tldt/internal/usage"
)

// runStats handles the `tldt stats` subcommand: print aggregate token-savings
// totals from ~/.tldt/usage.jsonl, emit them as JSON, or clear the log.
func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit aggregate totals as JSON")
	reset := fs.Bool("reset", false, "clear the usage log")
	_ = fs.Parse(args)

	path, err := usagelog.Path()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stats:", err)
		os.Exit(1)
	}

	if *reset {
		if err := usagelog.Reset(path); err != nil {
			fmt.Fprintln(os.Stderr, "stats:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "usage log cleared")
		return
	}

	agg, err := usagelog.Read(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stats:", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(agg); err != nil {
			fmt.Fprintln(os.Stderr, "stats:", err)
			os.Exit(1)
		}
		return
	}

	fmt.Printf("invocations:   %d\n", agg.Count)
	fmt.Printf("tokens in:     %d\n", agg.In)
	fmt.Printf("tokens out:    %d\n", agg.Out)
	fmt.Printf("tokens saved:  %d\n", agg.Saved)
	fmt.Printf("reduction:     %.1f%%\n", agg.Percent)
}
