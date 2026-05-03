package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// binaryPath holds the compiled binary built once by TestMain.
var binaryPath string

// coverDir is where the -cover binary writes its coverage data.
// If GOCOVERDIR is set in the environment (e.g. from Makefile), it is used as-is
// so that the caller can merge the data afterwards. Otherwise a temp dir is used
// and cleaned up automatically.
var coverDir string

// TestMain builds the binary with -cover before running tests.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "tldt-bin-*")
	if err != nil {
		panic("cannot create temp dir: " + err.Error())
	}
	bin := filepath.Join(tmp, "tldt")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	// Resolve coverDir: honour caller-provided GOCOVERDIR, else use a subdir of tmp.
	if dir := os.Getenv("GOCOVERDIR"); dir != "" {
		coverDir = dir
	} else {
		coverDir = filepath.Join(tmp, "covdata")
		if err := os.MkdirAll(coverDir, 0755); err != nil {
			panic("cannot create coverdir: " + err.Error())
		}
	}

	out, err := exec.Command("go", "build", "-cover", "-o", bin, ".").CombinedOutput()
	if err != nil {
		panic("build failed: " + string(out))
	}
	binaryPath = bin
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// run executes the binary with given stdin and args.
// Sets GOCOVERDIR so the -cover-built binary writes coverage data.
// Returns stdout, stderr, and whether exit code was 0.
func run(t *testing.T, stdin string, args ...string) (stdout, stderr string, ok bool) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return outBuf.String(), errBuf.String(), err == nil
}

