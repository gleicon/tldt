package extractor

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/gleicon/tldt/internal/surfaces"
)

// This file holds the lighter-weight extractors for formats beyond the four
// OOXML/PDF documents: notebooks, Markdown, EPUB, email, SVG, and image
// metadata. Each targets the places in its format where text rides along
// invisibly to a human reader but is delivered verbatim to a model.

// --- Jupyter notebooks (.ipynb) ---

// ExtractIPYNB reports injection surfaces in a Jupyter notebook: per-cell metadata
// and stored cell outputs. A reader viewing a rendered notebook sees neither the
// metadata nor, often, the raw output payloads, but a model handed the .ipynb
// reads the whole JSON.
func ExtractIPYNB(data []byte) []surfaces.HiddenSurface {
	var nb struct {
		Cells []struct {
			CellType string          `json:"cell_type"`
			Metadata json.RawMessage `json:"metadata"`
			Source   json.RawMessage `json:"source"`
			Outputs  []struct {
				OutputType string                     `json:"output_type"`
				Text       json.RawMessage            `json:"text"`
				Data       map[string]json.RawMessage `json:"data"`
			} `json:"outputs"`
		} `json:"cells"`
		Metadata json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(data, &nb); err != nil {
		return nil
	}

	var found []surfaces.HiddenSurface
	add := func(text string) {
		if t := strings.TrimSpace(text); len(t) > 3 {
			found = append(found, surfaces.HiddenSurface{Source: surfaces.SourceIPYNB, Text: t})
		}
	}
	if s := metaText(nb.Metadata); s != "" {
		add(s)
	}
	for _, c := range nb.Cells {
		if s := metaText(c.Metadata); s != "" {
			add(s)
		}
		for _, o := range c.Outputs {
			add(jsonStringsJoined(o.Text))
			for mime, raw := range o.Data {
				// text/plain and text/html outputs carry readable text; skip
				// binary image payloads.
				if strings.HasPrefix(mime, "text/") {
					add(jsonStringsJoined(raw))
				}
			}
		}
	}
	return found
}

// metaText pulls the non-empty string values out of a metadata JSON object,
// ignoring structural keys, so a phrase parked in metadata is surfaced.
func metaText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	var parts []string
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch t := v.(type) {
		case string:
			if len(strings.TrimSpace(t)) > 3 {
				parts = append(parts, t)
			}
		case map[string]interface{}:
			for _, vv := range t {
				walk(vv)
			}
		case []interface{}:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(m)
	return strings.Join(parts, " ")
}

// jsonStringsJoined accepts a notebook multiline field, which is either a JSON
// string or an array of strings, and joins it.
func jsonStringsJoined(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		return strings.Join(arr, "")
	}
	return ""
}

// --- Markdown ---

var (
	mdFrontMatterRE = regexp.MustCompile(`(?s)\A---\n(.*?)\n---`)
	mdHTMLCommentRE = regexp.MustCompile(`(?s)<!--(.*?)-->`)
)

// ExtractMarkdown reports YAML front-matter and HTML comments. Rendered Markdown
// drops both: front-matter is consumed as document metadata and HTML comments are
// never displayed, yet a model reading the source sees them.
func ExtractMarkdown(data []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	if m := mdFrontMatterRE.FindSubmatch(data); m != nil {
		if v := strings.TrimSpace(string(m[1])); v != "" {
			found = append(found, surfaces.HiddenSurface{Source: surfaces.SourceMarkdown, Text: v})
		}
	}
	for _, m := range mdHTMLCommentRE.FindAllSubmatch(data, -1) {
		if v := strings.TrimSpace(string(m[1])); len(v) > 3 {
			found = append(found, surfaces.HiddenSurface{Source: surfaces.SourceMarkdown, Text: v})
		}
	}
	return found
}

// --- EPUB ---

var opfMetaRE = regexp.MustCompile(`(?s)<dc:([a-zA-Z]+)[^>]*>([^<]{4,})</dc:`)

// ExtractEPUB reports Dublin Core metadata from the OPF package document inside an
// EPUB (a ZIP). Reading device chrome shows title and author but not description,
// subject, or publisher fields, where a payload can sit unread.
func ExtractEPUB(data []byte) []surfaces.HiddenSurface {
	zdata, ok := zipEntryMatching(data, func(name string) bool {
		return strings.HasSuffix(name, ".opf")
	})
	if !ok {
		return nil
	}
	var found []surfaces.HiddenSurface
	for _, m := range opfMetaRE.FindAllSubmatch(zdata, -1) {
		if v := strings.TrimSpace(string(m[2])); v != "" {
			found = append(found, surfaces.HiddenSurface{
				Source: surfaces.SourceEPUB,
				Text:   string(m[1]) + ": " + v,
			})
		}
	}
	return found
}

// --- Email (.eml) ---

