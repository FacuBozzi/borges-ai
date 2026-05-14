package doc

import (
	"fmt"
	"strings"
)

// WriteMarkdown serializes the document to canonical markdown.
//
// Block kinds: paragraph, heading, list items (regrouped into bullet/ordered
// lists during write), blockquote, fenced code block, horizontal rule.
// Inline marks: bold, italic, code, strike (and underline via <u>...</u>).
//
// Blocks are separated by a blank line; the result always ends with a
// single trailing newline.
func WriteMarkdown(d *Document) string {
	if d == nil || len(d.Blocks) == 0 {
		if d != nil {
			if meta := writeMeta(d.Meta); meta != "" {
				return "---\n" + meta + "---\n\n\n"
			}
		}
		return "\n"
	}
	var sb strings.Builder
	if meta := writeMeta(d.Meta); meta != "" {
		sb.WriteString("---\n")
		sb.WriteString(meta)
		sb.WriteString("---\n\n")
	}
	i := 0
	for i < len(d.Blocks) {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		b := d.Blocks[i]
		if b.Type == BlockListItem {
			// Consume the contiguous run of list items belonging to the
			// same list (matching depth + kind) and emit them as one list.
			end := i + 1
			for end < len(d.Blocks) && sameList(b, d.Blocks[end]) {
				end++
			}
			writeListRun(&sb, d.Blocks[i:end])
			i = end
			continue
		}
		writeBlock(&sb, b, 0)
		i++
	}
	sb.WriteByte('\n')
	return sb.String()
}

func sameList(a, b Block) bool {
	if a.Type != BlockListItem || b.Type != BlockListItem {
		return false
	}
	if listKind(a) != listKind(b) {
		return false
	}
	// Items at greater depth are considered part of the same enclosing list
	// (they print indented). Only a same-kind item at strictly lesser depth
	// or a totally different block ends the run.
	return listDepth(b) >= listDepth(a)
}

func listKind(b Block) string {
	if v, ok := b.Meta["list_kind"].(string); ok {
		return v
	}
	return "bullet"
}

func listDepth(b Block) int {
	if v, ok := b.Meta["depth"].(int); ok {
		return v
	}
	return 0
}

func listIndex(b Block) int {
	if v, ok := b.Meta["index"].(int); ok && v > 0 {
		return v
	}
	return 1
}

func writeListRun(sb *strings.Builder, items []Block) {
	for i, it := range items {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(strings.Repeat("  ", listDepth(it)))
		if listKind(it) == "ordered" {
			fmt.Fprintf(sb, "%d. ", listIndex(it))
		} else {
			sb.WriteString("- ")
		}
		writeInlines(sb, it.Inlines)
	}
}

// writeBlock emits one block at the given indent depth. Depth is used by
// nested lists.
func writeBlock(sb *strings.Builder, b Block, depth int) {
	indent := strings.Repeat("  ", depth)
	switch b.Type {
	case BlockHeading:
		level := HeadingLevel(b)
		if level < 1 {
			level = 1
		}
		sb.WriteString(strings.Repeat("#", level))
		sb.WriteByte(' ')
		writeInlines(sb, b.Inlines)
	case BlockBulletList, BlockOrderedList:
		// Container-style lists (legacy) — flatten and write.
		writeListRun(sb, b.Children)
	case BlockListItem:
		// Standalone list item — write as a single item.
		writeListRun(sb, []Block{b})
	case BlockQuote:
		writeQuote(sb, b)
	case BlockCodeBlock:
		writeCodeBlock(sb, b)
	case BlockHR:
		sb.WriteString("---")
	default:
		sb.WriteString(indent)
		writeInlines(sb, b.Inlines)
	}
}

func writeInlines(sb *strings.Builder, inlines []Inline) {
	for _, in := range inlines {
		writeInline(sb, in)
	}
}

func writeQuote(sb *strings.Builder, b Block) {
	var lineBuf strings.Builder
	writeInlines(&lineBuf, b.Inlines)
	for i, ln := range strings.Split(lineBuf.String(), "\n") {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString("> ")
		sb.WriteString(ln)
	}
}

func writeCodeBlock(sb *strings.Builder, b Block) {
	lang := ""
	if v, ok := b.Meta["lang"].(string); ok {
		lang = v
	}
	sb.WriteString("```")
	sb.WriteString(lang)
	sb.WriteByte('\n')
	sb.WriteString(b.PlainText())
	if !strings.HasSuffix(b.PlainText(), "\n") {
		sb.WriteByte('\n')
	}
	sb.WriteString("```")
}

// writeInline emits one styled run. Mark order (outside to inside):
// strike → bold → italic → underline → code.
func writeInline(sb *strings.Builder, in Inline) {
	if in.Text == "" {
		return
	}
	openClose := func(marker string, has bool) (string, string) {
		if has {
			return marker, marker
		}
		return "", ""
	}
	soS, soE := openClose("~~", in.Marks.Has(MarkStrike))
	bS, bE := openClose("**", in.Marks.Has(MarkBold))
	iS, iE := openClose("*", in.Marks.Has(MarkItalic))
	uS, uE := "", ""
	if in.Marks.Has(MarkUnderline) {
		uS, uE = "<u>", "</u>"
	}
	cS, cE := openClose("`", in.Marks.Has(MarkCode))
	sb.WriteString(soS + bS + iS + uS + cS + in.Text + cE + uE + iE + bE + soE)
}
