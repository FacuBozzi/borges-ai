package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// Editor mutators. Callers must hold e.mu, and should call e.invalidate()
// afterwards.
//
// Position semantics in this package: Offset is the byte offset within the
// block's concatenated plain text. Inline is always 0; the inline structure
// inside the block is bookkeeping for marks/styles and is handled by the
// doc.InsertText / doc.DeleteRange helpers.

// blockText returns the concatenated plain text of all inlines in the block.
func (e *RichEditor) blockText(blockIdx int) string {
	if blockIdx < 0 || blockIdx >= len(e.doc.Blocks) {
		return ""
	}
	return e.doc.Blocks[blockIdx].PlainText()
}

// blockLen returns the byte length of the block's plain text.
func (e *RichEditor) blockLen(blockIdx int) int {
	return len(e.blockText(blockIdx))
}

// selRange returns the normalized (lower, upper) endpoints of the selection.
func (e *RichEditor) selRange() (doc.Position, doc.Position) {
	if positionLess(e.sel.Head, e.sel.Anchor) {
		return e.sel.Head, e.sel.Anchor
	}
	return e.sel.Anchor, e.sel.Head
}

// selectionText returns the plain text contents of the current selection.
// Cross-block ranges separate blocks with "\n\n".
func (e *RichEditor) selectionText() string {
	if e.sel.IsCollapsed() {
		return ""
	}
	lo, hi := e.selRange()
	if lo.Path[0] == hi.Path[0] {
		t := e.blockText(lo.Path[0])
		return t[lo.Offset:hi.Offset]
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
// start. No-op when collapsed.
func (e *RichEditor) deleteSelection() {
	if e.sel.IsCollapsed() {
		return
	}
	lo, hi := e.selRange()
	if lo.Path[0] == hi.Path[0] {
		bi := lo.Path[0]
		e.doc.Blocks[bi] = doc.DeleteRange(e.doc.Blocks[bi], lo.Offset, hi.Offset)
		e.setCaret(lo)
		e.preferredX = -1
		return
	}
	// Strip the trailing portion of the start block and the leading portion
	// of the end block, then merge the two and drop intervening blocks.
	startBlk := e.doc.Blocks[lo.Path[0]]
	endBlk := e.doc.Blocks[hi.Path[0]]
	startBlk = doc.DeleteRange(startBlk, lo.Offset, len(startBlk.PlainText()))
	endBlk = doc.DeleteRange(endBlk, 0, hi.Offset)
	// Merge: append endBlk's inlines to startBlk.
	merged := startBlk
	mergeOffset := len(merged.PlainText())
	for _, in := range endBlk.Inlines {
		merged.Inlines = append(merged.Inlines, in)
	}
	merged = normalizeBlock(merged)
	e.doc.Blocks[lo.Path[0]] = merged
	e.doc.Blocks = append(e.doc.Blocks[:lo.Path[0]+1], e.doc.Blocks[hi.Path[0]+1:]...)
	e.setCaret(doc.Position{Path: []int{lo.Path[0]}, Inline: 0, Offset: mergeOffset})
	e.preferredX = -1
	_ = mergeOffset
}

// insertRune inserts r, replacing any current selection. r inherits the
// marks active at the caret (typing inside a bold span stays bold).
func (e *RichEditor) insertRune(r rune) {
	e.commitUndo(undoKindTyping)
	e.deleteSelection()
	c := e.sel.Head
	bi := c.Path[0]
	marks := e.activeMarks()
	buf := make([]byte, utf8.UTFMax)
	n := utf8.EncodeRune(buf, r)
	e.doc.Blocks[bi] = doc.InsertText(e.doc.Blocks[bi], c.Offset, string(buf[:n]), marks)
	e.setCaret(doc.Position{Path: []int{bi}, Inline: 0, Offset: c.Offset + n})
	e.preferredX = -1
}

// insertString inserts a literal string, splitting on "\n\n" into blocks and
// inserting "\n" as soft breaks inside paragraphs.
func (e *RichEditor) insertString(s string) {
	e.commitUndo(undoKindOther)
	e.deleteSelection()
	if s == "" {
		return
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.Split(s, "\n\n")
	for i, part := range parts {
		if i > 0 {
			e.splitBlock()
		}
		e.insertStringNoBreak(part)
	}
}

// insertStringNoBreak inserts s at the caret with the active marks. No
// block-splitting; embedded \n becomes literal text.
func (e *RichEditor) insertStringNoBreak(s string) {
	if s == "" {
		return
	}
	c := e.sel.Head
	bi := c.Path[0]
	marks := e.activeMarks()
	e.doc.Blocks[bi] = doc.InsertText(e.doc.Blocks[bi], c.Offset, s, marks)
	e.setCaret(doc.Position{Path: []int{bi}, Inline: 0, Offset: c.Offset + len(s)})
	e.preferredX = -1
}

// splitBlock breaks the current block at the caret. Used for Enter.
func (e *RichEditor) splitBlock() {
	e.commitUndo(undoKindOther)
	e.deleteSelection()
	c := e.sel.Head
	bi := c.Path[0]
	left, right := doc.SplitBlock(e.doc.Blocks[bi], c.Offset)
	e.doc.Blocks[bi] = left
	e.doc.Blocks = insertBlock(e.doc.Blocks, bi+1, right)
	e.setCaret(doc.Position{Path: []int{bi + 1}, Inline: 0, Offset: 0})
	e.preferredX = -1
}

// backspace deletes the selection, else one rune before the caret, else
// (if at start of a non-paragraph block) reverts the block to a paragraph,
// else merges with the previous block.
func (e *RichEditor) backspace() {
	if !e.sel.IsCollapsed() {
		e.commitUndo(undoKindOther)
		e.deleteSelection()
		return
	}
	e.commitUndo(undoKindDelete)
	c := e.sel.Head
	bi := c.Path[0]
	if c.Offset > 0 {
		text := e.blockText(bi)
		_, size := utf8.DecodeLastRuneInString(text[:c.Offset])
		e.doc.Blocks[bi] = doc.DeleteRange(e.doc.Blocks[bi], c.Offset-size, c.Offset)
		e.setCaret(doc.Position{Path: []int{bi}, Inline: 0, Offset: c.Offset - size})
		e.preferredX = -1
		return
	}
	// At start of block: if it's a non-paragraph (heading, quote, etc.),
	// revert it to a paragraph instead of merging — matches Notion / Bear.
	if e.doc.Blocks[bi].Type != doc.BlockParagraph {
		e.doc.Blocks[bi].Type = doc.BlockParagraph
		e.doc.Blocks[bi].Meta = nil
		e.preferredX = -1
		return
	}
	if bi == 0 {
		return
	}
	prev := e.doc.Blocks[bi-1]
	mergedOffset := len(prev.PlainText())
	prev.Inlines = append(prev.Inlines, e.doc.Blocks[bi].Inlines...)
	e.doc.Blocks[bi-1] = normalizeBlock(prev)
	e.doc.Blocks = removeBlock(e.doc.Blocks, bi)
	e.setCaret(doc.Position{Path: []int{bi - 1}, Inline: 0, Offset: mergedOffset})
	e.preferredX = -1
}

// deleteForward deletes the selection, else one rune at the caret, else
// merges with the next block.
func (e *RichEditor) deleteForward() {
	if !e.sel.IsCollapsed() {
		e.commitUndo(undoKindOther)
		e.deleteSelection()
		return
	}
	e.commitUndo(undoKindDelete)
	c := e.sel.Head
	bi := c.Path[0]
	text := e.blockText(bi)
	if c.Offset < len(text) {
		_, size := utf8.DecodeRuneInString(text[c.Offset:])
		e.doc.Blocks[bi] = doc.DeleteRange(e.doc.Blocks[bi], c.Offset, c.Offset+size)
		e.preferredX = -1
		return
	}
	if bi+1 >= len(e.doc.Blocks) {
		return
	}
	cur := e.doc.Blocks[bi]
	cur.Inlines = append(cur.Inlines, e.doc.Blocks[bi+1].Inlines...)
	e.doc.Blocks[bi] = normalizeBlock(cur)
	e.doc.Blocks = removeBlock(e.doc.Blocks, bi+1)
	e.preferredX = -1
}

// SetBlockType changes the type (and meta) of the block currently containing
// the caret. The block's inline content is preserved.
func (e *RichEditor) SetBlockType(t doc.BlockType, meta map[string]any) {
	e.mu.Lock()
	e.commitUndo(undoKindOther)
	bi := e.sel.Head.Path[0]
	if bi < 0 || bi >= len(e.doc.Blocks) {
		e.mu.Unlock()
		return
	}
	e.doc.Blocks[bi].Type = t
	if meta == nil {
		e.doc.Blocks[bi].Meta = nil
	} else {
		out := make(map[string]any, len(meta))
		for k, v := range meta {
			out[k] = v
		}
		e.doc.Blocks[bi].Meta = out
	}
	e.preferredX = -1
	e.mu.Unlock()
	e.invalidate()
}

// SetHeading turns the current block into a heading at the given level.
// Passing the level the block already has reverts it to a paragraph (toggle
// behavior on cmd+1/2/3).
func (e *RichEditor) SetHeading(level int) {
	e.mu.Lock()
	e.commitUndo(undoKindOther)
	bi := e.sel.Head.Path[0]
	if bi < 0 || bi >= len(e.doc.Blocks) {
		e.mu.Unlock()
		return
	}
	cur := e.doc.Blocks[bi]
	if cur.Type == doc.BlockHeading && doc.HeadingLevel(cur) == level {
		e.doc.Blocks[bi].Type = doc.BlockParagraph
		e.doc.Blocks[bi].Meta = nil
	} else {
		e.doc.Blocks[bi].Type = doc.BlockHeading
		e.doc.Blocks[bi].Meta = map[string]any{"level": level}
	}
	e.preferredX = -1
	e.mu.Unlock()
	e.invalidate()
}

// activeMarks returns the marks new text should inherit at the caret.
// Priority: explicit "pending marks" set by mark-toggle shortcuts when the
// caret is collapsed; otherwise the marks of the surrounding inline.
func (e *RichEditor) activeMarks() doc.Mark {
	if e.pendingMarksSet {
		return e.pendingMarks
	}
	if !e.sel.IsCollapsed() {
		return 0
	}
	bi := e.sel.Head.Path[0]
	if bi < 0 || bi >= len(e.doc.Blocks) {
		return 0
	}
	return doc.MarksAt(e.doc.Blocks[bi], e.sel.Head.Offset)
}

// normalizeBlock collapses adjacent inlines with identical marks and drops
// empty inlines.
func normalizeBlock(b doc.Block) doc.Block {
	// doc.InsertText/DeleteRange already normalize. We re-normalize here
	// after manual inline manipulation (backspace/delete merge paths).
	return normalizeInlinesInBlock(b)
}

func normalizeInlinesInBlock(b doc.Block) doc.Block {
	if len(b.Inlines) == 0 {
		b.Inlines = []doc.Inline{{}}
		return b
	}
	out := b.Inlines[:0]
	for _, in := range b.Inlines {
		if in.Text == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Marks == in.Marks {
			out[n-1].Text += in.Text
			continue
		}
		out = append(out, in)
	}
	if len(out) == 0 {
		out = append(out, doc.Inline{})
	}
	b.Inlines = out
	return b
}

func positionLess(a, b doc.Position) bool {
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

var _ = clamp // keep utility; may be used by future commands
