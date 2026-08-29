package detector

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"html"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Decoder bounds. Decoding runs on untrusted input inside an agent hook, so every
// chain is bounded in depth, in per-chain output, and in total output across the
// document. Depth alone is not sufficient: a document can carry thousands of small
// blobs whose individual chains are all legal.
const (
	// MaxDecodeDepth caps how many times a chain may be decoded. Observed
	// real-world layering (base64 of hex of text) is two levels; three leaves
	// headroom without opening an unbounded recursion.
	MaxDecodeDepth = 3

	// MaxExpansionRatio aborts a chain whose cumulative output exceeds this
	// multiple of the encoded token. This is what stops a decompression-style
	// bomb; base64's fixed 4:3 ratio means no legitimate chain approaches it.
	MaxExpansionRatio = 10

	// MaxChainBytes caps cumulative output for a single chain.
	MaxChainBytes = 1 << 20 // 1 MiB

	// MaxDocumentDecodeBytes caps cumulative output across every chain in one
	// document, so many small blobs cannot add up to an unbounded total.
	MaxDocumentDecodeBytes = 4 << 20 // 4 MiB

	// MinCandidateLen is the shortest encoded token considered. Eight base64
	// characters carry six bytes, the shortest payload worth reporting.
	MinCandidateLen = 8

	// MinDecodedLen is the shortest decoded output that may produce a finding.
	MinDecodedLen = 6

	// MinPrintableRatio is the fraction of decoded bytes that must be printable
	// before the output is treated as recovered text rather than binary. Random
	// key material and compiled artifacts fall well below it.
	MinPrintableRatio = 0.85
)

// budget tracks cumulative decoded output for one document.
type budget struct {
	documentBytes int
}

func (b *budget) spend(n int) bool {
	if b.documentBytes+n > MaxDocumentDecodeBytes {
		return false
	}
	b.documentBytes += n
	return true
}

// decoded is one recovered payload plus the chain that produced it.
type decoded struct {
	text  string // recovered plaintext
	chain string // encoding chain, e.g. "base64" or "percent>base64"
	// offset is the byte offset in the ORIGINAL input where the outermost encoded
	// token began, so findings can point at real input bytes (FR-9).
	offset int
}

// tokenDecoder recovers plaintext from a bounded region of text.
type tokenDecoder struct {
	name string
	// candidates returns byte ranges in text that look like this encoding.
	candidates func(text string) [][2]int
	// decode converts one candidate to plaintext, reporting whether it succeeded.
	decode func(token string) (string, bool)
}

// textDecoder transforms an entire text rather than isolated tokens. Reversal and
// character-shift ciphers have no token boundary to key on.
type textDecoder struct {
	name      string
	transform func(text string) (string, bool)
}

// printableRatio reports the fraction of runes in s that are printable ASCII,
// tab, newline, or carriage return. Non-ASCII runes count as printable so that
// recovered non-English text is not discarded.
func printableRatio(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	var printable, total int
	for _, r := range s {
		total++
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			printable++
		case r >= 0x20 && r < 0x7f:
			printable++
		case r > 0x7f && utf8.ValidRune(r) && r != utf8.RuneError:
			printable++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(printable) / float64(total)
}

// looksLikeRecoveredText reports whether decoded output is worth pattern-matching.
func looksLikeRecoveredText(s string) bool {
	return len(s) >= MinDecodedLen && printableRatio(s) >= MinPrintableRatio
}

// --- Candidate scanners ---

// base64Candidates finds tokens in the standard or URL-safe alphabet. Unlike
// highEntropyBase64 this applies no entropy gate: on the injection path the
// evidence is that the token decodes to a matching phrase, so the entropy prior
// that gate encodes is not needed and would exclude short payloads outright — a
// token under 23 characters cannot reach 4.5 bits/char at any content.
func base64Candidates(text string) [][2]int {
	return scanRuns(text, MinCandidateLen, &b64Table)
}

// Alphabet membership tables. A lookup table rather than a closure matters here:
// the scanners walk every byte of the input three times, and an indirect call per
// byte dominates the scan.
var (
	b64Table [256]bool
	b32Table [256]bool
	hexTable [256]bool
)

func init() {
	for c := 0; c < 256; c++ {
		b := byte(c)
		b64Table[c] = b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' ||
			b == '+' || b == '/' || b == '-' || b == '_' || b == '='
		b32Table[c] = b >= 'A' && b <= 'Z' || b >= '2' && b <= '7' || b == '='
		hexTable[c] = b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
	}
}

// base64Shaped rejects tokens that are ordinary words. Base64 of ASCII text
// reliably mixes case or includes digits and symbols; a lowercase-only run is
// prose. This is a cost filter, not a correctness one — a rejected token would
// decode to bytes that fail the printable check anyway.
func base64Shaped(tok string) bool {
	var upper, lower, other bool
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		switch {
		case c >= 'A' && c <= 'Z':
			upper = true
		case c >= 'a' && c <= 'z':
			lower = true
		default:
			other = true
		}
	}
	return other || (upper && lower)
}