// ExtractEML reports non-displayed headers and the text/html alternative part of
// an email. A mail client renders one body part and a handful of headers; the rest
// of the RFC 822 message — X-* headers, the unshown alternative — travels with the
// file and is read by a model processing it.
func ExtractEML(data []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface

	// Split headers from body on the first blank line.
	idx := bytes.Index(data, []byte("\n\n"))
	if idx < 0 {
		idx = bytes.Index(data, []byte("\r\n\r\n"))
	}
	headerBlock := data
	if idx >= 0 {
		headerBlock = data[:idx]
	}

	for _, line := range strings.Split(string(headerBlock), "\n") {
		line = strings.TrimSpace(line)
		// Report non-standard headers, where injected instructions hide. Standard
		// display headers (From/To/Subject/Date) are shown to the reader.
		if k := strings.SplitN(line, ":", 2); len(k) == 2 {
			key := strings.TrimSpace(k[0])
			if isHiddenHeader(key) {
				if v := strings.TrimSpace(k[1]); len(v) > 3 {
					found = append(found, surfaces.HiddenSurface{
						Source: surfaces.SourceEML,
						Text:   key + ": " + v,
					})
				}
			}
		}
	}

	// The text/html alternative is rendered only when a client prefers HTML; its
	// raw markup carries surfaces of its own.
	if i := bytes.Index(bytes.ToLower(data), []byte("content-type: text/html")); i >= 0 {
		found = append(found, ExtractHTML(data[i:])...)
	}
	return found
}

func isHiddenHeader(key string) bool {
	shown := map[string]bool{
		"from": true, "to": true, "cc": true, "subject": true, "date": true,
		"reply-to": true, "sender": true,
	}
	lk := strings.ToLower(key)
	if shown[lk] {
		return false
	}
	return strings.HasPrefix(lk, "x-") || lk == "comments" || lk == "keywords"
}

// --- SVG ---

var (
	svgTitleRE    = regexp.MustCompile(`(?s)<title[^>]*>([^<]{4,})</title>`)
	svgDescRE     = regexp.MustCompile(`(?s)<desc[^>]*>([^<]{4,})</desc>`)
	svgMetadataRE = regexp.MustCompile(`(?s)<metadata[^>]*>(.*?)</metadata>`)
)

// ExtractSVG reports the title, desc, and metadata elements of a standalone SVG.
// These are accessibility and provenance fields, not rendered graphics, so a model
// reading the SVG source sees text a viewer never draws.
func ExtractSVG(data []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	add := func(text string) {
		if v := strings.TrimSpace(stripTags(text)); len(v) > 3 {
			found = append(found, surfaces.HiddenSurface{Source: surfaces.SourceSVG, Text: v})
		}
	}
	for _, m := range svgTitleRE.FindAllSubmatch(data, -1) {
		add(string(m[1]))
	}
	for _, m := range svgDescRE.FindAllSubmatch(data, -1) {
		add(string(m[1]))
	}
	for _, m := range svgMetadataRE.FindAllSubmatch(data, -1) {
		add(string(m[1]))
	}
	return found
}

var tagRE = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string { return tagRE.ReplaceAllString(s, " ") }

// --- Image EXIF / IPTC ---

// ExtractImageMetadata reports EXIF ImageDescription and IPTC caption text from an
// image. These caption fields are a documented indirect-injection channel: they
// ride inside the file and are read when a model is asked to describe the image,
// but no viewer shows them by default.
//
// The scan is byte-level and format-agnostic rather than a full EXIF parser: it
// locates the ASCII/UTF-8 caption strings that follow the relevant tag markers.
func ExtractImageMetadata(data []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	seen := map[string]bool{}
	add := func(v string) {
		if v = strings.TrimSpace(v); len(v) > 3 && !seen[v] {
			seen[v] = true
			found = append(found, surfaces.HiddenSurface{Source: surfaces.SourceImageEXIF, Text: v})
		}
	}
	// XMP dc:description inside an image's metadata packet.
	for _, m := range opfMetaRE.FindAllSubmatch(data, -1) {
		if strings.EqualFold(string(m[1]), "description") || strings.EqualFold(string(m[1]), "title") {
			add(string(m[2]))
		}
	}
	// IPTC caption/abstract (record 2:120) and EXIF ImageDescription are stored as
	// readable strings; extract printable runs adjacent to their markers.
	for _, marker := range [][]byte{[]byte("ImageDescription"), []byte("caption"), []byte("Caption")} {
		if i := bytes.Index(data, marker); i >= 0 {
			add(printableRun(data[i+len(marker):], 200))
		}
	}
	return found
}

// printableRun returns the leading printable ASCII run of b, up to max bytes,
// skipping leading separators and NUL padding.
func printableRun(b []byte, max int) string {
	start := 0
	for start < len(b) && (b[start] < 0x20 || b[start] == '=' || b[start] == ':' || b[start] == ' ') {
		start++
	}
	end := start
	for end < len(b) && end-start < max && b[end] >= 0x20 && b[end] < 0x7f {
		end++
	}
	return string(b[start:end])
}

// zipEntryMatching returns the first ZIP entry whose name satisfies pred. Used by
// the EPUB extractor to find the OPF package document without knowing its path.
func zipEntryMatching(data []byte, pred func(name string) bool) ([]byte, bool) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, false
	}
	for _, f := range zr.File {
		if pred(f.Name) {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			b, err := io.ReadAll(rc)
			rc.Close()
			if err == nil {
				return b, true
			}
		}
	}
	return nil, false
}
