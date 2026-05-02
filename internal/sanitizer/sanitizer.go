// Package sanitizer removes Unicode steganographic content and normalizes text
// before it enters the summarization pipeline. It targets four attack classes:
// invisible control characters, bidi override sequences, zero-width characters,
// and compatibility variants that bypass pattern matching via homoglyph substitution.
//
// All operations are lossless for legitimate text — no semantic content is removed.
// Soft hyphens, zero-width joiners, and bidi controls are presentational only;
// NFKC normalization is the Unicode standard for identifier comparison (UTS#15).
package sanitizer

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// invisibleRanges lists Unicode codepoint ranges that have no visible glyph
// and are commonly used for steganographic injection.
// Ordered by attack frequency (most common first).
var invisibleRanges = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x00AD, Hi: 0x00AD, Stride: 1}, // SOFT HYPHEN — hides inside words
		{Lo: 0x200B, Hi: 0x200F, Stride: 1}, // ZERO WIDTH SPACE..RIGHT-TO-LEFT MARK
		{Lo: 0x2028, Hi: 0x2029, Stride: 1}, // LINE SEPARATOR, PARAGRAPH SEPARATOR
		{Lo: 0x202A, Hi: 0x202E, Stride: 1}, // LRE..RIGHT-TO-LEFT OVERRIDE (bidi attack)
		{Lo: 0x2060, Hi: 0x2064, Stride: 1}, // WORD JOINER..INVISIBLE PLUS
		{Lo: 0x2066, Hi: 0x2069, Stride: 1}, // LRI..PDI (bidi isolates)
		{Lo: 0xFEFF, Hi: 0xFEFF, Stride: 1}, // ZERO WIDTH NO-BREAK SPACE / BOM
	},
	R32: []unicode.Range32{
		{Lo: 0xE0000, Hi: 0xE01FF, Stride: 1}, // Tags block — used for ASCII-smuggling attacks
	},
}

// privateUseRanges covers the Unicode Private Use Area (PUA). PUA characters have
// no standardized meaning in plain text and cannot be legitimately required for
// content summarization.
var privateUseRanges = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0xE000, Hi: 0xF8FF, Stride: 1}, // BMP Private Use Area
	},
	R32: []unicode.Range32{
		{Lo: 0xF0000, Hi: 0xFFFFD, Stride: 1},  // Supplementary PUA-A
		{Lo: 0x100000, Hi: 0x10FFFD, Stride: 1}, // Supplementary PUA-B
	},
}

// InvisibleReport describes a single stripped codepoint for audit purposes.
type InvisibleReport struct {
	Rune     rune
	Name     string // human-readable description
	Offset   int    // byte offset in the original string
	Category string // "invisible", "bidi-control", "zero-width", "private-use", "format"
}

// runeCategory classifies a rune into a human-readable attack category.
func runeCategory(r rune) string {
	switch {
	case r >= 0x202A && r <= 0x202E:
		return "bidi-control"
	case r >= 0x2066 && r <= 0x2069:
		return "bidi-isolate"
	case r >= 0x200B && r <= 0x200F:
		return "zero-width"
	case r == 0x00AD:
		return "soft-hyphen"
	case r == 0xFEFF:
		return "bom"
	case r >= 0xE000 && r <= 0xF8FF:
		return "private-use"
	case r >= 0xE0000 && r <= 0xE01FF:
		return "tags-block"
	case unicode.Is(unicode.Cf, r):
		return "format"
	default:
		return "control"
	}
}

// runeName returns a short human-readable name for known codepoints.
func runeName(r rune) string {
	names := map[rune]string{
		0x00AD: "SOFT HYPHEN",
		0x200B: "ZERO WIDTH SPACE",
		0x200C: "ZERO WIDTH NON-JOINER",
		0x200D: "ZERO WIDTH JOINER",
		0x200E: "LEFT-TO-RIGHT MARK",
		0x200F: "RIGHT-TO-LEFT MARK",
		0x2028: "LINE SEPARATOR",
		0x2029: "PARAGRAPH SEPARATOR",
		0x202A: "LEFT-TO-RIGHT EMBEDDING",
		0x202B: "RIGHT-TO-LEFT EMBEDDING",
		0x202C: "POP DIRECTIONAL FORMATTING",
		0x202D: "LEFT-TO-RIGHT OVERRIDE",
		0x202E: "RIGHT-TO-LEFT OVERRIDE",
		0x2060: "WORD JOINER",
		0x2061: "FUNCTION APPLICATION",
		0x2062: "INVISIBLE TIMES",
		0x2063: "INVISIBLE SEPARATOR",
		0x2064: "INVISIBLE PLUS",
		0x2066: "LEFT-TO-RIGHT ISOLATE",
		0x2067: "RIGHT-TO-LEFT ISOLATE",
		0x2068: "FIRST STRONG ISOLATE",
		0x2069: "POP DIRECTIONAL ISOLATE",
		0xFEFF: "ZERO WIDTH NO-BREAK SPACE (BOM)",
	}
	if name, ok := names[r]; ok {
		return name
	}
	return fmt.Sprintf("U+%04X", r)
}

// StripInvisible removes Unicode Format characters (category Cf), enumerated
// invisible codepoints, and Private Use Area characters from s.
// Whitespace characters (\t, \n, \r, space) are always preserved.
// Returns valid UTF-8. Input must be valid UTF-8.
func StripInvisible(s string) string {
	return strings.Map(func(r rune) rune {
		// Always preserve printable whitespace
		if r == '\t' || r == '\n' || r == '\r' || r == ' ' {
			return r
		}
		// Strip enumerated invisible ranges
		if unicode.Is(invisibleRanges, r) {
			return -1
		}
		// Strip Private Use Area
		if unicode.Is(privateUseRanges, r) {
			return -1
		}
		// Strip remaining Unicode Format category (Cf) — catches future additions
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}

// NormalizeUnicode applies NFKC normalization to s.
// NFKC (Normalization Form Compatibility Composition) decomposes compatibility
// characters (fullwidth Latin, ligatures, enclosed alphanumerics) and recomposes
// canonically. This collapses the most common homoglyph substitutions used to
// defeat textual pattern matching.
//
// Note: NFKC does NOT collapse cross-script homoglyphs (e.g. Cyrillic 'а' vs
// Latin 'a'). Those require the Unicode Confusables database (UTS#39). NFKC
// addresses the low-cost variants; confusables require a separate lookup table.
func NormalizeUnicode(s string) string {
	return norm.NFKC.String(s)
}

// SanitizeAll applies StripInvisible followed by NormalizeUnicode.
// This is the single entry point for the --sanitize CLI flag.
// Order matters: strip first so invisible chars don't survive into the normalized form.
func SanitizeAll(s string) string {
	return NormalizeUnicode(StripInvisible(s))
}

// ReportInvisibles returns a description of every codepoint that StripInvisible
// would remove, without modifying s. Used by --detect-injection to produce an
// audit trail of what invisible content was found.
func ReportInvisibles(s string) []InvisibleReport {
	var reports []InvisibleReport
	offset := 0
	for _, r := range s {
		size := len(string(r))
		stripped := false
		if r != '\t' && r != '\n' && r != '\r' && r != ' ' {
			if unicode.Is(invisibleRanges, r) || unicode.Is(privateUseRanges, r) || unicode.Is(unicode.Cf, r) {
				stripped = true
			}
		}
		if stripped {
			reports = append(reports, InvisibleReport{
				Rune:     r,
				Name:     runeName(r),
				Offset:   offset,
				Category: runeCategory(r),
			})
		}
		offset += size
	}
	return reports
}
