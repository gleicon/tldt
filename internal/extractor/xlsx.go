package extractor

import (
	"archive/zip"
	"bytes"
	"regexp"
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
		case strings.HasPrefix(f.Name, "xl/threadedComments") && strings.HasSuffix(f.Name, ".xml"):
			found = append(found, extractXLSXComments(f)...)
		case f.Name == "xl/workbook.xml":
			found = append(found, extractXLSXWorkbook(f)...)
		}
	}
	return found
}

// hiddenSheetRE matches a sheet entry marked hidden or veryHidden. A veryHidden
// sheet cannot even be unhidden through the Excel UI, which makes it a favoured
// place to park text a reviewer will never see but a model parsing the file will.
var hiddenSheetRE = regexp.MustCompile(`<sheet\b[^>]*\bname="([^"]+)"[^>]*\bstate="(hidden|veryHidden)"`)
var hiddenSheetRE2 = regexp.MustCompile(`<sheet\b[^>]*\bstate="(hidden|veryHidden)"[^>]*\bname="([^"]+)"`)

// definedNameRE matches a defined name and its value. Defined names carry formulas
// and constants that never render as cell content.
var definedNameRE = regexp.MustCompile(`(?s)<definedName\b[^>]*\bname="([^"]+)"[^>]*>(.*?)</definedName>`)

// extractXLSXWorkbook reports hidden/veryHidden sheet names and defined names from
// xl/workbook.xml. Row and column hidden flags live in each sheet part; the sheet
// name is the higher-signal target and the one a reviewer acts on.
func extractXLSXWorkbook(f *zip.File) []surfaces.HiddenSurface {
	data, err := readZipEntry(f)
	if err != nil {
		return nil
	}
	var found []surfaces.HiddenSurface
	for _, m := range hiddenSheetRE.FindAllSubmatch(data, -1) {
		found = append(found, surfaces.HiddenSurface{
			Source: surfaces.SourceXLSXHidden,
			Text:   string(m[2]) + " sheet: " + string(m[1]),
		})
	}
	for _, m := range hiddenSheetRE2.FindAllSubmatch(data, -1) {
		found = append(found, surfaces.HiddenSurface{
			Source: surfaces.SourceXLSXHidden,
			Text:   string(m[1]) + " sheet: " + string(m[2]),
		})
	}
	for _, m := range definedNameRE.FindAllSubmatch(data, -1) {
		if v := strings.TrimSpace(string(m[2])); v != "" {
			found = append(found, surfaces.HiddenSurface{
				Source: surfaces.SourceXLSXDefined,
				Text:   string(m[1]) + " = " + v,
			})
		}
	}
	return found
}

// extractXLSXComments reads text from <t> elements in xl/comments*.xml.
func extractXLSXComments(f *zip.File) []surfaces.HiddenSurface {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	return extractWTextNodes(rc, surfaces.SourceXLSXComment)
}
