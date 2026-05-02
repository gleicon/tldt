package sanitizer

import (
	"strings"
	"testing"
)

// --- StripInvisible tests ---

func TestStripInvisible_ZeroWidthSpace(t *testing.T) {
	input := "hello\u200bworld"
	got := StripInvisible(input)
	if got != "helloworld" {
		t.Errorf("StripInvisible(ZWS) = %q, want %q", got, "helloworld")
	}
}

func TestStripInvisible_ZeroWidthNonJoiner(t *testing.T) {
	got := StripInvisible("pro\u200cject")
	if got != "project" {
		t.Errorf("StripInvisible(ZWNJ) = %q, want %q", got, "project")
	}
}

func TestStripInvisible_ZeroWidthJoiner(t *testing.T) {
	got := StripInvisible("man\u200dwoman")
	if got != "manwoman" {
		t.Errorf("StripInvisible(ZWJ) = %q, want %q", got, "manwoman")
	}
}

func TestStripInvisible_SoftHyphen(t *testing.T) {
	got := StripInvisible("pro\u00adject")
	if got != "project" {
		t.Errorf("StripInvisible(SHY) = %q, want %q", got, "project")
	}
}

func TestStripInvisible_BidiRightToLeftOverride(t *testing.T) {
	// U+202E is the RIGHT-TO-LEFT OVERRIDE — used in ASCII-smuggling attacks
	got := StripInvisible("text\u202eover")
	if got != "textover" {
		t.Errorf("StripInvisible(RLO) = %q, want %q", got, "textover")
	}
}

func TestStripInvisible_AllBidiControls(t *testing.T) {
	bidiChars := []rune{0x202A, 0x202B, 0x202C, 0x202D, 0x202E}
	for _, r := range bidiChars {
		input := "a" + string(r) + "b"
		got := StripInvisible(input)
		if got != "ab" {
			t.Errorf("StripInvisible(U+%04X) = %q, want %q", r, got, "ab")
		}
	}
}

func TestStripInvisible_BidiIsolates(t *testing.T) {
	isolates := []rune{0x2066, 0x2067, 0x2068, 0x2069}
	for _, r := range isolates {
		input := "a" + string(r) + "b"
		got := StripInvisible(input)
		if got != "ab" {
			t.Errorf("StripInvisible(U+%04X) = %q, want %q", r, got, "ab")
		}
	}
}

func TestStripInvisible_BOM(t *testing.T) {
	got := StripInvisible("foo\uFEFFbar")
	if got != "foobar" {
		t.Errorf("StripInvisible(BOM) = %q, want %q", got, "foobar")
	}
}

func TestStripInvisible_WordJoiner(t *testing.T) {
	got := StripInvisible("can\u2060not")
	if got != "cannot" {
		t.Errorf("StripInvisible(WJ) = %q, want %q", got, "cannot")
	}
}

func TestStripInvisible_InvisibleMathOperators(t *testing.T) {
	// U+2061 FUNCTION APPLICATION, U+2062 INVISIBLE TIMES, etc.
	for _, r := range []rune{0x2061, 0x2062, 0x2063, 0x2064} {
		input := "a" + string(r) + "b"
		got := StripInvisible(input)
		if got != "ab" {
			t.Errorf("StripInvisible(U+%04X) = %q, want %q", r, got, "ab")
		}
	}
}

func TestStripInvisible_PreservesTab(t *testing.T) {
	got := StripInvisible("col1\tcol2")
	if got != "col1\tcol2" {
		t.Errorf("StripInvisible must preserve tab, got %q", got)
	}
}

func TestStripInvisible_PreservesNewline(t *testing.T) {
	got := StripInvisible("line1\nline2")
	if got != "line1\nline2" {
		t.Errorf("StripInvisible must preserve newline, got %q", got)
	}
}

func TestStripInvisible_PreservesCarriageReturn(t *testing.T) {
	got := StripInvisible("line1\r\nline2")
	if got != "line1\r\nline2" {
		t.Errorf("StripInvisible must preserve CRLF, got %q", got)
	}
}

func TestStripInvisible_PreservesAllPrintableASCII(t *testing.T) {
	var ascii strings.Builder
	for r := rune(0x20); r <= 0x7E; r++ {
		ascii.WriteRune(r)
	}
	s := ascii.String()
	got := StripInvisible(s)
	if got != s {
		t.Errorf("StripInvisible modified printable ASCII: lost %d chars", len(s)-len(got))
	}
}

func TestStripInvisible_PreservesUnicodeLetters(t *testing.T) {
	// Legitimate non-ASCII: emoji, CJK, accented chars
	cases := []string{
		"café",        // U+00E9 LATIN SMALL LETTER E WITH ACUTE
		"日本語",        // CJK
		"مرحبا",       // Arabic (right-to-left but not a control char)
		"Ελληνικά",    // Greek
	}
	for _, s := range cases {
		got := StripInvisible(s)
		if got != s {
			t.Errorf("StripInvisible(%q) = %q, want unchanged", s, got)
		}
	}
}

func TestStripInvisible_EmptyString(t *testing.T) {
	got := StripInvisible("")
	if got != "" {
		t.Errorf("StripInvisible(\"\") = %q, want empty", got)
	}
}

func TestStripInvisible_OnlyInvisible(t *testing.T) {
	input := "\u200b\u200c\u200d\u202e\ufeff"
	got := StripInvisible(input)
	if got != "" {
		t.Errorf("StripInvisible(all invisible) = %q, want empty", got)
	}
}

func TestStripInvisible_PrivateUseArea(t *testing.T) {
	// U+E001 is in the Private Use Area — no legitimate use in plain text
	got := StripInvisible("a\uE001b")
	if got != "ab" {
		t.Errorf("StripInvisible(PUA) = %q, want %q", got, "ab")
	}
}

