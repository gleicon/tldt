package extractor

import (
	"archive/zip"
	"bytes"
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
func extractXLSXComments(f *zip.File) []surfaces.HiddenSurface {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	return extractWTextNodes(rc, surfaces.SourceXLSXComment)
}