func decodeBase64(tok string) (string, bool) {
	if !base64Shaped(tok) {
		return "", false
	}
	body := strings.TrimRight(tok, "=")
	if len(body) < MinCandidateLen {
		return "", false
	}
	enc := base64.StdEncoding
	if strings.ContainsAny(body, "-_") {
		enc = base64.URLEncoding
	}
	padded := body + strings.Repeat("=", (4-len(body)%4)%4)
	out, err := enc.DecodeString(padded)
	if err != nil {
		return "", false
	}
	return string(out), true
}

func base32Candidates(text string) [][2]int {
	return scanRuns(text, 16, &b32Table)
}

func decodeBase32(tok string) (string, bool) {
	body := strings.TrimRight(tok, "=")
	if len(body) < 16 {
		return "", false
	}
	padded := body + strings.Repeat("=", (8-len(body)%8)%8)
	out, err := base32.StdEncoding.DecodeString(padded)
	if err != nil {
		return "", false
	}
	return string(out), true
}

func hexCandidates(text string) [][2]int {
	return scanRuns(text, 16, &hexTable)
}

func decodeHexString(tok string) (string, bool) {
	if len(tok)%2 != 0 {
		tok = tok[:len(tok)-1]
	}
	out, err := hex.DecodeString(tok)
	if err != nil {
		return "", false
	}
	return string(out), true
}

// decodeEscapes expands \xNN and \uNNNN sequences. Both forms are handled by one
// decoder because they appear interchangeably in the same payloads.
func decodeEscapes(tok string) (string, bool) {
	var b strings.Builder
	found := false
	for i := 0; i < len(tok); {
		if tok[i] == '\\' && i+3 < len(tok) && (tok[i+1] == 'x' || tok[i+1] == 'X') {
			if v, err := strconv.ParseUint(tok[i+2:i+4], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 4
				found = true
				continue
			}
		}
		if tok[i] == '\\' && i+5 < len(tok) && (tok[i+1] == 'u' || tok[i+1] == 'U') {
			if v, err := strconv.ParseUint(tok[i+2:i+6], 16, 32); err == nil {
				b.WriteRune(rune(v))
				i += 6
				found = true
				continue
			}
		}
		b.WriteByte(tok[i])
		i++
	}
	return b.String(), found
}

func decodePercent(tok string) (string, bool) {
	if !strings.Contains(tok, "%") {
		return "", false
	}
	out, err := url.QueryUnescape(tok)
	if err != nil || out == tok {
		return "", false
	}
	return out, true
}

func decodeEntities(tok string) (string, bool) {
	if !strings.Contains(tok, "&") {
		return "", false
	}
	out := html.UnescapeString(tok)
	if out == tok {
		return "", false
	}
	return out, true
}

// decodeTags reconstructs ASCII from the Unicode Tags block (U+E0000–U+E007F).
// Each tag character is its ASCII counterpart offset by 0xE0000, which makes a
// run of them an invisible message rather than merely suspicious noise — the
// sanitizer strips these, so detection has to read them first (see FR-32).
func decodeTags(text string) (string, bool) {
	var b strings.Builder
	found := false
	for _, r := range text {
		if r >= 0xE0000 && r <= 0xE007F {
			b.WriteRune(r - 0xE0000)
			found = true
		}
	}
	return b.String(), found
}

// decodeZeroWidthBinary reads runs of zero-width space and zero-width non-joiner
// as binary digits, eight bits to a byte.
func decodeZeroWidthBinary(text string) (string, bool) {
	var bits strings.Builder
	for _, r := range text {
		switch r {
		case '​':
			bits.WriteByte('0')
		case '‌':
			bits.WriteByte('1')
		}
	}
	s := bits.String()
	if len(s) < 16 {
		return "", false
	}
	var out strings.Builder
	for i := 0; i+8 <= len(s); i += 8 {
		v, err := strconv.ParseUint(s[i:i+8], 2, 8)
		if err != nil {
			return "", false
		}
		out.WriteByte(byte(v))
	}
	return out.String(), true
}

