// Package testutil provides shared test helpers for the tldt test suite.
// It is a regular (non-test) package so test files across packages can import it.
package testutil

import (
	"archive/zip"
	"bytes"
	"strings"

	"github.com/gleicon/tldt/internal/surfaces"
)

// SurfacesOfSource filters surfs to only those with the given source constant.
func SurfacesOfSource(surfs []surfaces.HiddenSurface, source string) []surfaces.HiddenSurface {
	var out []surfaces.HiddenSurface
	for _, s := range surfs {
		if s.Source == source {
			out = append(out, s)
		}
	}
	return out
}

// JoinTexts returns all Text fields from surfs joined by a single space.
func JoinTexts(surfs []surfaces.HiddenSurface) string {
	var parts []string
	for _, s := range surfs {
		parts = append(parts, s.Text)
	}
	return strings.Join(parts, " ")
}

// BuildZIP creates an in-memory ZIP archive from the given name→content map.
func BuildZIP(files map[string]string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, _ := w.Create(name)
		_, _ = f.Write([]byte(content))
	}
	_ = w.Close()
	return buf.Bytes()
}
