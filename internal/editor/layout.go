package editor

import (
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// visualLine is one wrapped line of text on screen. It belongs to exactly
// one block (by blockIdx) and covers a byte range within that block's plain
// text. The cached style is the block-level style applicable to this line
// (font size, base bold flag for headings, etc.).
type visualLine struct {
	blockIdx  int
	text      string
	startByte int     // byte offset within block's plain text
	endByte   int     // exclusive
	x         float32 // left edge of text content
	y         float32 // top edge
	width     float32 // measured pixel width of text
	height    float32 // line box height
	style     doc.BlockStyle
	hardBreak bool
}

// layout produces the line table for the given document at the given width.
func layout(d *doc.Document, contentWidth float32) []visualLine {
	if contentWidth < minContentWidth {
		contentWidth = minContentWidth
	}
	var lines []visualLine
	y := editorVPadding
	for bi, b := range d.Blocks {
		style := doc.StyleForBlock(b)
		blockLines := wrapBlock(bi, b, style, contentWidth)
		for i := range blockLines {
			blockLines[i].x = editorHPadding + style.Indent
			blockLines[i].y = y
			blockLines[i].height = style.LineHeight
			blockLines[i].style = style
			y += style.LineHeight
		}
		lines = append(lines, blockLines...)
		y += style.GapBelow
	}
	return lines
}

func wrapBlock(blockIdx int, b doc.Block, style doc.BlockStyle, width float32) []visualLine {
	text := b.PlainText()
	if text == "" {
		return []visualLine{{blockIdx: blockIdx, text: "", startByte: 0, endByte: 0}}
	}
	var lines []visualLine
	segs := strings.Split(text, "\n")
	offset := 0
	for si, seg := range segs {
		wrapped := wrapLine(blockIdx, seg, offset, style, width)
		if si < len(segs)-1 && len(wrapped) > 0 {
			wrapped[len(wrapped)-1].hardBreak = true
		}
		lines = append(lines, wrapped...)
		offset += len(seg)
		if si < len(segs)-1 {
			offset++
		}
	}
	return lines
}

func wrapLine(blockIdx int, line string, baseOffset int, style doc.BlockStyle, width float32) []visualLine {
	if line == "" {
		return []visualLine{{blockIdx: blockIdx, text: "", startByte: baseOffset, endByte: baseOffset}}
	}
	available := width - 2*editorHPadding - style.Indent
	if available < 50 {
		available = 50
	}
	textStyle := blockTextStyle(style)

	var out []visualLine
	start := 0
	cursor := 0
	for cursor < len(line) {
		wordStart, wordEnd := nextSegment(line, cursor)
		candidate := line[start:wordEnd]
		w := fyne.MeasureText(candidate, style.FontSize, textStyle).Width
		if w <= available || start == wordStart {
			cursor = wordEnd
			continue
		}
		end := wordStart
		for end > start && isWrapBreakable(line[end-1]) {
			end--
		}
		if end == start {
			end = wordStart
		}
		seg := line[start:end]
		segW := fyne.MeasureText(seg, style.FontSize, textStyle).Width
		out = append(out, visualLine{
			blockIdx:  blockIdx,
			text:      seg,
			startByte: baseOffset + start,
			endByte:   baseOffset + end,
			width:     segW,
		})
		start = wordStart
		cursor = wordEnd
	}
	if start < len(line) {
		seg := line[start:]
		segW := fyne.MeasureText(seg, style.FontSize, textStyle).Width
		out = append(out, visualLine{
			blockIdx:  blockIdx,
			text:      seg,
			startByte: baseOffset + start,
			endByte:   baseOffset + len(line),
			width:     segW,
		})
	}
	if len(out) == 0 {
		out = append(out, visualLine{blockIdx: blockIdx, startByte: baseOffset, endByte: baseOffset})
	}
	return out
}

func nextSegment(s string, from int) (int, int) {
	i := from
	for i < len(s) && isWrapBreakable(s[i]) {
		i++
	}
	start := i
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if isWrapBreakableRune(r) {
			break
		}
		i += size
	}
	if start == from {
		return start, i
	}
	return from, i
}

func isWrapBreakable(b byte) bool        { return b == ' ' || b == '\t' }
func isWrapBreakableRune(r rune) bool    { return r == ' ' || r == '\t' }

