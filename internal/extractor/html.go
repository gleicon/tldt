package extractor

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"

	"github.com/gleicon/tldt/internal/surfaces"
)

// ExtractHTML parses rawHTML and returns all non-visible text surfaces present
// in the raw HTML but stripped by readability: comments, meta, noscript,
// textarea, hidden inputs, placeholders, alt/aria/title attributes, and
// data-* values longer than 20 chars. Returns nil on parse failure.
func ExtractHTML(rawHTML []byte) []surfaces.HiddenSurface {
	doc, err := html.Parse(bytes.NewReader(rawHTML))
	if err != nil {
		return nil
	}
	var found []surfaces.HiddenSurface
	add := func(source, text string) {
		if t := strings.TrimSpace(text); t != "" {
			found = append(found, surfaces.HiddenSurface{Source: source, Text: t})
		}
	}
	attr := func(n *html.Node, key string) string {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val
			}
		}
		return ""
	}
	attrPrefix := func(n *html.Node, prefix string) []html.Attribute {
		var out []html.Attribute
		for _, a := range n.Attr {
			if strings.HasPrefix(a.Key, prefix) {
				out = append(out, a)
			}
		}
		return out
	}
	textContent := func(n *html.Node) string {
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				b.WriteString(c.Data)
			}
		}
		return b.String()
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.CommentNode:
			add(surfaces.SourceHTMLComment, n.Data)

		case html.ElementNode:
			tag := strings.ToLower(n.Data)
			switch tag {
			case "meta":
				nameOrProp := attr(n, "name")
				if nameOrProp == "" {
					nameOrProp = attr(n, "property")
				}
				if content := attr(n, "content"); content != "" {
					skip := map[string]bool{
						"viewport": true, "charset": true, "robots": true,
						"theme-color": true, "msapplication-tilecolor": true,
					}
					if !skip[strings.ToLower(nameOrProp)] {
						add(surfaces.SourceHTMLMeta, nameOrProp+": "+content)
					}
				}

			case "noscript":
				add(surfaces.SourceHTMLNoscript, textContent(n))

			case "textarea":
				add(surfaces.SourceHTMLTextarea, textContent(n))

			case "script":
				// JSON-LD carries structured data a model reads but no reader
				// renders. Other script types are code, not text, and are skipped.
				if strings.EqualFold(attr(n, "type"), "application/ld+json") {
					add(surfaces.SourceHTMLJSONLD, textContent(n))
				}

			case "input":
				if strings.EqualFold(attr(n, "type"), "hidden") {
					if v := attr(n, "value"); v != "" {
						add(surfaces.SourceHTMLHiddenInput, v)
					}
				}
				if ph := attr(n, "placeholder"); ph != "" {
					add(surfaces.SourceHTMLPlaceholder, ph)
				}

			default:
				if ph := attr(n, "placeholder"); ph != "" {
					add(surfaces.SourceHTMLPlaceholder, ph)
				}
			}

			if tag == "img" || tag == "area" {
				if alt := attr(n, "alt"); alt != "" {
					add(surfaces.SourceHTMLAlt, alt)
				}
			}

			if style := attr(n, "style"); style != "" && styleHides(style) {
				if txt := deepText(n); txt != "" {
					add(surfaces.SourceHTMLCSSHidden, txt)
				}
			}

			if v := attr(n, "aria-label"); v != "" {
				add(surfaces.SourceHTMLAriaLabel, v)
			}

			if v := attr(n, "title"); v != "" {
				add(surfaces.SourceHTMLTitleAttr, v)
			}

			// data-* attributes: only include values longer than 20 chars to
			// reduce noise from short identifiers like data-id="abc".
			for _, da := range attrPrefix(n, "data-") {
				if len(strings.TrimSpace(da.Val)) > 20 {
					add(surfaces.SourceHTMLDataAttr, da.Key+"="+da.Val)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

// styleHides reports whether an inline style renders its element's text
// invisible. This is the enumerated supplement to differential extraction
// (DifferentialHTML): it names a reason for the common cases, but the set of CSS
// hiding techniques is unbounded, so it is not the primary detector.
func styleHides(style string) bool {
	s := strings.ToLower(strings.ReplaceAll(style, " ", ""))
	needles := []string{
		"display:none", "visibility:hidden", "opacity:0",
		"font-size:0", "clip-path:inset(100%)",
		"position:absolute;left:-", "text-indent:-",
		"width:0", "height:0", "clip:rect(0,0,0,0)",
	}
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

// deepText collects all descendant text of a node, not just its direct children.
func deepText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
			b.WriteByte(' ')
		}
		for ch := c.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

// VisibleText returns the text a reader would see: all text nodes except those
// inside script, style, head, or an element hidden by an inline style. It is the
// reader-path baseline that DifferentialHTML compares against.
func VisibleText(rawHTML []byte) string {
	doc, err := html.Parse(bytes.NewReader(rawHTML))
	if err != nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, hidden bool) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style", "head", "noscript", "template":
				return
			}
			for _, a := range n.Attr {
				if a.Key == "style" && styleHides(a.Val) {
					hidden = true
				}
				if a.Key == "hidden" {
					hidden = true
				}
			}
		}
		if n.Type == html.TextNode && !hidden {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, hidden)
		}
	}
	walk(doc, false)
	return normalizeSpace(b.String())
}

// AllText returns every text node in the document regardless of visibility.
func AllText(rawHTML []byte) string {
	doc, err := html.Parse(bytes.NewReader(rawHTML))
	if err != nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style":
				// script/style content is code, handled by their own surfaces.
				if !strings.EqualFold(attrOf(n, "type"), "application/ld+json") {
					return
				}
			}
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return normalizeSpace(b.String())
}

func attrOf(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// DifferentialHTML reports text present in the raw document but absent from the
// reader path, as a single css-hidden surface.
//
// This is the primary hidden-text detector (FR-18). Rather than enumerate CSS
// tricks, it takes the set difference between every text node and the text a
// reader sees, so a technique nobody wrote a rule for — a clip-path, an off-screen
// transform, colour matched to the background by a stylesheet — still surfaces.
// styleHides remains as a supplement that can name a reason for the common cases.
func DifferentialHTML(rawHTML []byte) []surfaces.HiddenSurface {
	visible := wordSet(VisibleText(rawHTML))
	all := AllText(rawHTML)

	var hidden []string
	for _, w := range strings.Fields(all) {
		if len(w) < 3 {
			continue
		}
		if !visible[strings.ToLower(w)] {
			hidden = append(hidden, w)
		}
	}
	if len(hidden) == 0 {
		return nil
	}
	return []surfaces.HiddenSurface{{
		Source: surfaces.SourceHTMLCSSHidden,
		Text:   strings.Join(hidden, " "),
	}}
}

func wordSet(text string) map[string]bool {
	m := make(map[string]bool)
	for _, w := range strings.Fields(text) {
		m[strings.ToLower(w)] = true
	}
	return m
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
