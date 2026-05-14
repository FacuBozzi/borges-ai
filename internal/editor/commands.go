package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// Mutators are private helpers operating on the editor while the caller
// already holds e.mu. They keep the doc + selection consistent and signal
// "changed". Callers should call e.invalidate() afterwards.
//
// M2a paragraph-only assumption: every block has exactly one inline at index
// 0, so Position.Offset is the byte offset within the block's plain text.
// This lifts in M2b when multi-inline support lands.

func (e *RichEditor) blockText(blockIdx int) string {
	if blockIdx < 0 || blockIdx >= len(e.doc.Blocks) {
		return ""
	}
	if len(e.doc.Blocks[blockIdx].Inlines) == 0 {
		return ""
	}
	return e.doc.Blocks[blockIdx].Inlines[0].Text
}

func (e *RichEditor) setBlockText(blockIdx int, text string) {
	if blockIdx < 0 || blockIdx >= len(e.doc.Blocks) {
		return
	}
	if len(e.doc.Blocks[blockIdx].Inlines) == 0 {
		e.doc.Blocks[blockIdx].Inlines = []doc.Inline{{Text: text}}
		return
	}
	e.doc.Blocks[blockIdx].Inlines[0].Text = text
}

// selRange returns the normalized (lower, upper) endpoints of the selection.
// If collapsed, lower == upper.
func (e *RichEditor) selRange() (doc.Position, doc.Position) {
	if positionLess(e.sel.Head, e.sel.Anchor) {
		return e.sel.Head, e.sel.Anchor
	}
	return e.sel.Anchor, e.sel.Head
}

// selectionText returns the plain text contents of the current selection.
// Used by Copy/Cut. Block boundaries are emitted as "\n\n".
func (e *RichEditor) selectionText() string {
	if e.sel.IsCollapsed() {
		return ""
	}
	lo, hi := e.selRange()
	if lo.Path[0] == hi.Path[0] {
		return e.blockText(lo.Path[0])[lo.Offset:hi.Offset]
	}
	var sb strings.Builder
	sb.WriteString(e.blockText(lo.Path[0])[lo.Offset:])
	for i := lo.Path[0] + 1; i < hi.Path[0]; i++ {
		sb.WriteString("\n\n")
		sb.WriteString(e.blockText(i))
	}
	sb.WriteString("\n\n")
	sb.WriteString(e.blockText(hi.Path[0])[:hi.Offset])
	return sb.String()
}

// deleteSelection removes the selected range and collapses the caret at the
// start. Safe to call when collapsed (no-op).
func (e *RichEditor) deleteSelection() {
	if e.sel.IsCollapsed() {
		return
	}
	lo, hi := e.selRange()
	if lo.Path[0] == hi.Path[0] {
		bi := lo.Path[0]
		t := e.blockText(bi)
		e.setBlockText(bi, t[:lo.Offset]+t[hi.Offset:])
		e.setCaret(lo)
		e.preferredX = -1
		return
	}
	loText := e.blockText(lo.Path[0])
	hiText := e.blockText(hi.Path[0])
	merged := loText[:lo.Offset] + hiText[hi.Offset:]
	e.setBlockText(lo.Path[0], merged)
	// Remove blocks (lo+1 .. hi] inclusive of hi.
	e.doc.Blocks = append(e.doc.Blocks[:lo.Path[0]+1], e.doc.Blocks[hi.Path[0]+1:]...)
	e.setCaret(lo)
	e.preferredX = -1
}

// insertRune inserts r, replacing any current selection.
func (e *RichEditor) insertRune(r rune) {
	e.deleteSelection()
	c := e.sel.Head
	bi := c.Path[0]
	text := e.blockText(bi)
	off := clamp(c.Offset, 0, len(text))
	buf := make([]byte, utf8.UTFMax)
	n := utf8.EncodeRune(buf, r)
	e.setBlockText(bi, text[:off]+string(buf[:n])+text[off:])
	e.setCaret(doc.Position{Path: []int{bi}, Inline: 0, Offset: off + n})
	e.preferredX = -1
}

// insertString inserts a literal string (newlines split into new blocks),
// replacing any current selection.
func (e *RichEditor) insertString(s string) {
	e.deleteSelection()
	if s == "" {
		return
	}
	// Normalize line endings to '\n' and split on blank-line block separators
	// ("\n\n") so pasted multi-paragraph text lands as separate blocks.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.Split(s, "\n\n")
	for i, part := range parts {
		if i > 0 {
			e.splitBlock()
		}
		// Within a part, treat single \n as a hard break inside the same
		// paragraph by inserting a literal newline (M2c will refine this for
		// list items and code blocks).
		e.insertStringNoNewline(part)
	}
}

