package doc

import "strings"

// WriteMarkdown serializes the document to canonical markdown. M1 emits
// only paragraphs; future milestones extend to headings, lists, etc.
//
// Two paragraphs are separated by a blank line. The result always ends with
// a single trailing newline.
func WriteMarkdown(d *Document) string {
	if d == nil || len(d.Blocks) == 0 {
		return "\n"
	}
	var sb strings.Builder
	for i, b := range d.Blocks {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		writeBlock(&sb, b)
	}
	sb.WriteByte('\n')
	return sb.String()
}

func writeBlock(sb *strings.Builder, b Block) {
	switch b.Type {
	case BlockParagraph:
		sb.WriteString(b.PlainText())
	default:
		// M2 will handle other block types; for M1 just emit their text.
		sb.WriteString(b.PlainText())
	}
}