// rot13 applies the self-inverse ROT13 substitution.
func rot13(text string) (string, bool) {
	b := []byte(text)
	changed := false
	for i := range b {
		switch {
		case b[i] >= 'a' && b[i] <= 'z':
			b[i] = 'a' + (b[i]-'a'+13)%26
			changed = true
		case b[i] >= 'A' && b[i] <= 'Z':
			b[i] = 'A' + (b[i]-'A'+13)%26
			changed = true
		}
	}
	return string(b), changed
}

// reverseText reverses by rune so multi-byte characters survive.
func reverseText(text string) (string, bool) {
	r := []rune(text)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r), len(r) > 0
}

// scanRuns returns byte ranges of maximal runs of at least min bytes drawn from
// the alphabet table.
func scanRuns(text string, min int, table *[256]bool) [][2]int {
	var out [][2]int
	start := -1
	for i := 0; i < len(text); i++ {
		if table[text[i]] {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 && i-start >= min {
			out = append(out, [2]int{start, i})
		}
		start = -1
	}
	if start >= 0 && len(text)-start >= min {
		out = append(out, [2]int{start, len(text)})
	}
	return out
}

// tokenDecoders are applied to isolated tokens found by their candidate scanner.
var tokenDecoders = []tokenDecoder{
	{name: "base64", candidates: base64Candidates, decode: decodeBase64},
	{name: "base32", candidates: base32Candidates, decode: decodeBase32},
	{name: "hex", candidates: hexCandidates, decode: decodeHexString},
}

// textDecoders transform the whole input. Each costs one additional pattern pass,
// which the anchor prefilter makes cheap when the transform yields nothing
// resembling an injection phrase.
var textDecoders = []textDecoder{
	{name: "escape", transform: func(t string) (string, bool) { return decodeEscapes(t) }},
	{name: "percent", transform: decodePercent},
	{name: "entity", transform: decodeEntities},
	{name: "tags", transform: decodeTags},
	{name: "zero-width", transform: decodeZeroWidthBinary},
	{name: "rot13", transform: rot13},
	{name: "reversed", transform: reverseText},
}

// decodeAll recovers every plaintext reachable from text within the configured
// bounds. Results carry the encoding chain that produced them.
//
// The walk is breadth-first over a visited set of recovered texts. Both matter:
// breadth-first means a text reachable by several routes is reported under the
// shortest chain, and the visited set prunes the identity cycles that self-inverse
// transforms create (rot13 of rot13, reverse of reverse) instead of spending the
// depth budget rediscovering the input.
func decodeAll(text string, b *budget) []decoded {
	type item struct {
		text    string
		chain   string
		depth   int
		origin  int
		encoded int
	}

	var out []decoded
	visited := map[string]bool{text: true}
	queue := []item{{text: text, encoded: len(text)}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= MaxDecodeDepth {
			continue
		}

		// Filters run cheapest-first. Most decode attempts on ordinary prose
		// succeed structurally and yield binary garbage, so rejecting on the
		// printable ratio before touching the budget, the visited map, or the
		// anchor scan is what keeps the decoder's cost proportional to how much
		// of the document is genuinely encoded.
		emit := func(plain, name string, offset int) {
			if len(plain) < MinDecodedLen || len(plain) > MaxChainBytes {
				return
			}
			if cur.encoded > 0 && len(plain) > cur.encoded*MaxExpansionRatio {
				return
			}
			recovered := looksLikeRecoveredText(plain)
			if !recovered && !looksEncoded(plain) {
				return
			}
			if visited[plain] {
				return
			}
			// A recovered text with no pattern anchor cannot produce a pattern
			// finding, and decoding it further is speculative — unless it is
			// itself encoding-shaped, in which case it is an intermediate layer.
			if recovered && !anchored(plain) && !looksLikePII(plain) && !looksEncoded(plain) {
				return
			}
			if !b.spend(len(plain)) {
				return
			}
			visited[plain] = true

			chain := name
			if cur.chain != "" {
				chain = cur.chain + ">" + name
			}
			if recovered {
				out = append(out, decoded{text: plain, chain: chain, offset: offset})
			}
			queue = append(queue, item{
				text:    plain,
				chain:   chain,
				depth:   cur.depth + 1,
				origin:  offset,
				encoded: len(plain),
			})
		}

		for _, d := range tokenDecoders {
			for _, r := range d.candidates(cur.text) {
				plain, ok := d.decode(cur.text[r[0]:r[1]])
				if !ok {
					continue
				}
				emit(plain, d.name, cur.origin+r[0])
			}
		}

		// Whole-text transforms run only against the original input. Each one
		// allocates a full copy of the document, so allowing them to recurse
		// multiplies that cost by the branching factor at every level — measured
		// at 197 ms for a 256 KB document against 56 ms without. Layering a
		// character-shift cipher under a reversal is also not an observed
		// technique; the cost buys nothing.
		if cur.depth == 0 {
			for _, d := range textDecoders {
				plain, ok := d.transform(cur.text)
				if !ok || plain == cur.text {
					continue
				}
				emit(plain, d.name, cur.origin)
			}
		}
	}
	return out
}

// looksEncoded reports whether text is predominantly drawn from an encoding
// alphabet, meaning it is plausibly another layer rather than a dead end. Without
// this a legitimate chain dies at its intermediate: base64 wrapping a hex string
// decodes to hex digits, which carry no pattern anchor and no PII marker.
func looksEncoded(text string) bool {
	if len(text) < MinCandidateLen {
		return false
	}
	var alpha int
	for i := 0; i < len(text); i++ {
		if b64Table[text[i]] {
			alpha++
		}
	}
	return float64(alpha)/float64(len(text)) > 0.9
}

// wholeTextGateBytes is the size above which a decoded text must clear the
// stricter anchor gate before earning a full pattern pass.
//
// Whole-document transforms (rot13, reversal) produce an output the size of the
// input. Several pattern anchors are short, generic literals — "no", "as", "is",
// "you" — which reversed or shifted English prose hits by chance, so the ordinary
// gate lets a full-size garbage document through and each one costs a complete
// pattern pass. Measured: 85.8 ms of a 256 KB DetectDecoded was two such passes.
const wholeTextGateBytes = 4096

// strongAnchors are the anchor literals long enough to be meaningful on their
// own. Derived from the pattern anchor sets rather than hand-listed, so a new
// pattern's anchors are covered automatically.
var strongAnchors = func() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range injectionPatterns {
		for _, group := range p.anchors {
			for _, lit := range group {
				if len(lit) >= 6 && !seen[lit] {
					seen[lit] = true
					out = append(out, lit)
				}
			}
		}
	}
	return out
}()

