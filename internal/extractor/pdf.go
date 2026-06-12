package extractor

import (
	"bytes"
	"encoding/xml"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gleicon/tldt/internal/surfaces"
)

// ExtractPDF extracts hidden injection surfaces from a PDF file.
// Scans XMP metadata (embedded XML packet), PDF Info dictionary string values,
// and JavaScript action streams (/JS).
// No external PDF library required — uses byte-level scanning.
// Returns nil on unreadable input.
func ExtractPDF(data []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	found = append(found, extractPDFXMP(data)...)
	found = append(found, extractPDFInfoDict(data)...)
	found = append(found, extractPDFJavaScript(data)...)
	return found
}

// extractPDFXMP finds the XMP metadata packet (<?xpacket ...> ... <?xpacket end>)
// and extracts dc:title, dc:description, dc:subject, xmp:Description, and
// pdf:Keywords fields.
func extractPDFXMP(data []byte) []surfaces.HiddenSurface {
	start := bytes.Index(data, []byte("<?xpacket"))
	if start < 0 {
		return nil
	}
	end := bytes.LastIndex(data, []byte("<?xpacket end"))
	if end <= start {
		return nil
	}
	// Find the closing ?> of the end packet tag
	tail := data[end:]
	closeIdx := bytes.Index(tail, []byte("?>"))
	if closeIdx < 0 {
		return nil
	}
	xmpData := data[start : end+closeIdx+2]

	// XMP uses nested namespaced elements; extract common injection-relevant fields.
	type xmpRDF struct {
		Descriptions []struct {
			Title       string `xml:"title>Alt>li"`
			Description string `xml:"description>Alt>li"`
			Subject     struct {
				Bag []string `xml:"Bag>li"`
			} `xml:"subject"`
			Keywords string `xml:"Keywords"`
			// dc namespace
			DCTitle   string `xml:"http://purl.org/dc/elements/1.1/ title>Alt>li"`
			DCSubject struct {
				Bag []string `xml:"http://purl.org/dc/elements/1.1/ Bag>li"`
			} `xml:"http://purl.org/dc/elements/1.1/ subject"`
			DCDescription string `xml:"http://purl.org/dc/elements/1.1/ description>Alt>li"`
			PDFKeywords   string `xml:"http://ns.adobe.com/pdf/1.3/ Keywords"`
		} `xml:"RDF>Description"`
	}

	var xmp xmpRDF
	if err := xml.Unmarshal(xmpData, &xmp); err != nil {
		// Fall back to raw text extraction from XMP packet
		return extractXMPRaw(xmpData)
	}

	var found []surfaces.HiddenSurface
	add := func(label, val string) {
		if v := strings.TrimSpace(val); v != "" && utf8.ValidString(v) {
			found = append(found, surfaces.HiddenSurface{Source: surfaces.SourcePDFMetadata, Text: label + ": " + v})
		}
	}
	for _, d := range xmp.Descriptions {
		add("title", d.Title)
		add("description", d.Description)
		add("keywords", d.Keywords)
		add("title", d.DCTitle)
		add("description", d.DCDescription)
		add("keywords", d.PDFKeywords)
		for _, s := range d.Subject.Bag {
			add("subject", s)
		}
		for _, s := range d.DCSubject.Bag {
			add("subject", s)
		}
	}
	return found
}

// extractXMPRaw uses simple text extraction when XML unmarshalling fails.
// Finds content between common XMP field tags.
var xmpFieldRE = regexp.MustCompile(`(?s)<(?:dc:|pdf:|xmp:)?(?:title|description|subject|Keywords|Creator)[^>]*>([^<]{4,})<`)

func extractXMPRaw(xmpData []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	for _, m := range xmpFieldRE.FindAllSubmatch(xmpData, -1) {
		if v := strings.TrimSpace(string(m[1])); v != "" && utf8.ValidString(v) {
			found = append(found, surfaces.HiddenSurface{Source: surfaces.SourcePDFMetadata, Text: v})
		}
	}
	return found
}

// pdfStringRE matches PDF literal strings: (text content) in Info dict context.
// Anchored to known Info dict keys to reduce false positives.
var pdfInfoKeyRE = regexp.MustCompile(`/(?:Title|Subject|Author|Keywords|Creator|Producer)\s*\(([^)]{4,})\)`)

// extractPDFInfoDict finds /Title, /Subject, /Author, /Keywords in raw PDF bytes.
// PDF strings may be ASCII or PDFDocEncoding; we skip non-UTF-8 values.
func extractPDFInfoDict(data []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	for _, m := range pdfInfoKeyRE.FindAllSubmatch(data, -1) {
		v := string(m[1])
		if !utf8.ValidString(v) {
			continue
		}
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		found = append(found, surfaces.HiddenSurface{Source: surfaces.SourcePDFMetadata, Text: v})
	}
	return found
}

// pdfJSRE matches /JS (...) or /JS << /S /JavaScript >> stream content markers.
var pdfJSRE = regexp.MustCompile(`/JS\s*\(([^)]{10,})\)`)

// extractPDFJavaScript extracts inline /JS action strings from the raw PDF.
func extractPDFJavaScript(data []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	for _, m := range pdfJSRE.FindAllSubmatch(data, -1) {
		v := string(m[1])
		if !utf8.ValidString(v) {
			continue
		}
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		found = append(found, surfaces.HiddenSurface{Source: surfaces.SourcePDFJavaScript, Text: v})
	}
	return found
}
