package extractor

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"github.com/gleicon/tldt/internal/surfaces"
)

// ExtractXLSX extracts hidden injection surfaces from an XLSX file (ZIP+XML).
// Scans document properties (core.xml, app.xml) and cell comments
// (xl/comments*.xml). Returns nil on unreadable input.
func ExtractXLSX(data []byte) []surfaces.HiddenSurface {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	var found []surfaces.HiddenSurface
	for _, f := range zr.File {
		switch {
		case f.Name == "docProps/core.xml":
			found = append(found, extractCoreProperties(f, surfaces.SourceXLSXProperty)...)
		case f.Name == "docProps/app.xml":
			found = append(found, extractAppProperties(f, surfaces.SourceXLSXProperty)...)
		case strings.HasPrefix(f.Name, "xl/comments") && strings.HasSuffix(f.Name, ".xml"):
			found = append(found, extractXLSXComments(f)...)
		}
	}
	return found
}

// extractXLSXComments reads text from <t> elements in xl/comments*.xml.
// Each comment has a <text><r><t>...</t></r></text> structure.
func extractXLSXComments(f *zip.File) []surfaces.HiddenSurface {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	return extractSpreadsheetCommentText(rc)
}

func extractSpreadsheetCommentText(r io.Reader) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	dec := xml.NewDecoder(r)
	inT := false
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inT = true
				buf.Reset()
			}
		case xml.CharData:
			if inT {
				buf.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				if v := strings.TrimSpace(buf.String()); v != "" {
					found = append(found, surfaces.HiddenSurface{Source: surfaces.SourceXLSXComment, Text: v})
				}
				inT = false
			}
		}
	}
	return found
}