// anchored reports whether text is worth a full pattern pass. Small decoded
// payloads use the ordinary per-pattern gate; large ones must contain a strong
// anchor, because the pass they would earn costs as much as scanning the original
// document again.
func anchored(text string) bool {
	lower := asciiLower(text)
	if len(text) > wholeTextGateBytes {
		for _, lit := range strongAnchors {
			if strings.Contains(lower, lit) {
				return true
			}
		}
		return false
	}
	for _, p := range preparedPatterns {
		if anchorsPresent(lower, p.anchors) {
			return true
		}
	}
	return false
}

// looksLikePII reports whether text contains a marker any PII pattern requires,
// so decoded secrets survive the anchor prune.
func looksLikePII(text string) bool {
	return strings.ContainsAny(text, "@") ||
		strings.Contains(text, "-----BEGIN") ||
		strings.Contains(text, "sk-") ||
		strings.Contains(text, "Bearer ") ||
		strings.Contains(text, "AKIA") ||
		strings.Contains(text, "AIza")
}

// DetectDecoded decodes obfuscated payloads in text and re-runs pattern and PII
// detection against the recovered plaintext. Findings report the decoded content
// with the encoding chain in Provenance, and the byte offset of the encoded token
// in the caller's input.
//
// This is the injection path. It applies no entropy gate: see base64Candidates.
func DetectDecoded(text string) []Finding {
	var b budget
	var findings []Finding

	// decodeAll already deduplicates by recovered text, reporting each under its
	// shortest chain, so no second pass is needed here.
	for _, d := range decodeAll(text, &b) {
		for _, f := range detectPatternsIn(d.text, d.chain) {
			f.Category = CategoryEncoding
			f.Offset = d.offset
			f.Pattern = "decoded:" + f.Pattern
			findings = append(findings, f)
		}
		for _, f := range DetectPII(d.text) {
			f.Offset = d.offset
			f.Provenance = d.chain
			f.Pattern = "decoded:" + f.Pattern
			findings = append(findings, f)
		}
	}
	return findings
}
