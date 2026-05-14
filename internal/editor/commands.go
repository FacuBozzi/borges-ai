package editor

import (
	"unicode/utf8"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// Mutators are private helpers that operate on the editor while the caller
// already holds e.mu. They keep the doc and caret consistent and signal
// "changed". Callers should call e.invalidate() afterwards.
//
// M1 paragraph-only assumption: every block has exactly one inline at index
// 0. Position.Offset is therefore the byte offset within the block's plain
// text, which simplifies all the math below.

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

// insertRune inserts r at the caret. Caller holds e.mu.
func (e *RichEditor) insertRune(r rune) {
	bi := e.caret.Path[0]
	text := e.blockText(bi)
	off := clamp(e.caret.Offset, 0, len(text))
	buf := make([]byte, utf8.UTFMax)
	n := utf8.EncodeRune(buf, r)
	e.setBlockText(bi, text[:off]+string(buf[:n])+text[off:])
	e.caret.Offset = off + n
	e.preferredX = -1
}

// insertString inserts a literal string (no newlines) at the caret.
func (e *RichEditor) insertString(s string) {
	bi := e.caret.Path[0]
	text := e.blockText(bi)
	off := clamp(e.caret.Offset, 0, len(text))
	e.setBlockText(bi, text[:off]+s+text[off:])
	e.caret.Offset = off + len(s)
	e.preferredX = -1
}

// splitBlock breaks the current block at the caret, putting everything after
// the caret into a new paragraph block right after the current one. Used for
// Enter.
func (e *RichEditor) splitBlock() {
	bi := e.caret.Path[0]
	text := e.blockText(bi)
	off := clamp(e.caret.Offset, 0, len(text))
	before, after := text[:off], text[off:]
	e.setBlockText(bi, before)
	newBlock := doc.Block{Type: doc.BlockParagraph, Inlines: []doc.Inline{{Text: after}}}
	e.doc.Blocks = insertBlock(e.doc.Blocks, bi+1, newBlock)
	e.caret = doc.Position{Path: []int{bi + 1}, Inline: 0, Offset: 0}
	e.preferredX = -1
}

// backspace deletes one rune before the caret, or merges with the previous
// block if at offset 0.
func (e *RichEditor) backspace() {
	bi := e.caret.Path[0]
	text := e.blockText(bi)
	if e.caret.Offset > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:e.caret.Offset])
		_ = r
		e.setBlockText(bi, text[:e.caret.Offset-size]+text[e.caret.Offset:])
		e.caret.Offset -= size
		e.preferredX = -1
		return
	}
	// At start of block: merge with previous if any.
	if bi == 0 {
		return
	}
	prevText := e.blockText(bi - 1)
	mergedOffset := len(prevText)
	e.setBlockText(bi-1, prevText+text)
	e.doc.Blocks = removeBlock(e.doc.Blocks, bi)
	e.caret = doc.Position{Path: []int{bi - 1}, Inline: 0, Offset: mergedOffset}
	e.preferredX = -1
}

// deleteForward removes one rune at the caret, or merges with the next block
// if at end. Used for Delete key.
func (e *RichEditor) deleteForward() {
	bi := e.caret.Path[0]
	text := e.blockText(bi)
	if e.caret.Offset < len(text) {
		_, size := utf8.DecodeRuneInString(text[e.caret.Offset:])
		e.setBlockText(bi, text[:e.caret.Offset]+text[e.caret.Offset+size:])
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