func lineAtY(lines []visualLine, y float32) int {
	if len(lines) == 0 {
		return -1
	}
	for i, ln := range lines {
		if y >= ln.y && y < ln.y+ln.height {
			return i
		}
	}
	if y < lines[0].y {
		return 0
	}
	return len(lines) - 1
}

// offsetAtX returns the rune offset within the line's text closest to x.
func offsetAtX(line visualLine, x float32) int {
	target := x - line.x
	if target <= 0 {
		return 0
	}
	textStyle := blockTextStyle(line.style)
	text := line.text
	size := line.style.FontSize
	if size == 0 {
		size = fontSize
	}
	if target >= fyne.MeasureText(text, size, textStyle).Width {
		return len(text)
	}
	var prev float32
	for i := 0; i < len(text); {
		_, sz := utf8.DecodeRuneInString(text[i:])
		w := fyne.MeasureText(text[:i+sz], size, textStyle).Width
		if w >= target {
			if w-target < target-prev {
				return i + sz
			}
			return i
		}
		prev = w
		i += sz
	}
	return len(text)
}

func lineForPosition(lines []visualLine, blockIdx, byteOffset int) int {
	last := -1
	for i, ln := range lines {
		if ln.blockIdx != blockIdx {
			if last != -1 {
				return last
			}
			continue
		}
		last = i
		if byteOffset >= ln.startByte && byteOffset < ln.endByte {
			return i
		}
		if byteOffset == ln.endByte {
			if i+1 < len(lines) && lines[i+1].blockIdx == blockIdx && lines[i+1].startByte == ln.endByte {
				continue
			}
			return i
		}
	}
	return last
}

func xForOffset(line visualLine, byteOffset int) float32 {
	if byteOffset <= line.startByte {
		return line.x
	}
	if byteOffset >= line.endByte {
		return line.x + line.width
	}
	local := byteOffset - line.startByte
	textStyle := blockTextStyle(line.style)
	size := line.style.FontSize
	if size == 0 {
		size = fontSize
	}
	return line.x + fyne.MeasureText(line.text[:local], size, textStyle).Width
}

// blockTextStyle returns the base fyne.TextStyle implied by the block style
// alone (no inline marks).
func blockTextStyle(s doc.BlockStyle) fyne.TextStyle {
	return fyne.TextStyle{Bold: s.Bold, Monospace: s.Monospace}
}

// styleRun is one styled segment of a visual line.
type styleRun struct {
	text     string
	marks    doc.Mark
	fontSize float32
	style    fyne.TextStyle
	x        float32
	y        float32
	w        float32
	h        float32
}

// decomposeLine splits a visual line into styled runs by walking the inlines
// that cover its byte range.
func decomposeLine(line visualLine, b doc.Block) []styleRun {
	if len(b.Inlines) == 0 || line.text == "" {
		return nil
	}
	base := blockTextStyle(line.style)
	size := line.style.FontSize
	if size == 0 {
		size = fontSize
	}
	var runs []styleRun
	consumed := 0
	xCursor := line.x
	for _, in := range b.Inlines {
		inlineStart := consumed
		inlineEnd := consumed + len(in.Text)
		consumed = inlineEnd
		start := line.startByte
		if inlineStart > start {
			start = inlineStart
		}
		end := line.endByte
		if inlineEnd < end {
			end = inlineEnd
		}
		if start >= end {
			if inlineStart >= line.endByte {
				break
			}
			continue
		}
		localFrom := start - inlineStart
		localTo := end - inlineStart
		txt := in.Text[localFrom:localTo]
		style := mergeStyle(base, in.Marks)
		w := fyne.MeasureText(txt, size, style).Width
		runs = append(runs, styleRun{
			text:     txt,
			marks:    in.Marks,
			fontSize: size,
			style:    style,
			x:        xCursor,
			y:        line.y,
			w:        w,
			h:        line.height,
		})
		xCursor += w
	}
	return runs
}

// mergeStyle combines the block-level base style with inline marks.
func mergeStyle(base fyne.TextStyle, m doc.Mark) fyne.TextStyle {
	return fyne.TextStyle{
		Bold:      base.Bold || m.Has(doc.MarkBold),
		Italic:    base.Italic || m.Has(doc.MarkItalic),
		Monospace: base.Monospace || m.Has(doc.MarkCode),
	}
}
