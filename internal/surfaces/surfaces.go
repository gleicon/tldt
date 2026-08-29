// Package surfaces defines the HiddenSurface type shared by all document
// extractors (HTML, PDF, DOCX, XLSX). A HiddenSurface is a piece of text that
// is invisible to standard content-extraction algorithms (readability,
// document converters) but is present in the raw file and readable by an LLM
// processing the file — making it a viable channel for indirect prompt injection.
package surfaces

// Source constants for each known injection surface.
const (
	// HTML surfaces
	SourceHTMLComment     = "html:comment"      // <!-- ... -->
	SourceHTMLPlaceholder = "html:placeholder"  // <input placeholder="...">
	SourceHTMLMeta        = "html:meta"         // <meta name/property content="...">
	SourceHTMLNoscript    = "html:noscript"     // <noscript>...</noscript>
	SourceHTMLHiddenInput = "html:hidden-input" // <input type="hidden" value="...">
	SourceHTMLAlt         = "html:alt"          // <img alt="...">
	SourceHTMLAriaLabel   = "html:aria-label"   // aria-label="..."
	SourceHTMLTitleAttr   = "html:title-attr"   // title="..." attribute on any element
	SourceHTMLDataAttr    = "html:data-attr"    // data-*="..." custom attributes
	SourceHTMLTextarea    = "html:textarea"     // <textarea>pre-filled</textarea>
	SourceHTMLCSSHidden   = "html:css-hidden"   // display:none / visibility:hidden / color==bg
	SourceHTMLJSONLD      = "html:json-ld"      // <script type="application/ld+json">

	// PDF surfaces
	SourcePDFMetadata   = "pdf:metadata"   // XMP / Info dict fields
	SourcePDFJavaScript = "pdf:javascript" // /JS action streams
	SourcePDFAnnotation = "pdf:annotation" // /Annot /Contents text
	SourcePDFAcroForm   = "pdf:acroform"   // /AcroForm field values (/V)
	SourcePDFInvisible  = "pdf:invisible"  // white or sub-4pt content-stream text

	// DOCX surfaces
	SourceDOCXProperty  = "docx:property"   // docProps/core.xml and app.xml
	SourceDOCXComment   = "docx:comment"    // word/comments.xml
	SourceDOCXHidden    = "docx:hidden"     // w:hidden text runs
	SourceDOCXFieldCode = "docx:field-code" // w:instrText field instructions
	SourceDOCXFootnote  = "docx:footnote"   // word/footnotes.xml, word/endnotes.xml
	SourceDOCXHeader    = "docx:header"     // word/header*.xml, word/footer*.xml
	SourceDOCXTextbox   = "docx:textbox"    // w:txbxContent shape text
	SourceDOCXTracked   = "docx:tracked"    // w:del tracked-change deletions
	SourceDOCXCustomXML = "docx:custom-xml" // docProps/custom.xml

	// XLSX surfaces
	SourceXLSXProperty = "xlsx:property" // docProps/core.xml and app.xml
	SourceXLSXComment  = "xlsx:comment"  // xl/comments*.xml
	SourceXLSXHidden   = "xlsx:hidden"   // hidden rows/columns/sheets
	SourceXLSXDefined  = "xlsx:defined"  // defined names (workbook.xml)

	// PPTX surfaces
	SourcePPTXNotes = "pptx:notes" // notesSlideN.xml speaker notes

	// Other document formats
	SourceIPYNB     = "ipynb:hidden" // cell metadata and stored outputs
	SourceMarkdown  = "md:hidden"    // front-matter and HTML comments
	SourceEPUB      = "epub:meta"    // OPF metadata
	SourceEML       = "eml:hidden"   // non-displayed headers, text/html parts
	SourceSVG       = "svg:meta"     // <title>/<desc>/<metadata>
	SourceImageEXIF = "image:exif"   // EXIF/IPTC caption fields
)

// HiddenSurface holds a single piece of text extracted from a non-visible
// document surface. Source identifies the origin (see Source* constants).
type HiddenSurface struct {
	Source string
	Text   string
}
