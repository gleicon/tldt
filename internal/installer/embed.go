package installer

import "embed"

// EmbeddedFiles holds the skill and hook template files compiled into the binary.
// The skills/, hooks/, and codex/ directories are siblings of this file in
// internal/installer/. embed directives do not support .. path components, so
// templates must live here.
//
//go:embed skills hooks codex
var EmbeddedFiles embed.FS
