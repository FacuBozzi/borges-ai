package doc

import "strings"

// WriteMarkdown serializes the document to canonical markdown with inline
// marks. Block kinds beyond paragraph fall back to plain text for now (M2c).
//
// Paragraphs are separated by a blank line; the result always ends with a
// single trailing newline.
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
	for _, in := range b.Inlines {
		writeInline(sb, in)
	}
}

// writeInline emits one styled run. Mark order (outside to inside):
// strike → bold → italic → underline → code. Code is innermost because its
// content can't contain markdown emphasis. Underline goes through HTML
// inline tags.
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
	soStart, soEnd := openClose("~~", in.Marks.Has(MarkStrike))
	boldStart, boldEnd := openClose("**", in.Marks.Has(MarkBold))
	italicStart, italicEnd := openClose("*", in.Marks.Has(MarkItalic))
	ulStart, ulEnd := "", ""
	if in.Marks.Has(MarkUnderline) {
		ulStart, ulEnd = "<u>", "</u>"
	}
	codeStart, codeEnd := openClose("`", in.Marks.Has(MarkCode))

	sb.WriteString(soStart)
	sb.WriteString(boldStart)
	sb.WriteString(italicStart)
	sb.WriteString(ulStart)
	sb.WriteString(codeStart)
	sb.WriteString(in.Text)
	sb.WriteString(codeEnd)
	sb.WriteString(ulEnd)
	sb.WriteString(italicEnd)
	sb.WriteString(boldEnd)
	sb.WriteString(soEnd)
}
