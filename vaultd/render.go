package main

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	// No WithUnsafe: raw HTML in notes (e.g. a clipped <script>) is dropped, and
	// dangerous link schemes such as javascript: are not rendered as links.
)

// Links inside the app are hash routes handled by the WAF shell.
func noteHref(p string) string   { return "#/note/" + url.PathEscape(p) }
func folderHref(p string) string { return "#/folder/" + url.PathEscape(p) }
func tagHref(t string) string    { return "#/tag/" + url.PathEscape(t) }

// RenderNote converts an Obsidian note to an HTML fragment.
func (v *Vault) RenderNote(n *Note) string {
	src := v.rewriteWikiLinks(n.Body)
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "<pre>" + html.EscapeString(n.Body) + "</pre>"
	}
	return callouts(buf.String())
}

// rewriteWikiLinks turns [[Target#Heading|Alias]] into Markdown links (or a
// "missing" span), outside fenced code blocks and inline code.
func (v *Vault) rewriteWikiLinks(src string) string {
	lines := strings.Split(src, "\n")
	inFence := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.Contains(line, "[[") {
			continue
		}
		parts := strings.Split(line, "`")
		for j := 0; j < len(parts); j += 2 { // even parts are outside inline code
			parts[j] = wikiLinkRe.ReplaceAllStringFunc(parts[j], func(m string) string {
				s := wikiLinkRe.FindStringSubmatch(m)
				target, heading, alias := s[1], strings.TrimPrefix(s[2], "#"), s[3]
				label := alias
				if label == "" {
					label = strings.TrimSuffix(path.Base(target), ".md")
					if heading != "" {
						label += " › " + heading
					}
				}
				label = escapeMdLabel(label)
				p, ok := v.Resolve(target)
				if !ok {
					return "~~" + label + "~~" // missing note: strikethrough
				}
				if !strings.HasSuffix(strings.ToLower(p), ".md") {
					return "*" + label + "*" // attachment we cannot show
				}
				return "[" + label + "](" + noteHref(p) + ")"
			})
		}
		lines[i] = strings.Join(parts, "`")
	}
	return strings.Join(lines, "\n")
}

func escapeMdLabel(s string) string {
	r := strings.NewReplacer(`[`, `\[`, `]`, `\]`, `|`, `\|`)
	return r.Replace(s)
}

var calloutRe = regexp.MustCompile(`<blockquote>\s*<p>\[!([A-Za-z-]+)\][+-]?[ \t]*([^\n<]*)\n?`)

var calloutNames = map[string]string{
	"note": "Nota", "info": "Info", "tip": "Consejo", "hint": "Consejo", "important": "Importante",
	"warning": "Atención", "caution": "Atención", "attention": "Atención", "danger": "Peligro",
	"error": "Error", "bug": "Bug", "example": "Ejemplo", "quote": "Cita", "cite": "Cita",
	"question": "Pregunta", "faq": "Pregunta", "help": "Pregunta", "success": "Hecho",
	"check": "Hecho", "done": "Hecho", "failure": "Falló", "fail": "Falló", "missing": "Falta",
	"abstract": "Resumen", "summary": "Resumen", "tldr": "Resumen", "todo": "Pendiente",
}

// callouts turns Obsidian callouts (> [!type] Title) into styled boxes.
func callouts(h string) string {
	return calloutRe.ReplaceAllStringFunc(h, func(m string) string {
		s := calloutRe.FindStringSubmatch(m)
		kind := strings.ToLower(s[1])
		title := strings.TrimSpace(s[2])
		if title == "" {
			title = calloutNames[kind]
			if title == "" {
				title = strings.ToUpper(kind[:1]) + kind[1:]
			}
		}
		return fmt.Sprintf(`<blockquote class="callout callout-%s"><p class="callout-title">%s</p><p>`, html.EscapeString(kind), title)
	})
}