// --- NormalizeUnicode tests ---

func TestNormalizeUnicode_FiLigature(t *testing.T) {
	// U+FB01 LATIN SMALL LIGATURE FI → "fi"
	got := NormalizeUnicode("\uFB01le")
	if got != "file" {
		t.Errorf("NormalizeUnicode(fi-ligature) = %q, want %q", got, "file")
	}
}

func TestNormalizeUnicode_FullwidthLatinA(t *testing.T) {
	// U+FF21 FULLWIDTH LATIN CAPITAL LETTER A → "A"
	got := NormalizeUnicode("\uFF21")
	if got != "A" {
		t.Errorf("NormalizeUnicode(fullwidth A) = %q, want %q", got, "A")
	}
}

func TestNormalizeUnicode_FullwidthDigit(t *testing.T) {
	// U+FF10 FULLWIDTH DIGIT ZERO → "0"
	got := NormalizeUnicode("\uFF10")
	if got != "0" {
		t.Errorf("NormalizeUnicode(fullwidth 0) = %q, want %q", got, "0")
	}
}

func TestNormalizeUnicode_EnclosedAlphanumeric(t *testing.T) {
	// U+2460 CIRCLED DIGIT ONE → "1"
	got := NormalizeUnicode("\u2460")
	if got != "1" {
		t.Errorf("NormalizeUnicode(circled 1) = %q, want %q", got, "1")
	}
}

func TestNormalizeUnicode_PreservesNormalASCII(t *testing.T) {
	s := "normal ASCII text with punctuation: hello, world!"
	got := NormalizeUnicode(s)
	if got != s {
		t.Errorf("NormalizeUnicode modified normal ASCII: %q → %q", s, got)
	}
}

func TestNormalizeUnicode_CyrillicRemainsDistinct(t *testing.T) {
	// Cyrillic 'а' (U+0430) is NOT normalized to Latin 'a' (U+0061) by NFKC.
	// This is expected — NFKC covers compatibility variants, not confusables.
	cyrillic := "\u0430" // Cyrillic small letter a
	got := NormalizeUnicode(cyrillic)
	if got == "a" {
		t.Errorf("NormalizeUnicode incorrectly collapsed Cyrillic 'а' to Latin 'a' — confusables require UTS#39 lookup")
	}
	if got != cyrillic {
		t.Errorf("NormalizeUnicode(Cyrillic а) = %q, want %q (unchanged)", got, cyrillic)
	}
}

// --- SanitizeAll tests ---

func TestSanitizeAll_ChainedOps(t *testing.T) {
	// Input: "fi" literal + ZWNJ (invisible) + fi-ligature (U+FB01)
	// ZWNJ stripped; U+FB01 NFKC-expands to "fi"
	// Result: "fi" + "fi" = "fifi" (two fi sequences, one visible one from ligature)
	input := "fi\u200c\uFB01"
	got := SanitizeAll(input)
	want := "fifi"
	if got != want {
		t.Errorf("SanitizeAll(chained) = %q, want %q", got, want)
	}
}

func TestSanitizeAll_BidiPlusLigature(t *testing.T) {
	input := "text\u202e\uFB01le"
	got := SanitizeAll(input)
	want := "textfile"
	if got != want {
		t.Errorf("SanitizeAll(bidi+ligature) = %q, want %q", got, want)
	}
}

func TestSanitizeAll_CleanInput(t *testing.T) {
	s := "The quick brown fox jumps over the lazy dog."
	got := SanitizeAll(s)
	if got != s {
		t.Errorf("SanitizeAll(clean) = %q, want unchanged", got)
	}
}

// --- ReportInvisibles tests ---

func TestReportInvisibles_FindsZWS(t *testing.T) {
	input := "a\u200bb"
	reports := ReportInvisibles(input)
	if len(reports) != 1 {
		t.Fatalf("ReportInvisibles: got %d findings, want 1", len(reports))
	}
	if reports[0].Rune != 0x200B {
		t.Errorf("ReportInvisibles: rune = U+%04X, want U+200B", reports[0].Rune)
	}
	if reports[0].Category != "zero-width" {
		t.Errorf("ReportInvisibles: category = %q, want %q", reports[0].Category, "zero-width")
	}
	if reports[0].Offset != 1 {
		t.Errorf("ReportInvisibles: offset = %d, want 1", reports[0].Offset)
	}
}

func TestReportInvisibles_DoesNotModifyInput(t *testing.T) {
	input := "hello\u200bworld"
	_ = ReportInvisibles(input)
	if input != "hello\u200bworld" {
		t.Error("ReportInvisibles must not modify input")
	}
}

func TestReportInvisibles_MultipleCodepoints(t *testing.T) {
	input := "a\u200b\u202eb"
	reports := ReportInvisibles(input)
	if len(reports) != 2 {
		t.Fatalf("ReportInvisibles: got %d findings, want 2", len(reports))
	}
}

func TestReportInvisibles_CleanInput(t *testing.T) {
	reports := ReportInvisibles("normal text")
	if len(reports) != 0 {
		t.Errorf("ReportInvisibles(clean): got %d findings, want 0", len(reports))
	}
}

func TestReportInvisibles_BidiControlName(t *testing.T) {
	reports := ReportInvisibles("a\u202eb")
	if len(reports) != 1 {
		t.Fatalf("want 1 report, got %d", len(reports))
	}
	if reports[0].Name != "RIGHT-TO-LEFT OVERRIDE" {
		t.Errorf("Name = %q, want %q", reports[0].Name, "RIGHT-TO-LEFT OVERRIDE")
	}
}
