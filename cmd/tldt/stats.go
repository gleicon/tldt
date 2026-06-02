package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	usagelog "github.com/gleicon/tldt/internal/usage"
)

// fail prints a stats error to stderr and exits non-zero.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "stats:", err)
	os.Exit(1)
}

// emitJSON writes v to stdout as indented JSON, exiting non-zero on error.
func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fail(err)
	}
}

// runStats handles the `tldt stats` subcommand: print aggregate token-savings
// totals from ~/.tldt/usage.jsonl, emit them as JSON, or clear the log.
func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit totals as JSON")
	reset := fs.Bool("reset", false, "clear the usage log")
	daily := fs.Bool("daily", false, "report a per-day breakdown")
	_ = fs.Parse(args)

	path, err := usagelog.Path()
	if err != nil {
		fail(err)
	}

	if *reset {
		if err := usagelog.Reset(path); err != nil {
			fail(err)
		}
		fmt.Fprintln(os.Stderr, "usage log cleared")
		return
	}

	if *daily {
		runStatsDaily(path, *jsonOut)
		return
	}

	agg, err := usagelog.Read(path)
	if err != nil {
		fail(err)
	}

	if *jsonOut {
		emitJSON(agg)
		return
	}

	fmt.Printf("invocations:   %d\n", agg.Count)
	fmt.Printf("tokens in:     %d\n", agg.In)
	fmt.Printf("tokens out:    %d\n", agg.Out)
	fmt.Printf("tokens saved:  %d\n", agg.Saved)
	fmt.Printf("reduction:     %.1f%%\n", agg.Percent)
}

// runStatsDaily prints the per-day breakdown as a table, or as JSON when jsonOut
// is set. An empty/missing log yields an empty JSON array or a header-only table.
func runStatsDaily(path string, jsonOut bool) {
	days, err := usagelog.ReadDaily(path)
	if err != nil {
		fail(err)
	}

	if jsonOut {
		if days == nil {
			days = []usagelog.DailyAggregate{}
		}
		emitJSON(days)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "date\tinvocations\tin\tout\tsaved\treduction")
	for _, d := range days {
		_, _ = fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%.1f%%\n", d.Date, d.Count, d.In, d.Out, d.Saved, d.Percent)
	}
	_ = w.Flush()
}
