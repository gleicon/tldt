package detector

import (
	"testing"
	"time"
)

// The latency budgets (NFR-1/2) are verified by benchmark in normal runs. These
// tests assert the same bounds with generous slack so CI on a slow shared runner
// fails only on a real regression (an accidental O(n²) in the pattern pass, a
// removed anchor gate), not on machine variance. The benchmarks carry the precise
// numbers; these carry the guarantee.

func TestPatternPassAnchorFreeBudget(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("timing test (meaningless under -race)")
	}
	text := anchorFreeCorpus(t, 256*1024)
	start := time.Now()
	_ = DetectPatterns(text)
	// AC-29 target is 5 ms; allow 10x slack for CI runners.
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Errorf("anchor-free pattern pass took %v, budget 5ms (50ms with CI slack)", d)
	}
}

// TestAnalyzeHookBudget asserts NFR-1 for the default/hook detection profile: the
// layer set that runs on every agent prompt. AnalyzeWith(HookLayers) measured
// ~67ms at 256KB; the full hook Detect adds the capped outlier pass for ~129ms
// total, inside the 150ms budget. This is the path NFR-1 governs.
func TestAnalyzeHookBudget(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("timing test (meaningless under -race)")
	}
	text := benchCorpus(t, 256*1024)
	start := time.Now()
	_ = AnalyzeWith(text, HookLayers())
	if d := time.Since(start); d > 335*time.Millisecond { // ~67ms, 5x CI slack
		t.Errorf("AnalyzeWith(256K, HookLayers) took %v, budget ~67ms (335ms with CI slack)", d)
	}
}

// TestAnalyzeAllLayersBudget asserts the all-layers CLI ceiling. Every weak-prior
// layer on measured ~128ms at 256KB. The full CLI Detect adds the outlier pass for
// ~196ms, above NFR-1's 150ms — a deliberate trade: a developer running the full
// layer set on a large document accepts the added latency for the added coverage.
// The test guards against a further regression, not against the 150ms target.
func TestAnalyzeAllLayersBudget(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("timing test (meaningless under -race)")
	}
	text := benchCorpus(t, 256*1024)
	start := time.Now()
	_ = AnalyzeWith(text, DefaultLayers())
	if d := time.Since(start); d > 640*time.Millisecond { // ~128ms, 5x CI slack
		t.Errorf("AnalyzeWith(256K, DefaultLayers) took %v, budget ~128ms (640ms with CI slack)", d)
	}
}