// writeTempFile creates a temp file with content, returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "tldt-ref-*.txt")
	if err != nil {
		t.Fatalf("cannot create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("cannot write temp file: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// ── validateInput ─────────────────────────────────────────────────────────────

func TestValidateInput_NormalText(t *testing.T) {
	text, isEmpty, err := validateInput([]byte("Hello world."))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isEmpty {
		t.Error("want isEmpty=false")
	}
	if text != "Hello world." {
		t.Errorf("text = %q, want %q", text, "Hello world.")
	}
}

func TestValidateInput_WhitespaceOnly(t *testing.T) {
	_, isEmpty, err := validateInput([]byte("   \n\t  "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isEmpty {
		t.Error("whitespace-only: want isEmpty=true")
	}
}

func TestValidateInput_Empty(t *testing.T) {
	_, isEmpty, err := validateInput([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isEmpty {
		t.Error("empty: want isEmpty=true")
	}
}

func TestValidateInput_NULByte(t *testing.T) {
	_, _, err := validateInput([]byte("hello\x00world"))
	if err == nil {
		t.Error("NUL byte: want error, got nil")
	}
}

func TestValidateInput_InvalidUTF8(t *testing.T) {
	// 0xff 0xfe alone (no NUL) exercises the utf8.Valid branch, not the NUL branch.
	_, _, err := validateInput([]byte{0xff, 0xfe})
	if err == nil {
		t.Error("invalid UTF-8 (no NUL): want error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("error message = %q, want to mention UTF-8", err.Error())
	}
}

func TestValidateInput_ValidUnicode(t *testing.T) {
	_, isEmpty, err := validateInput([]byte("Héllo wörld."))
	if err != nil {
		t.Fatalf("valid unicode: unexpected error: %v", err)
	}
	if isEmpty {
		t.Error("valid unicode: want isEmpty=false")
	}
}

// ── resolveInputBytes ─────────────────────────────────────────────────────────

func TestResolveInputBytes_PositionalArgs(t *testing.T) {
	got, err := resolveInputBytes([]string{"hello", "world"}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("got %q, want %q", string(got), "hello world")
	}
}

func TestResolveInputBytes_FilePath(t *testing.T) {
	path := writeTempFile(t, "file content here")
	got, err := resolveInputBytes([]string{}, path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "file content here" {
		t.Errorf("got %q, want %q", string(got), "file content here")
	}
}

func TestResolveInputBytes_FileNotFound(t *testing.T) {
	_, err := resolveInputBytes([]string{}, "/nonexistent/path/file.txt", "")
	if err == nil {
		t.Error("missing file: want error, got nil")
	}
}

func TestResolveInputBytes_NoInput(t *testing.T) {
	_, err := resolveInputBytes([]string{}, "", "")
	if err == nil {
		t.Error("no input: want error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "no input") {
		t.Errorf("error = %q, want to mention 'no input'", err.Error())
	}
}

func TestResolveInputBytes_Stdin(t *testing.T) {
	// Replace os.Stdin with a pipe to exercise the stdin branch.
	// Tests in package main run sequentially (no t.Parallel), so global mutation is safe.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		r.Close()
	}()
	if _, err := w.WriteString("piped content here"); err != nil {
		t.Fatalf("write pipe: %v", err)
	}
	w.Close()

	got, err := resolveInputBytes([]string{}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "piped content here" {
		t.Errorf("stdin: got %q, want %q", string(got), "piped content here")
	}
}

// ── applySentenceCap ──────────────────────────────────────────────────────────

func TestApplySentenceCap_BelowCap(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence."
	got := applySentenceCap(text, 10)
	if got != text {
		t.Errorf("below cap: text modified unexpectedly\ngot:  %q\nwant: %q", got, text)
	}
}

func TestApplySentenceCap_AtCap(t *testing.T) {
	text := "A. B. C."
	got := applySentenceCap(text, 3)
	// Exactly at cap → no truncation
	parts := strings.Split(got, ".")
	if len(parts) < 3 {
		t.Errorf("at-cap: fewer sentences than expected in %q", got)
	}
}

func TestApplySentenceCap_ExceedsCap(t *testing.T) {
	// Build 10 sentences, cap at 3
	sentences := []string{
		"Alpha sentence one.", "Beta sentence two.", "Gamma sentence three.",
		"Delta sentence four.", "Epsilon sentence five.", "Zeta sentence six.",
		"Eta sentence seven.", "Theta sentence eight.", "Iota sentence nine.",
		"Kappa sentence ten.",
	}
	text := strings.Join(sentences, " ")
	got := applySentenceCap(text, 3)
	// Result must not contain sentences 4-10
	for _, s := range sentences[3:] {
		if strings.Contains(got, s) {
			t.Errorf("applySentenceCap kept sentence beyond cap: %q", s)
		}
	}
}

func TestApplySentenceCap_ExceedsCap_OutputLength(t *testing.T) {
	sentences := make([]string, 20)
	for i := range sentences {
		sentences[i] = "This is sentence number one here."
	}
	text := strings.Join(sentences, " ")
	got := applySentenceCap(text, 5)
	// Capped result must be shorter than original
	if len(got) >= len(text) {
		t.Errorf("capped text (len=%d) not shorter than original (len=%d)", len(got), len(text))
	}
}

// ── formatTokens ──────────────────────────────────────────────────────────────

func TestFormatTokens_SmallNumbers(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"}, {9, "9"}, {99, "99"}, {999, "999"},
	}
	for _, tc := range cases {
		if got := formatTokens(tc.n); got != tc.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestFormatTokens_Thousands(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1000, "1,000"}, {1234, "1,234"}, {12345, "12,345"},
		{123456, "123,456"}, {1234567, "1,234,567"},
	}
	for _, tc := range cases {
		if got := formatTokens(tc.n); got != tc.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// ── groupIntoParagraphs ───────────────────────────────────────────────────────

func TestGroupIntoParagraphs_ZeroN(t *testing.T) {
	got := groupIntoParagraphs([]string{"A.", "B.", "C."}, 0)
	if !strings.Contains(got, "A.") || !strings.Contains(got, "B.") {
		t.Errorf("n=0 dropped sentences: %q", got)
	}
}

func TestGroupIntoParagraphs_One(t *testing.T) {
	got := groupIntoParagraphs([]string{"A.", "B.", "C."}, 1)
	if strings.Contains(got, "\n\n") {
		t.Errorf("n=1 should have no double-newlines, got %q", got)
	}
}

func TestGroupIntoParagraphs_Equal(t *testing.T) {
	got := groupIntoParagraphs([]string{"A.", "B.", "C."}, 3)
	if parts := strings.Split(got, "\n\n"); len(parts) != 3 {
		t.Errorf("n=3 from 3 sentences: want 3 paragraphs, got %d", len(parts))
	}
}

func TestGroupIntoParagraphs_NCap(t *testing.T) {
	got := groupIntoParagraphs([]string{"A.", "B."}, 10)
	if !strings.Contains(got, "A.") || !strings.Contains(got, "B.") {
		t.Errorf("n>len dropped sentences: %q", got)
	}
}

func TestGroupIntoParagraphs_Empty(t *testing.T) {
	if got := groupIntoParagraphs([]string{}, 3); got != "" {
		t.Errorf("empty input: want \"\", got %q", got)
	}
}

func TestGroupIntoParagraphs_AllSentencesPresent(t *testing.T) {
	sentences := []string{"First.", "Second.", "Third.", "Fourth.", "Fifth."}
	got := groupIntoParagraphs(sentences, 2)
	for _, s := range sentences {
		if !strings.Contains(got, s) {
			t.Errorf("dropped sentence %q", s)
		}
	}
}

// ── main() via subprocess ─────────────────────────────────────────────────────

const shortText = "The fox is clever and quick. Dogs are loyal and brave. Scientists study animals carefully."

func TestMain_StdinText(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-sentences", "2")
	if !ok {
		t.Fatal("binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("expected non-empty output")
	}
}

func TestMain_AlgorithmLexRank(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-algorithm", "lexrank", "-sentences", "2")
	if !ok {
		t.Fatal("lexrank: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("lexrank: empty output")
	}
}

func TestMain_AlgorithmTextRank(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-algorithm", "textrank", "-sentences", "2")
	if !ok {
		t.Fatal("textrank: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("textrank: empty output")
	}
}

func TestMain_AlgorithmGraph(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-algorithm", "graph", "-sentences", "2")
	if !ok {
		t.Fatal("graph: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("graph: empty output")
	}
}

func TestMain_AlgorithmEnsemble(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-algorithm", "ensemble", "-sentences", "2")
	if !ok {
		t.Fatal("ensemble: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("ensemble: empty output")
	}
}

func TestMain_AlgorithmUnknown(t *testing.T) {
	_, stderr, ok := run(t, shortText, "-algorithm", "bogus")
	if ok {
		t.Error("unknown algorithm: want non-zero exit")
	}
	if !strings.Contains(stderr, "bogus") {
		t.Errorf("stderr %q does not mention unknown algorithm name", stderr)
	}
}

func TestMain_FormatJSON(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-format", "json", "-sentences", "2")
	if !ok {
		t.Fatal("json format: binary exited non-zero")
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Errorf("json format: output not JSON object: %q", stdout)
	}
}

func TestMain_FormatMarkdown(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-format", "markdown", "-sentences", "2")
	if !ok {
		t.Fatal("markdown format: binary exited non-zero")
	}
	if !strings.Contains(stdout, ">") {
		t.Errorf("markdown format: expected blockquote '>', got %q", stdout)
	}
}

func TestMain_FormatText(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-format", "text", "-sentences", "2")
	if !ok {
		t.Fatal("text format: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("text format: empty output")
	}
}

func TestMain_Verbose(t *testing.T) {
	_, stderr, ok := run(t, shortText, "-verbose", "-sentences", "2")
	if !ok {
		t.Fatal("verbose: binary exited non-zero")
	}
	if !strings.Contains(stderr, "tokens") {
		t.Errorf("verbose: stderr %q does not mention 'tokens'", stderr)
	}
}

func TestMain_Explain(t *testing.T) {
	_, stderr, ok := run(t, shortText, "-explain", "-sentences", "2")
	if !ok {
		t.Fatal("explain: binary exited non-zero")
	}
	if !strings.Contains(stderr, "explain:") {
		t.Errorf("explain: stderr %q does not contain 'explain:'", stderr)
	}
}

func TestMain_ExplainGraph(t *testing.T) {
	// Graph doesn't implement Explainer; must fall back without crashing
	_, stderr, ok := run(t, shortText, "-explain", "-algorithm", "graph", "-sentences", "2")
	if !ok {
		t.Fatal("explain+graph: binary exited non-zero")
	}
	if !strings.Contains(stderr, "not supported") {
		t.Errorf("explain+graph: expected fallback note in stderr, got %q", stderr)
	}
}

func TestMain_Paragraphs(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-paragraphs", "2", "-sentences", "3")
	if !ok {
		t.Fatal("paragraphs: binary exited non-zero")
	}
	if !strings.Contains(stdout, "\n\n") {
		t.Errorf("paragraphs=2: expected blank line in output, got %q", stdout)
	}
}

func TestMain_FileInput(t *testing.T) {
	path := writeTempFile(t, shortText)
	stdout, _, ok := run(t, "", "-f", path, "-sentences", "2")
	if !ok {
		t.Fatal("-f file: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("-f file: empty output")
	}
}

func TestMain_FileNotFound(t *testing.T) {
	_, _, ok := run(t, "", "-f", "/no/such/file.txt")
	if ok {
		t.Error("missing file: want non-zero exit")
	}
}

func TestMain_NoCap(t *testing.T) {
	stdout, _, ok := run(t, shortText, "-no-cap", "-sentences", "2")
	if !ok {
		t.Fatal("-no-cap: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("-no-cap: empty output")
	}
}

func TestMain_BinaryInput(t *testing.T) {
	// Feed NUL byte via file — binary input must exit non-zero
	path := writeTempFile(t, "hello\x00world")
	_, stderr, ok := run(t, "", "-f", path)
	if ok {
		t.Error("binary input via file: want non-zero exit")
	}
	if !strings.Contains(stderr, "binary") {
		t.Errorf("binary input: stderr %q does not mention 'binary'", stderr)
	}
}

func TestMain_Rouge(t *testing.T) {
	ref := writeTempFile(t, "Foxes and dogs are animals studied by scientists.")
	_, stderr, ok := run(t, shortText, "-rouge", ref, "-sentences", "2")
	if !ok {
		t.Fatal("rouge: binary exited non-zero")
	}
	if !strings.Contains(stderr, "rouge-1") {
		t.Errorf("rouge: stderr %q does not contain 'rouge-1'", stderr)
	}
}

func TestMain_RougeFileNotFound(t *testing.T) {
	_, _, ok := run(t, shortText, "-rouge", "/no/such/ref.txt", "-sentences", "2")
	if ok {
		t.Error("rouge missing file: want non-zero exit")
	}
}

func TestMain_VerboseJSON_NoTokenStats(t *testing.T) {
	// -verbose with -format json must NOT print token stats (TOK-02)
	_, stderr, ok := run(t, shortText, "-verbose", "-format", "json", "-sentences", "2")
	if !ok {
		t.Fatal("verbose+json: binary exited non-zero")
	}
	if strings.Contains(stderr, "tokens") {
		t.Errorf("verbose+json: stderr must not print token stats, got %q", stderr)
	}
}

func TestMain_EmptyInput_ExitsZero(t *testing.T) {
	_, _, ok := run(t, "   ", "-sentences", "2")
	if !ok {
		t.Error("whitespace-only stdin: want exit 0")
	}
}

func TestMain_PositionalArgs(t *testing.T) {
	// Exercise resolveInputBytes positional-args branch (no stdin, no -f)
	stdout, _, ok := run(t, "",
		"The fox is clever and quick.",
		"Dogs are loyal and brave.",
		"Scientists study animals carefully.",
		"-sentences", "2",
	)
	if !ok {
		t.Fatal("positional args: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("positional args: empty output")
	}
}

func TestMain_NoInput_ExitsNonZero(t *testing.T) {
	// No stdin, no -f, no positional args → resolveInputBytes returns error
	_, stderr, ok := run(t, "")
	if ok {
		t.Error("no input: want non-zero exit")
	}
	if !strings.Contains(stderr, "no input") {
		t.Errorf("no input: stderr %q does not mention 'no input'", stderr)
	}
}

// ── --url flag integration tests ──────────────────────────────────────────────
//
// NOTE: httptest.NewServer binds to 127.0.0.1 (loopback). After SSRF hardening
// (Phase 8, Plan 01), the fetcher blocks loopback addresses at the initial pre-check.
// Tests that previously used httptest to serve HTML/404/redirect/non-HTML content are
// replaced with SSRF-focused binary integration tests that verify SSRF errors surface
// correctly through the CLI binary. The underlying fetcher behaviors (404, redirect,
// non-HTML) remain covered by the package-level tests in internal/fetcher/fetcher_test.go
// using the lookupHost override pattern.

// TestMain_URLFlag_SSRFBlocksLoopback verifies that the binary emits an SSRF error
// and exits non-zero when a URL resolves to a loopback address (httptest server).
func TestMain_URLFlag_SSRFBlocksLoopback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><article><p>Content that should never be reached.</p></article></body></html>`)
	}))
	defer ts.Close()

	_, stderr, ok := run(t, "", "--url", ts.URL)
	if ok {
		t.Error("--url SSRF loopback: expected non-zero exit code, got exit 0")
	}
	if !strings.Contains(stderr, "SSRF blocked") && !strings.Contains(stderr, "loopback") {
		t.Errorf("--url SSRF loopback: expected SSRF block error in stderr, got %q", stderr)
	}
}

// TestMain_URLFlag_SSRFBlocksPrivateIP verifies that the binary emits an SSRF error
// for a URL with a private IP in the hostname.
func TestMain_URLFlag_SSRFBlocksPrivateIP(t *testing.T) {
	_, stderr, ok := run(t, "", "--url", "http://192.168.1.1/admin")
	if ok {
		t.Error("--url SSRF private IP: expected non-zero exit code, got exit 0")
	}
	if !strings.Contains(stderr, "SSRF blocked") && !strings.Contains(stderr, "private") {
		t.Errorf("--url SSRF private IP: expected SSRF block error in stderr, got %q", stderr)
	}
}

// TestMain_URLFlag_SSRFBlocksCloudMeta verifies that the binary emits an SSRF error
// for the AWS instance metadata endpoint (169.254.169.254).
func TestMain_URLFlag_SSRFBlocksCloudMeta(t *testing.T) {
	_, stderr, ok := run(t, "", "--url", "http://169.254.169.254/latest/meta-data/")
	if ok {
		t.Error("--url SSRF cloud meta: expected non-zero exit code, got exit 0")
	}
	if !strings.Contains(stderr, "SSRF blocked") && !strings.Contains(stderr, "link-local") {
		t.Errorf("--url SSRF cloud meta: expected SSRF block error in stderr, got %q", stderr)
	}
}

// ── config file and --level flag integration tests ────────────────────────────

// longText has 12 sentences — enough for --level aggressive (10) tests.
const longText = "The fox is clever and quick in the forest. " +
	"Dogs are loyal and brave companions to humans. " +
	"Scientists study animal behavior carefully over many years. " +
	"Research shows that animals communicate in complex ways. " +
	"Ecosystems depend on balanced predator and prey relationships. " +
	"Migration patterns change with the seasons across continents. " +
	"Marine biologists track whale populations using acoustic sensors. " +
	"Forest conservation efforts have increased biodiversity in protected areas. " +
	"Climate change affects animal habitats around the world significantly. " +
	"Genetic studies reveal evolutionary relationships between species groups. " +
	"Urban wildlife adapts to human environments in surprising ways. " +
	"Behavioral ecology combines field observation with statistical modeling."

// writeConfig writes a TOML config file into a fresh temp HOME directory and
// sets HOME so the subprocess inherits it.
func writeConfig(t *testing.T, content string) {
	t.Helper()
	tmpHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpHome, ".tldt.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmpHome)
}

// countNonEmptyLines returns the number of non-empty lines in s.
func countNonEmptyLines(s string) int {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	return count
}

func TestMain_ConfigFileDefaults(t *testing.T) {
	// algorithm="ensemble" sentences=3 — verify config values are used when no CLI flags override
	writeConfig(t, "algorithm = \"ensemble\"\nsentences = 3\n")
	stdout, _, ok := run(t, longText)
	if !ok {
		t.Fatal("config file defaults: binary exited non-zero")
	}
	got := countNonEmptyLines(stdout)
	if got != 3 {
		t.Errorf("config file defaults: want 3 output lines, got %d\nstdout: %q", got, stdout)
	}
}

func TestMain_ConfigOverrideSentences(t *testing.T) {
	// Config sets sentences=7, CLI --sentences 2 must override (CFG-02)
	writeConfig(t, "sentences = 7\n")
	stdout, _, ok := run(t, shortText, "--sentences", "2")
	if !ok {
		t.Fatal("config override sentences: binary exited non-zero")
	}
	got := countNonEmptyLines(stdout)
	if got != 2 {
		t.Errorf("config override sentences: want 2 output lines, got %d\nstdout: %q", got, stdout)
	}
}

func TestMain_ConfigOverrideAlgorithm(t *testing.T) {
	// Config sets algorithm="textrank", CLI --algorithm lexrank must override (CFG-02)
	writeConfig(t, "algorithm = \"textrank\"\n")
	_, _, ok := run(t, shortText, "--algorithm", "lexrank", "--sentences", "2")
	if !ok {
		t.Fatal("config override algorithm: binary exited non-zero")
	}
}

func TestMain_ConfigMissing(t *testing.T) {
	// No .tldt.toml in HOME — should silently use built-in defaults (CFG-03)
	t.Setenv("HOME", t.TempDir())
	stdout, _, ok := run(t, shortText, "--sentences", "2")
	if !ok {
		t.Fatal("config missing: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("config missing: expected non-empty output")
	}
}

func TestMain_ConfigMalformed(t *testing.T) {
	// Malformed TOML in .tldt.toml — should silently use built-in defaults (CFG-03)
	writeConfig(t, "algorithm = bad toml [[[")
	stdout, _, ok := run(t, shortText, "--sentences", "2")
	if !ok {
		t.Fatal("config malformed: binary exited non-zero")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("config malformed: expected non-empty output")
	}
}

func TestMain_LevelLite(t *testing.T) {
	// --level lite produces exactly 10 sentences (CFG-04: lite = least compression)
	stdout, _, ok := run(t, longText, "--level", "lite")
	if !ok {
		t.Fatal("--level lite: binary exited non-zero")
	}
	got := countNonEmptyLines(stdout)
	if got != 10 {
		t.Errorf("--level lite: want 10 output lines, got %d\nstdout: %q", got, stdout)
	}
}

func TestMain_LevelStandard(t *testing.T) {
	// --level standard produces exactly 5 sentences (CFG-04)
	stdout, _, ok := run(t, longText, "--level", "standard")
	if !ok {
		t.Fatal("--level standard: binary exited non-zero")
	}
	got := countNonEmptyLines(stdout)
	if got != 5 {
		t.Errorf("--level standard: want 5 output lines, got %d\nstdout: %q", got, stdout)
	}
}

func TestMain_LevelAggressive(t *testing.T) {
	// --level aggressive produces exactly 3 sentences (CFG-04: aggressive = most compression)
	stdout, _, ok := run(t, longText, "--level", "aggressive")
	if !ok {
		t.Fatal("--level aggressive: binary exited non-zero")
	}
	got := countNonEmptyLines(stdout)
	if got != 3 {
		t.Errorf("--level aggressive: want 3 output lines, got %d\nstdout: %q", got, stdout)
	}
}

func TestMain_LevelInvalid(t *testing.T) {
	// --level bogus must exit non-zero with descriptive error (T-05-06)
	_, stderr, ok := run(t, shortText, "--level", "bogus")
	if ok {
		t.Error("--level bogus: want non-zero exit")
	}
	if !strings.Contains(stderr, "unknown --level") {
		t.Errorf("--level bogus: stderr %q does not contain 'unknown --level'", stderr)
	}
}

func TestMain_LevelOverriddenBySentences(t *testing.T) {
	// Config sets level="aggressive" (3), CLI --sentences 2 must override (CFG-05)
	writeConfig(t, "level = \"aggressive\"\n")
	stdout, _, ok := run(t, longText, "--sentences", "2")
	if !ok {
		t.Fatal("level overridden by sentences: binary exited non-zero")
	}
	got := countNonEmptyLines(stdout)
	if got != 2 {
		t.Errorf("level overridden by sentences: want 2 output lines, got %d\nstdout: %q", got, stdout)
	}
}

func TestMain_ConfigLevelDefault(t *testing.T) {
	// Config sets level="lite" — running with no flags should produce 10 sentences (CFG-05: lite = least compression)
	writeConfig(t, "level = \"lite\"\n")
	stdout, _, ok := run(t, longText)
	if !ok {
		t.Fatal("config level default: binary exited non-zero")
	}
	got := countNonEmptyLines(stdout)
	if got != 10 {
		t.Errorf("config level default: want 10 output lines, got %d\nstdout: %q", got, stdout)
	}
}

// ── --detect-pii and --sanitize-pii integration tests ─────────────────────────

// piiText has 3 sentences; one contains an email address for PII detection tests.
const piiText = "Contact alice@example.com for support. This is a second sentence for content. Third sentence here."

// cleanText has 3 sentences with no PII content.
const cleanText = "The quick brown fox jumps over the lazy dog. " +
	"No secrets or personal information here. " +
	"This is a third sentence."

// TestDetectPIIFlag verifies --detect-pii is advisory: summary on stdout, WARNING on stderr.
func TestDetectPIIFlag(t *testing.T) {
	stdout, stderr, ok := run(t, piiText, "--detect-pii")
	if !ok {
		t.Fatalf("tldt --detect-pii failed\nstderr: %s", stderr)
	}
	if strings.Contains(stdout, "pii-detect") {
		t.Errorf("--detect-pii: pii-detect output leaked to stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "pii-detect") {
		t.Errorf("--detect-pii: expected pii-detect output on stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "WARNING") {
		t.Errorf("--detect-pii: expected WARNING on stderr for PII input, got: %q", stderr)
	}
}

// TestDetectPIIFlagCleanInput verifies --detect-pii reports "no findings" for clean text.
func TestDetectPIIFlagCleanInput(t *testing.T) {
	stdout, stderr, ok := run(t, cleanText, "--detect-pii")
	if !ok {
		t.Fatalf("tldt --detect-pii (clean) failed\nstderr: %s", stderr)
	}
	_ = stdout // summary on stdout is expected; content not checked here
	if !strings.Contains(stderr, "no findings") {
		t.Errorf("--detect-pii: expected 'no findings' on stderr for clean input, got: %q", stderr)
	}
}

// TestSanitizePIIFlag verifies --sanitize-pii redacts PII before summarization.
func TestSanitizePIIFlag(t *testing.T) {
	const email = "alice@example.com"
	stdout, stderr, ok := run(t, piiText, "--sanitize-pii")
	if !ok {
		t.Fatalf("tldt --sanitize-pii failed\nstderr: %s", stderr)
	}
	if strings.Contains(stdout, email) {
		t.Errorf("--sanitize-pii: original email present in stdout summary: %q", stdout)
	}
	if !strings.Contains(stderr, "redaction(s) applied") {
		t.Errorf("--sanitize-pii: expected 'redaction(s) applied' on stderr, got: %q", stderr)
	}
}

// TestSanitizePIIFlagStdoutOnly verifies --sanitize-pii output is summary-only on stdout.
func TestSanitizePIIFlagStdoutOnly(t *testing.T) {
	const apiKeyText = "Email sk-abc12345678901234567. Second sentence for content. Third sentence."
	stdout, _, ok := run(t, apiKeyText, "--sanitize-pii")
	if !ok {
		t.Fatalf("tldt --sanitize-pii (api key) failed")
	}
	if strings.Contains(stdout, "pii-detect") {
		t.Errorf("--sanitize-pii: pii-detect output on stdout: %q", stdout)
	}
}