// insertStringNoNewline inserts s at the caret with no block splitting.
// Newlines in s are inserted verbatim into the block's text.
func (e *RichEditor) insertStringNoNewline(s string) {
	c := e.sel.Head
	bi := c.Path[0]
	text := e.blockText(bi)
	off := clamp(c.Offset, 0, len(text))
	e.setBlockText(bi, text[:off]+s+text[off:])
	e.setCaret(doc.Position{Path: []int{bi}, Inline: 0, Offset: off + len(s)})
	e.preferredX = -1
}

// splitBlock breaks the current block at the caret, putting everything after
// the caret into a new paragraph block. Used for Enter.
func (e *RichEditor) splitBlock() {
	e.deleteSelection()
	c := e.sel.Head
	bi := c.Path[0]
	text := e.blockText(bi)
	off := clamp(c.Offset, 0, len(text))
	before, after := text[:off], text[off:]
	e.setBlockText(bi, before)
	newBlock := doc.Block{Type: doc.BlockParagraph, Inlines: []doc.Inline{{Text: after}}}
	e.doc.Blocks = insertBlock(e.doc.Blocks, bi+1, newBlock)
	e.setCaret(doc.Position{Path: []int{bi + 1}, Inline: 0, Offset: 0})
	e.preferredX = -1
}

// backspace deletes the selection if any, else one rune before the caret,
// else merges with the previous block.
func (e *RichEditor) backspace() {
	if !e.sel.IsCollapsed() {
		e.deleteSelection()
		return
	}
	c := e.sel.Head
	bi := c.Path[0]
	text := e.blockText(bi)
	if c.Offset > 0 {
		_, size := utf8.DecodeLastRuneInString(text[:c.Offset])
		e.setBlockText(bi, text[:c.Offset-size]+text[c.Offset:])
		e.setCaret(doc.Position{Path: []int{bi}, Inline: 0, Offset: c.Offset - size})
		e.preferredX = -1
		return
	}
	if bi == 0 {
		return
	}
	prevText := e.blockText(bi - 1)
	mergedOffset := len(prevText)
	e.setBlockText(bi-1, prevText+text)
	e.doc.Blocks = removeBlock(e.doc.Blocks, bi)
	e.setCaret(doc.Position{Path: []int{bi - 1}, Inline: 0, Offset: mergedOffset})
	e.preferredX = -1
}

// deleteForward deletes the selection if any, else one rune at the caret,
// else merges with the next block.
func (e *RichEditor) deleteForward() {
	if !e.sel.IsCollapsed() {
		e.deleteSelection()
		return
	}
	c := e.sel.Head
	bi := c.Path[0]
	text := e.blockText(bi)
	if c.Offset < len(text) {
		_, size := utf8.DecodeRuneInString(text[c.Offset:])
		e.setBlockText(bi, text[:c.Offset]+text[c.Offset+size:])
		e.preferredX = -1
		return
	}
	if bi+1 >= len(e.doc.Blocks) {
		return
	}
	nextText := e.blockText(bi + 1)
	e.setBlockText(bi, text+nextText)
	e.doc.Blocks = removeBlock(e.doc.Blocks, bi+1)
	e.preferredX = -1
}

// positionLess reports whether a precedes b in document order.
func positionLess(a, b doc.Position) bool {
	// Compare paths element-wise.
	for i := 0; i < len(a.Path) && i < len(b.Path); i++ {
		if a.Path[i] != b.Path[i] {
			return a.Path[i] < b.Path[i]
		}
	}
	if len(a.Path) != len(b.Path) {
		return len(a.Path) < len(b.Path)
	}
	if a.Inline != b.Inline {
		return a.Inline < b.Inline
	}
	return a.Offset < b.Offset
}

func insertBlock(blocks []doc.Block, at int, b doc.Block) []doc.Block {
	blocks = append(blocks, doc.Block{})
	copy(blocks[at+1:], blocks[at:])
	blocks[at] = b
	return blocks
}

func removeBlock(blocks []doc.Block, at int) []doc.Block {
	if at < 0 || at >= len(blocks) {
		return blocks
	}
	return append(blocks[:at], blocks[at+1:]...)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
