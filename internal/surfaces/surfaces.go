// Package surfaces defines the HiddenSurface type shared by all document
// extractors (HTML, PDF, DOCX, XLSX). A HiddenSurface is a piece of text that
// is invisible to standard content-extraction algorithms (readability,
// document converters) but is present in the raw file and readable by an LLM
// processing the file — making it a viable channel for indirect prompt injection.
package surfaces

// Source constants for each known injection surface.
const (
	// HTML surfaces
	SourceHTMLComment      = "html:comment"       // <!-- ... -->
	SourceHTMLPlaceholder  = "html:placeholder"    // <input placeholder="...">
	SourceHTMLMeta         = "html:meta"           // <meta name/property content="...">
	SourceHTMLNoscript     = "html:noscript"       // <noscript>...</noscript>
	SourceHTMLHiddenInput  = "html:hidden-input"   // <input type="hidden" value="...">
	SourceHTMLAlt          = "html:alt"            // <img alt="...">
	SourceHTMLAriaLabel    = "html:aria-label"     // aria-label="..."
	SourceHTMLTitleAttr    = "html:title-attr"     // title="..." attribute on any element
	SourceHTMLDataAttr     = "html:data-attr"      // data-*="..." custom attributes
	SourceHTMLTextarea     = "html:textarea"       // <textarea>pre-filled</textarea>

	// PDF surfaces
	SourcePDFMetadata   = "pdf:metadata"    // XMP / Info dict fields
	SourcePDFAnnotation = "pdf:annotation"  // /Annot objects with /Contents
	SourcePDFJavaScript = "pdf:javascript"  // /JS action streams
	SourcePDFBookmark   = "pdf:bookmark"    // outline /Title entries
	SourcePDFFormField  = "pdf:form-field"  // /Widget tooltip (/TU) and value (/V)

	// DOCX surfaces
	SourceDOCXProperty  = "docx:property"   // docProps/core.xml and app.xml
	SourceDOCXComment   = "docx:comment"    // word/comments.xml
	SourceDOCXHidden    = "docx:hidden"     // w:hidden text runs
	SourceDOCXFieldCode = "docx:field-code" // w:instrText field instructions

	// XLSX surfaces
	SourceXLSXProperty = "xlsx:property" // docProps/core.xml and app.xml
	SourceXLSXComment  = "xlsx:comment"  // xl/comments*.xml
)

// HiddenSurface holds a single piece of text extracted from a non-visible
// document surface. Source identifies the origin (see Source* constants).
type HiddenSurface struct {
	Source string
	Text   string
}
