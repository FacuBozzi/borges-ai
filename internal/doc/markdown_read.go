package doc

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// ParseMarkdown converts a markdown string into our Document model.
// M1 scope: paragraphs only. Headings, lists, etc. fall through as
// paragraph text so nothing is lost on round-trip; M2 will recognize them.
func ParseMarkdown(src string) *Document {
	if strings.TrimSpace(src) == "" {
		return New()
	}
	source := []byte(src)
	root := goldmark.DefaultParser().Parse(text.NewReader(source))

	d := &Document{}
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		switch node := n.(type) {
		case *ast.Paragraph:
			d.Blocks = append(d.Blocks, paragraphFromAST(node, source))
		default:
			// Treat unknown blocks as a paragraph carrying their raw text so
			// content survives M1 round-trip even if we don't render it.
			text := strings.TrimRight(extractRawText(node, source), "\n")
			if text != "" {
				d.Blocks = append(d.Blocks, Block{
					Type:    BlockParagraph,
					Inlines: []Inline{{Text: text}},
				})
			}
		}
	}
	if len(d.Blocks) == 0 {
		return New()
	}
	return d
}

func paragraphFromAST(p *ast.Paragraph, source []byte) Block {
	var sb strings.Builder
	for c := p.FirstChild(); c != nil; c = c.NextSibling() {
		appendInlineText(&sb, c, source)
	}
	return Block{
		Type:    BlockParagraph,
		Inlines: []Inline{{Text: sb.String()}},
	}
}

func appendInlineText(sb *strings.Builder, n ast.Node, source []byte) {
	switch t := n.(type) {
	case *ast.Text:
		sb.Write(t.Segment.Value(source))
		if t.SoftLineBreak() {
			sb.WriteByte('\n')
		}
		if t.HardLineBreak() {
			sb.WriteByte('\n')
		}
	default:
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			appendInlineText(sb, c, source)
		}
	}
}

func extractRawText(n ast.Node, source []byte) string {
	var sb strings.Builder
	if lines := n.Lines(); lines != nil {
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			sb.Write(seg.Value(source))
		}
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		sb.WriteString(extractRawText(c, source))
	}
	return sb.String()
}
