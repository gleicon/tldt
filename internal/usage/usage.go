// Package usage appends counts-only summarization records to ~/.tldt/usage.jsonl.
// Records hold token counts and a timestamp only — never source or prompt content.
package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Record is one usage-log line. Field order matches the on-disk JSON schema
// {ts, in, out, saved}. ts is RFC3339; in/out/saved are estimated token counts.
type Record struct {
	TS    string `json:"ts"`
	In    int    `json:"in"`
	Out   int    `json:"out"`
	Saved int    `json:"saved"`
}

// Path returns the usage-log path (~/.tldt/usage.jsonl).
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tldt", "usage.jsonl"), nil
}

// Append writes rec as one newline-terminated JSON line to path, creating the
// parent directory if needed. The record is emitted in a single O_APPEND write
// so concurrent appends from parallel processes stay atomic (NFR-2).
func Append(path string, rec Record) error {
	if path == "" {
		return errors.New("usage: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("usage: create dir: %w", err)
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("usage: marshal: %w", err)
	}
	line = append(line, '\n')
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("usage: open: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("usage: write: %w", err)
	}
	return nil
}

// Aggregate holds totals computed across all usage-log records. Percent is the
// reduction (Saved/In * 100), or 0 when In is 0.
type Aggregate struct {
	Count   int     `json:"count"`
	In      int     `json:"in"`
	Out     int     `json:"out"`
	Saved   int     `json:"saved"`
	Percent float64 `json:"percent"`
}

// Read parses path and returns aggregate totals. A missing log is treated as
// empty and yields a zero Aggregate with no error (FR-16). Malformed lines —
// e.g. a record half-written by a crashed process — are skipped, not fatal.
func Read(path string) (Aggregate, error) {
	if path == "" {
		return Aggregate{}, errors.New("usage: empty path")
	}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Aggregate{}, nil
	}
	if err != nil {
		return Aggregate{}, fmt.Errorf("usage: open: %w", err)
	}
	defer func() { _ = f.Close() }()

	var agg Aggregate
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for s.Scan() {
		line := bytes.TrimSpace(s.Bytes())
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		agg.Count++
		agg.In += r.In
		agg.Out += r.Out
		agg.Saved += r.Saved
	}
	if err := s.Err(); err != nil {
		return Aggregate{}, fmt.Errorf("usage: read: %w", err)
	}
	if agg.In > 0 {
		agg.Percent = float64(agg.Saved) / float64(agg.In) * 100
	}
	return agg, nil
}

// Reset clears the usage log by removing it. A missing log is not an error.
func Reset(path string) error {
	if path == "" {
		return errors.New("usage: empty path")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("usage: reset: %w", err)
	}
	return nil
}
