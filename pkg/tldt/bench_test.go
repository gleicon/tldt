package tldt

import (
	"fmt"
	"os"
	"testing"
)

// benchCorpus returns real prose repeated to exactly size bytes.
func benchCorpus(tb testing.TB, size int) string {
	b, err := os.ReadFile("../../test-data/longform_3000.txt")
	if err != nil {
		tb.Skipf("corpus unavailable: %v", err)
	}
	s := string(b)
	for len(s) < size {
		s += s
	}
	return s[:size]
}

// BenchmarkDetect measures the whole detection pipeline, outlier pass included.
// NFR-1 budgets 150 ms at 256 KB; NFR-2 budgets 15 ms at 16 KB for hook mode.
func BenchmarkDetect(b *testing.B) {
	for _, sz := range []int{2 * 1024, 8 * 1024, 16 * 1024, 32 * 1024, 64 * 1024, 256 * 1024} {
		t := benchCorpus(b, sz)
		b.Run(fmt.Sprintf("%dK", sz/1024), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := Detect(t, DetectOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDetectHook measures the default/hook detection profile — the layer set
// that runs on every agent prompt — against NFR-1 (150ms at 256KB) and NFR-2
// (15ms at 16KB).
func BenchmarkDetectHook(b *testing.B) {
	hook := HookLayers()
	for _, sz := range []int{16 * 1024, 256 * 1024} {
		t := benchCorpus(b, sz)
		b.Run(fmt.Sprintf("%dK", sz/1024), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := Detect(t, DetectOptions{Layers: &hook}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
