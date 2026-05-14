package editor

import (
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// visualLine is one wrapped line of text on screen. It belongs to exactly
// one block (by blockIdx) and covers a byte range within that block's plain
// text, which is also the range we use for caret offset math.
type visualLine struct {
	blockIdx  int
	text      string  // exact characters in this line, no trailing newline
	startByte int     // byte offset within block's plain text
	endByte   int     // byte offset within block's plain text (exclusive)
	x         float32 // left edge (relative to widget origin)
	y         float32 // top edge
	width     float32 // measured pixel width of text
	height    float32 // line box height
	// hardBreak is true when this line was followed by a forced newline (a
	// soft line break inside the source paragraph). False when the break was
	// inserted by word-wrap. M1 treats both identically for navigation; we
	// keep the flag for future use.
	hardBreak bool
}

// layout produces the line table for the given document at the given width.
// It is pure: same inputs always yield the same lines.
func layout(d *doc.Document, contentWidth float32) []visualLine {
	if contentWidth < minContentWidth {
		contentWidth = minContentWidth
	}
	var lines []visualLine
	y := editorVPadding
	for bi, b := range d.Blocks {
		blockLines := wrapBlock(bi, b, contentWidth)
		for i := range blockLines {
			blockLines[i].x = editorHPadding
			blockLines[i].y = y
			blockLines[i].height = lineHeight
			y += lineHeight
		}
		lines = append(lines, blockLines...)
		y += paragraphGap
	}
	return lines
}

// wrapBlock greedily word-wraps the block's plain text into one or more
// visualLines. Empty paragraphs still produce one zero-width line so the
// caret has somewhere to land.
func wrapBlock(blockIdx int, b doc.Block, width float32) []visualLine {
	text := b.PlainText()
	if text == "" {
		return []visualLine{{blockIdx: blockIdx, text: "", startByte: 0, endByte: 0}}
	}

	var lines []visualLine
	// First split on hard line breaks (explicit \n inside the block), then
	// word-wrap each segment.
	segs := strings.Split(text, "\n")
	offset := 0
	for si, seg := range segs {
		wrapped := wrapLine(blockIdx, seg, offset, width)
		// Mark hard break on the last wrapped line of each segment except the
		// final one (since the trailing \n belongs to the next segment).
		if si < len(segs)-1 && len(wrapped) > 0 {
			wrapped[len(wrapped)-1].hardBreak = true
		}
		lines = append(lines, wrapped...)
		offset += len(seg)
		if si < len(segs)-1 {
			offset++ // for the consumed '\n'
		}
	}
	return lines
}

// wrapLine word-wraps a single logical line (no embedded \n). The returned
// visualLines carry byte offsets relative to the *block's* plain text, so
// `baseOffset` is added to whatever local position we compute.
func wrapLine(blockIdx int, line string, baseOffset int, width float32) []visualLine {
	if line == "" {
		return []visualLine{{blockIdx: blockIdx, text: "", startByte: baseOffset, endByte: baseOffset}}
	}
	available := width - 2*editorHPadding
	if available < 50 {
		available = 50
	}

	var out []visualLine
	style := fyne.TextStyle{}
	// Greedy: track the current line as a [start, end) byte range within `line`.
	start := 0
	cursor := 0
	for cursor < len(line) {
		wordStart, wordEnd := nextSegment(line, cursor)
		candidate := line[start:wordEnd]
		w := fyne.MeasureText(candidate, fontSize, style).Width
		if w <= available || start == wordStart {
			// Fits, or we'd produce an empty line — accept and continue.
			cursor = wordEnd
			continue
		}
		// Doesn't fit. Flush [start, wordStart) as a line, start a new one at
		// wordStart (skipping any leading whitespace we just broke on).
		end := wordStart
		for end > start && isWrapBreakable(line[end-1]) {
			end--
		}
		if end == start {
			end = wordStart
		}
		seg := line[start:end]
		segW := fyne.MeasureText(seg, fontSize, style).Width
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
		segW := fyne.MeasureText(seg, fontSize, style).Width
		out = append(out, visualLine{
			blockIdx:  blockIdx,
			text:      seg,
			startByte: baseOffset + start,
			endByte:   baseOffset + len(line),
			width:     segW,
		})
	}
	if len(out) == 0 {
		out = append(out, visualLine{
			blockIdx:  blockIdx,
			startByte: baseOffset,
			endByte:   baseOffset,
		})
	}
	return out
}

// nextSegment returns the byte range [wordStart, wordEnd) describing the
// next "atomic" chunk we consider for wrapping: any whitespace run plus the
// following non-whitespace run. We don't break inside a non-whitespace run
// unless it alone exceeds the line width (handled by the start==wordStart
// guard in wrapLine).
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
		// Pure non-whitespace run from the very start; no leading space.
		return start, i
	}
	return from, i
}

func isWrapBreakable(b byte) bool {
	return b == ' ' || b == '\t'
}

func isWrapBreakableRune(r rune) bool {
	return r == ' ' || r == '\t'
}

// lineAtY returns the index of the visual line containing y, clamped to the
// valid range. Used for click positioning.
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
// Used for click positioning. Walks runes left-to-right measuring cumulative
// width; this is O(n) per line which is fine since lines are short.
func offsetAtX(line visualLine, x float32) int {
	target := x - line.x
	if target <= 0 {
		return 0
	}
	style := fyne.TextStyle{}
	text := line.text
	if target >= fyne.MeasureText(text, fontSize, style).Width {
		return len(text)
	}
	var prev float32
	for i := 0; i < len(text); {
		_, size := utf8.DecodeRuneInString(text[i:])
		w := fyne.MeasureText(text[:i+size], fontSize, style).Width
		if w >= target {
			// Snap to whichever side is closer.
			if w-target < target-prev {
				return i + size
			}
			return i
		}
		prev = w
		i += size
	}
	return len(text)
}

// lineForPosition finds the visual line containing the given caret position
// inside its block. The position's offset is the rune offset within the
// block's plain text.
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
		// Caret sits on the line that contains its offset. End-of-line edge
		// case: an offset equal to a line's endByte belongs to *that* line
		// unless the next line is also in this block and starts at the same
		// byte (i.e. wrap point) — then prefer the next line so the caret
		// shows at the start of the wrapped continuation, matching most
		// editors.
		if byteOffset >= ln.startByte && byteOffset < ln.endByte {
			return i
		}
		if byteOffset == ln.endByte {
			// Look ahead.
			if i+1 < len(lines) && lines[i+1].blockIdx == blockIdx && lines[i+1].startByte == ln.endByte {
				continue
			}
			return i
		}
	}
	return last
}

// xForOffset returns the X coordinate (relative to widget origin) of the
// caret at the given rune offset within the given line.
func xForOffset(line visualLine, byteOffset int) float32 {
	if byteOffset <= line.startByte {
		return line.x
	}
	if byteOffset >= line.endByte {
		return line.x + line.width
	}
	local := byteOffset - line.startByte
	style := fyne.TextStyle{}
	return line.x + fyne.MeasureText(line.text[:local], fontSize, style).Width
}
