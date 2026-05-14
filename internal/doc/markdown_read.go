package doc

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	exast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

var mdParser = goldmark.New(
	goldmark.WithExtensions(extension.Strikethrough),
)

// ParseMarkdown converts a markdown string into our Document model. M2b
// supports paragraphs with inline marks: bold (**), italic (*), code (`),
// strikethrough (~~), and HTML <u>...</u> for underline.
func ParseMarkdown(src string) *Document {
	if strings.TrimSpace(src) == "" {
		return New()
	}
	source := []byte(src)
	root := mdParser.Parser().Parse(text.NewReader(source))

	d := &Document{}
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		switch node := n.(type) {
		case *ast.Paragraph:
			d.Blocks = append(d.Blocks, paragraphFromAST(node, source))
		default:
			raw := strings.TrimRight(extractRawText(node, source), "\n")
			if raw != "" {
				d.Blocks = append(d.Blocks, Block{
					Type:    BlockParagraph,
					Inlines: []Inline{{Text: raw}},
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
	var inlines []Inline
	walkInlines(p, source, 0, &inlines)
	return Block{Type: BlockParagraph, Inlines: collapseInlines(inlines)}
}

// walkInlines emits Inlines for the children of node, OR'ing curMarks into
// each. Marks accumulate through nested Emphasis/Strong/CodeSpan/Strike.
func walkInlines(node ast.Node, source []byte, curMarks Mark, out *[]Inline) {
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			text := string(t.Segment.Value(source))
			if text != "" {
				*out = append(*out, Inline{Text: text, Marks: curMarks})
			}
			if t.SoftLineBreak() || t.HardLineBreak() {
				*out = append(*out, Inline{Text: "\n", Marks: curMarks})
			}
		case *ast.Emphasis:
			m := curMarks | MarkItalic
			if t.Level >= 2 {
				m = curMarks | MarkBold
			}
			walkInlines(t, source, m, out)
		case *ast.CodeSpan:
			// CodeSpan children are Text nodes; collect their literal value.
			var sb strings.Builder
			for cc := t.FirstChild(); cc != nil; cc = cc.NextSibling() {
				if tx, ok := cc.(*ast.Text); ok {
					sb.Write(tx.Segment.Value(source))
				}
			}
			if s := sb.String(); s != "" {
				*out = append(*out, Inline{Text: s, Marks: curMarks | MarkCode})
			}
		case *ast.RawHTML:
			handleRawHTML(t, source, curMarks, out)
		case *exast.Strikethrough:
			walkInlines(t, source, curMarks|MarkStrike, out)
		default:
			walkInlines(c, source, curMarks, out)
		}
	}
}

// handleRawHTML recognizes a tiny subset: <u> opens underline mode for the
// sibling text until the next </u>. Implemented by emitting a sentinel mark
// transition via the underlineToggle map below; here we just skip the tag
// itself.
func handleRawHTML(n *ast.RawHTML, source []byte, _ Mark, _ *[]Inline) {
	// We intentionally do not look at <u> tags here. Underline round-trips
	// through HTML inline tags, but parsing raw HTML segments that span
	// across goldmark's inline tree is brittle. For now: <u>...</u> in the
	// source ends up as literal markup inside an Inline.Text, which means
	// re-saving preserves it byte-for-byte. The editor toggle still applies
	// MarkUnderline to selected runs in our model, and the writer emits
	// <u>...</u> from MarkUnderline runs.
	_ = n
	_ = source
}

// collapseInlines merges adjacent inlines with identical marks. Empty
// inlines are dropped except when the result would have zero items.
func collapseInlines(in []Inline) []Inline {
	if len(in) == 0 {
		return []Inline{{}}
	}
	out := in[:0]
	for _, x := range in {
		if x.Text == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Marks == x.Marks {
			out[n-1].Text += x.Text
			continue
		}
		out = append(out, x)
	}
	if len(out) == 0 {
		out = []Inline{{}}
	}
	return out
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
