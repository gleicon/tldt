// Package extractor extracts hidden injection surfaces from document files
// (DOCX, XLSX, PDF) that are invisible to standard text converters but present
// in raw file bytes and readable by LLMs.
package extractor

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"github.com/gleicon/tldt/internal/surfaces"
)

// ExtractDOCX extracts hidden injection surfaces from a DOCX file (ZIP+XML).
// Scans document properties (core.xml, app.xml), inline comments
// (word/comments.xml), hidden text runs (w:hidden), and field codes (w:instrText).
// Returns nil on unreadable input — callers treat it as no surfaces found.
func ExtractDOCX(data []byte) []surfaces.HiddenSurface {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	var found []surfaces.HiddenSurface
	for _, f := range zr.File {
		switch {
		case f.Name == "docProps/core.xml":
			found = append(found, extractCoreProperties(f, surfaces.SourceDOCXProperty)...)
		case f.Name == "docProps/app.xml":
			found = append(found, extractAppProperties(f, surfaces.SourceDOCXProperty)...)
		case f.Name == "word/comments.xml":
			found = append(found, extractWordComments(f)...)
		case f.Name == "word/document.xml":
			found = append(found, extractWordHiddenAndFields(f)...)
		}
	}
	return found
}

// extractCoreProperties reads dc:title, dc:subject, dc:description, cp:keywords
// from a docProps/core.xml ZIP entry.
func extractCoreProperties(f *zip.File, source string) []surfaces.HiddenSurface {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	return parseCoreXML(rc, source)
}

func parseCoreXML(r io.Reader, source string) []surfaces.HiddenSurface {
	type coreProperties struct {
		Title       string `xml:"title"`
		Subject     string `xml:"subject"`
		Description string `xml:"description"`
		Keywords    string `xml:"keywords"`
		Creator     string `xml:"creator"`
	}
	var cp coreProperties
	if err := xml.NewDecoder(r).Decode(&cp); err != nil {
		return nil
	}
	var found []surfaces.HiddenSurface
	add := func(label, val string) {
		if v := strings.TrimSpace(val); v != "" {
			found = append(found, surfaces.HiddenSurface{Source: source, Text: label + ": " + v})
		}
	}
	add("title", cp.Title)
	add("subject", cp.Subject)
	add("description", cp.Description)
	add("keywords", cp.Keywords)
	add("creator", cp.Creator)
	return found
}

// extractAppProperties reads Application, Manager, Company from docProps/app.xml.
func extractAppProperties(f *zip.File, source string) []surfaces.HiddenSurface {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	type appProperties struct {
		Application string `xml:"Application"`
		Manager     string `xml:"Manager"`
		Company     string `xml:"Company"`
	}
	var ap appProperties
	if err := xml.NewDecoder(rc).Decode(&ap); err != nil {
		return nil
	}
	var found []surfaces.HiddenSurface
	add := func(label, val string) {
		if v := strings.TrimSpace(val); v != "" {
			found = append(found, surfaces.HiddenSurface{Source: source, Text: label + ": " + v})
		}
	}
	add("manager", ap.Manager)
	add("company", ap.Company)
	return found
}

// extractWordComments pulls text from w:comment elements in word/comments.xml.
func extractWordComments(f *zip.File) []surfaces.HiddenSurface {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	return extractWTextNodes(rc, surfaces.SourceDOCXComment)
}

// extractWordHiddenAndFields scans word/document.xml for:
//   - w:t inside w:rPr with w:hidden (hidden text)
//   - w:instrText (field codes like { HYPERLINK ... })
func extractWordHiddenAndFields(f *zip.File) []surfaces.HiddenSurface {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil
	}
	var found []surfaces.HiddenSurface
	found = append(found, extractHiddenRuns(data)...)
	found = append(found, extractFieldCodes(data)...)
	return found
}

// extractWTextNodes extracts all w:t (text) node values from an XML reader.
func extractWTextNodes(r io.Reader, source string) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	dec := xml.NewDecoder(r)
	inText := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			inText = t.Name.Local == "t"
		case xml.CharData:
			if inText {
				if v := strings.TrimSpace(string(t)); v != "" {
					found = append(found, surfaces.HiddenSurface{Source: source, Text: v})
				}
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		}
	}
	return found
}

// extractHiddenRuns finds w:t text inside runs that have w:hidden set.
// Uses a simple state machine on the XML token stream.
func extractHiddenRuns(data []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	dec := xml.NewDecoder(bytes.NewReader(data))
	inRun := false
	inRPr := false
	isHidden := false
	inT := false
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "r":
				inRun = true
				isHidden = false
			case "rPr":
				inRPr = true
			case "hidden":
				if inRPr {
					isHidden = true
				}
			case "t":
				inT = true
				buf.Reset()
			}
		case xml.CharData:
			if inT {
				buf.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "r":
				inRun = false
				isHidden = false
			case "rPr":
				inRPr = false
			case "t":
				if inRun && isHidden {
					if v := strings.TrimSpace(buf.String()); v != "" {
						found = append(found, surfaces.HiddenSurface{Source: surfaces.SourceDOCXHidden, Text: v})
					}
				}
				inT = false
			}
		}
	}
	return found
}

// extractFieldCodes pulls text from w:instrText elements (Word field instructions).
func extractFieldCodes(data []byte) []surfaces.HiddenSurface {
	var found []surfaces.HiddenSurface
	dec := xml.NewDecoder(bytes.NewReader(data))
	inInstr := false
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "instrText" {
				inInstr = true
				buf.Reset()
			}
		case xml.CharData:
			if inInstr {
				buf.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "instrText" {
				if v := strings.TrimSpace(buf.String()); v != "" {
					found = append(found, surfaces.HiddenSurface{Source: surfaces.SourceDOCXFieldCode, Text: v})
				}
				inInstr = false
			}
		}
	}
	return found
}
