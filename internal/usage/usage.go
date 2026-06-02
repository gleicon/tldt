// Package usage appends counts-only summarization records to ~/.tldt/usage.jsonl.
// Records hold token counts and a timestamp only — never source or prompt content.
package usage

import (
	"encoding/json"
	"errors"
	"fmt"
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
