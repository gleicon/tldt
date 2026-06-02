package usage

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppend_CreatesDirAndWritesLine(t *testing.T) {
	// Path points into a not-yet-existing subdir to prove Append creates it.
	path := filepath.Join(t.TempDir(), "sub", "usage.jsonl")

	rec := Record{TS: "2026-06-02T10:00:00Z", In: 1000, Out: 250, Saved: 750}
	if err := Append(path, rec); err != nil {
		t.Fatalf("Append: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Errorf("line not newline-terminated: %q", data)
	}

	var got Record
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != rec {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, rec)
	}
	// Schema field names must match the spec exactly.
	if !strings.Contains(string(data), `"ts"`) ||
		!strings.Contains(string(data), `"in"`) ||
		!strings.Contains(string(data), `"out"`) ||
		!strings.Contains(string(data), `"saved"`) {
		t.Errorf("missing expected field name in %q", data)
	}
}

func TestAppend_MultipleRecordsOnePerLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")

	recs := []Record{
		{TS: "2026-06-02T10:00:00Z", In: 100, Out: 40, Saved: 60},
		{TS: "2026-06-02T10:01:00Z", In: 200, Out: 50, Saved: 150},
		{TS: "2026-06-02T10:02:00Z", In: 300, Out: 90, Saved: 210},
	}
	for _, r := range recs {
		if err := Append(path, r); err != nil {
			t.Fatalf("Append(%+v): %v", r, err)
		}
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()

	var n int
	s := bufio.NewScanner(f)
	for s.Scan() {
		var got Record
		if err := json.Unmarshal(s.Bytes(), &got); err != nil {
			t.Fatalf("line %d unmarshal: %v", n, err)
		}
		if got != recs[n] {
			t.Errorf("line %d: got %+v, want %+v", n, got, recs[n])
		}
		n++
	}
	if err := s.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if n != len(recs) {
		t.Errorf("line count = %d, want %d", n, len(recs))
	}
}

func TestAppend_EmptyPath(t *testing.T) {
	if err := Append("", Record{}); err == nil {
		t.Error("Append(\"\"): want error, got nil")
	}
}
