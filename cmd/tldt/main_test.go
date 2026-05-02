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

func TestMain_URLFlag_ServesHTML(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><article>
			<p>The fox is clever and quick in the forest.</p>
			<p>Dogs are loyal and brave companions to their owners.</p>
			<p>Scientists study animal behavior carefully over many years.</p>
			<p>Research shows that animals communicate in complex ways.</p>
			<p>Ecosystems depend on balanced predator and prey relationships.</p>
		</article></body></html>`)
	}))
	defer ts.Close()

	stdout, _, ok := run(t, "", "--url", ts.URL, "--sentences", "2")
	if !ok {
		t.Fatal("--url: binary exited non-zero for valid HTML page")
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("--url: expected non-empty summary on stdout, got empty string")
	}
}

func TestMain_URLFlag_404(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()

	_, stderr, ok := run(t, "", "--url", ts.URL)
	if ok {
		t.Error("--url 404: expected non-zero exit code, got exit 0")
	}
	if !strings.Contains(stderr, "404") {
		t.Errorf("--url 404: expected '404' in stderr, got %q", stderr)
	}
}
