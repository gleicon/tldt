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
