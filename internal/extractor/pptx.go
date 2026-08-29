package extractor

import (
	"archive/zip"
	"bytes"
	"strings"

	"github.com/gleicon/tldt/internal/surfaces"
)

// ExtractPPTX extracts hidden injection surfaces from a PPTX file (ZIP+XML).
//
// Speaker notes are the archetypal presentation trap: invisible on the projected
// slide, fully present in the file, and read by any model summarizing the deck.
// They live in ppt/notesSlides/notesSlideN.xml as ordinary a:t text nodes, so the
// shared node walker handles them. Returns nil on unreadable input.
func ExtractPPTX(data []byte) []surfaces.HiddenSurface {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	var found []surfaces.HiddenSurface
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/notesSlides/notesSlide") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			found = append(found, extractWTextNodes(rc, surfaces.SourcePPTXNotes)...)
			rc.Close()
		}
	}
	return found
}
