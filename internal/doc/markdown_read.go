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

// ParseMarkdown converts a markdown string into our Document model.
// M2c supports: paragraphs, headings, bullet/ordered lists, blockquote,
// fenced + indented code blocks, horizontal rule. Inline marks: bold,
// italic, code, strike. Underline tags pass through as literal text.
func ParseMarkdown(src string) *Document {
	front, body := splitFrontMatter(src)
	meta := parseMeta(front)
	if strings.TrimSpace(body) == "" {
		d := New()
		d.Meta = meta
		return d
	}
	source := []byte(body)
	root := mdParser.Parser().Parse(text.NewReader(source))

	d := &Document{Meta: meta}
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		d.Blocks = append(d.Blocks, blocksFromAST(n, source)...)
	}
	if len(d.Blocks) == 0 {
		empty := New()
		empty.Meta = meta
		return empty
	}
	return d
}

// blocksFromAST converts one top-level AST node into one or more of our
// Blocks. Lists contribute one container block; everything else contributes
// a single block.
func blocksFromAST(n ast.Node, source []byte) []Block {
	switch node := n.(type) {
	case *ast.Paragraph:
		return []Block{paragraphFromAST(node, source)}
	case *ast.Heading:
		return []Block{headingFromAST(node, source)}
	case *ast.List:
		return listBlocksFromAST(node, source, 0)
	case *ast.Blockquote:
		return []Block{quoteFromAST(node, source)}
	case *ast.FencedCodeBlock:
		return []Block{codeBlockFromAST(node, source, true)}
	case *ast.CodeBlock:
		return []Block{codeBlockFromAST(node, source, false)}
	case *ast.ThematicBreak:
		return []Block{{Type: BlockHR, Inlines: []Inline{{}}}}
	default:
		raw := strings.TrimRight(extractRawText(node, source), "\n")
		if raw == "" {
			return nil
		}
		return []Block{{Type: BlockParagraph, Inlines: []Inline{{Text: raw}}}}
	}
}

func paragraphFromAST(p *ast.Paragraph, source []byte) Block {
	var inlines []Inline
	walkInlines(p, source, 0, &inlines)
	return Block{Type: BlockParagraph, Inlines: collapseInlines(inlines)}
}

func headingFromAST(h *ast.Heading, source []byte) Block {
	var inlines []Inline
	walkInlines(h, source, 0, &inlines)
	return Block{
		Type:    BlockHeading,
		Inlines: collapseInlines(inlines),
		Meta:    map[string]any{"level": h.Level},
	}
}

// listBlocksFromAST flattens an AST list (possibly nested) into a sequence
// of top-level BlockListItem blocks. Each item carries Meta:
//   - "list_kind": "bullet" | "ordered"
//   - "depth":     int (0 for top-level lists, +1 per nesting)
//   - "index":     int (1-based; only set for ordered lists)
//
// This keeps the layout layer flat — no recursive walk needed for
// rendering — while the writer reconstitutes the original markdown by
// grouping consecutive items.
func listBlocksFromAST(l *ast.List, source []byte, depth int) []Block {
	kind := "bullet"
	startIdx := 1
	if l.IsOrdered() {
		kind = "ordered"
		startIdx = l.Start
		if startIdx <= 0 {
			startIdx = 1
		}
	}
	var out []Block
	i := 0
	for c := l.FirstChild(); c != nil; c = c.NextSibling() {
		li, ok := c.(*ast.ListItem)
		if !ok {
			continue
		}
		item := listItemFromAST(li, source, kind, depth, startIdx+i)
		out = append(out, item.item)
		out = append(out, item.nested...)
		i++
	}
	return out
}

type listItemResult struct {
	item   Block
	nested []Block
}

func listItemFromAST(li *ast.ListItem, source []byte, kind string, depth, index int) listItemResult {
	var inlines []Inline
	var nested []Block
	for c := li.FirstChild(); c != nil; c = c.NextSibling() {
		switch cc := c.(type) {
		case *ast.Paragraph, *ast.TextBlock:
			walkInlines(cc, source, 0, &inlines)
		case *ast.List:
			nested = append(nested, listBlocksFromAST(cc, source, depth+1)...)
		default:
			walkInlines(cc, source, 0, &inlines)
		}
	}
	meta := map[string]any{
		"list_kind": kind,
		"depth":     depth,
	}
	if kind == "ordered" {
		meta["index"] = index
	}
	return listItemResult{
		item: Block{Type: BlockListItem, Inlines: collapseInlines(inlines), Meta: meta},
		nested: nested,
	}
}

func quoteFromAST(q *ast.Blockquote, source []byte) Block {
	// Flatten the quote into a single block whose inlines are the
	// concatenated text of its inner paragraphs (separated by '\n').
	var inlines []Inline
	first := true
	for c := q.FirstChild(); c != nil; c = c.NextSibling() {
		if !first {
			inlines = append(inlines, Inline{Text: "\n"})
		}
		first = false
		walkInlines(c, source, 0, &inlines)
	}
	return Block{Type: BlockQuote, Inlines: collapseInlines(inlines)}
}

func codeBlockFromAST(n ast.Node, source []byte, fenced bool) Block {
	var sb strings.Builder
	lines := n.Lines()
	if lines != nil {
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			sb.Write(seg.Value(source))
		}
	}
	body := strings.TrimRight(sb.String(), "\n")
	meta := map[string]any{"fenced": fenced}
	if fcb, ok := n.(*ast.FencedCodeBlock); ok {
		if lang := string(fcb.Language(source)); lang != "" {
			meta["lang"] = lang
		}
	}
	return Block{Type: BlockCodeBlock, Inlines: []Inline{{Text: body}}, Meta: meta}
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
			var sb strings.Builder
			for cc := t.FirstChild(); cc != nil; cc = cc.NextSibling() {
				if tx, ok := cc.(*ast.Text); ok {
					sb.Write(tx.Segment.Value(source))
				}
			}
			if s := sb.String(); s != "" {
				*out = append(*out, Inline{Text: s, Marks: curMarks | MarkCode})
			}
		case *exast.Strikethrough:
			walkInlines(t, source, curMarks|MarkStrike, out)
		default:
			walkInlines(c, source, curMarks, out)
		}
	}
}

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
