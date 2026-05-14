package editor

import (
	"testing"
	"unicode/utf8"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

func docInsert(b doc.Block, off int, s string, marks doc.Mark) doc.Block {
	return doc.InsertText(b, off, s, marks)
}

func testPos(block, offset int) doc.Position {
	return doc.Position{Path: []int{block}, Inline: 0, Offset: offset}
}

func utf8EncodeRune(buf []byte, r rune) int { return utf8.EncodeRune(buf, r) }

// Tests exercise the pure-data mutation path (commands.go) without touching
// the renderer, so they don't need a Fyne app context.

func newTestEditor(initial string) *RichEditor {
	d := doc.ParseMarkdown(initial)
	e := &RichEditor{doc: d, preferredX: -1}
	e.setCaret(d.Start())
	return e
}

func pos(block, offset int) doc.Position {
	return doc.Position{Path: []int{block}, Inline: 0, Offset: offset}
}

func TestInsertRune(t *testing.T) {
	e := newTestEditor("hello\n")
	e.setCaret(pos(0, 5))
	e.insertRune('!')
	if e.blockText(0) != "hello!" {
		t.Errorf("got %q", e.blockText(0))
	}
	if e.sel.Head.Offset != 6 {
		t.Errorf("caret at %d, want 6", e.sel.Head.Offset)
	}
}

func TestBackspaceMergesBlocks(t *testing.T) {
	e := newTestEditor("first\n\nsecond\n")
	e.setCaret(pos(1, 0))
	e.backspace()
	if len(e.doc.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(e.doc.Blocks))
	}
	if e.blockText(0) != "firstsecond" {
		t.Errorf("got %q", e.blockText(0))
	}
	if e.sel.Head.Path[0] != 0 || e.sel.Head.Offset != 5 {
		t.Errorf("caret at block %d off %d", e.sel.Head.Path[0], e.sel.Head.Offset)
	}
}

func TestSplitBlock(t *testing.T) {
	e := newTestEditor("hello world\n")
	e.setCaret(pos(0, 6))
	e.splitBlock()
	if len(e.doc.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(e.doc.Blocks))
	}
	if e.blockText(0) != "hello " || e.blockText(1) != "world" {
		t.Errorf("blocks = %q / %q", e.blockText(0), e.blockText(1))
	}
}

func TestDeleteForwardMergesBlocks(t *testing.T) {
	e := newTestEditor("first\n\nsecond\n")
	e.setCaret(pos(0, 5))
	e.deleteForward()
	if len(e.doc.Blocks) != 1 || e.blockText(0) != "firstsecond" {
		t.Errorf("blocks=%d text=%q", len(e.doc.Blocks), e.blockText(0))
	}
}

func TestTypeThenSaveRoundTrip(t *testing.T) {
	e := newTestEditor("")
	for _, r := range "hello world" {
		e.insertRune(r)
	}
	e.splitBlock()
	for _, r := range "second para" {
		e.insertRune(r)
	}
	got := doc.WriteMarkdown(e.doc)
	want := "hello world\n\nsecond para\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestSelectionRangeNormalization(t *testing.T) {
	e := newTestEditor("hello world\n")
	// Anchor after head.
	e.sel = doc.Selection{Anchor: pos(0, 11), Head: pos(0, 0)}
	lo, hi := e.selRange()
	if lo.Offset != 0 || hi.Offset != 11 {
		t.Errorf("normalize: lo=%d hi=%d", lo.Offset, hi.Offset)
	}
}

func TestDeleteSelectionSameBlock(t *testing.T) {
	e := newTestEditor("hello world\n")
	e.sel = doc.Selection{Anchor: pos(0, 5), Head: pos(0, 11)} // " world"
	e.deleteSelection()
	if e.blockText(0) != "hello" {
		t.Errorf("got %q", e.blockText(0))
	}
	if !e.sel.IsCollapsed() || e.sel.Head.Offset != 5 {
		t.Errorf("selection should collapse at 5, got %+v", e.sel)
	}
}

func TestDeleteSelectionAcrossBlocks(t *testing.T) {
	e := newTestEditor("alpha\n\nbeta\n\ngamma\n")
	// From middle of "alpha" to middle of "gamma" — should collapse blocks.
	e.sel = doc.Selection{Anchor: pos(0, 2), Head: pos(2, 3)}
	e.deleteSelection()
	if len(e.doc.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(e.doc.Blocks))
	}
	// "alpha"[:2] + "gamma"[3:] = "al" + "ma" = "alma"
	if e.blockText(0) != "alma" {
		t.Errorf("got %q want %q", e.blockText(0), "alma")
	}
}

func TestSelectionTextSameBlock(t *testing.T) {
	e := newTestEditor("hello world\n")
	e.sel = doc.Selection{Anchor: pos(0, 6), Head: pos(0, 11)}
	if got := e.selectionText(); got != "world" {
		t.Errorf("got %q", got)
	}
}

func TestSelectionTextAcrossBlocks(t *testing.T) {
	e := newTestEditor("foo\n\nbar\n\nbaz\n")
	e.sel = doc.Selection{Anchor: pos(0, 1), Head: pos(2, 2)}
	want := "oo\n\nbar\n\nba"
	if got := e.selectionText(); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestUndoTypingRun(t *testing.T) {
	e := newTestEditor("")
	// Simulate a run of TypedRune. The same-kind coalescing collapses these
	// into one undo entry.
	for _, r := range "hello" {
		e.commitUndo(undoKindTyping)
		e.deleteSelection()
		c := e.sel.Head
		bi := c.Path[0]
		marks := e.activeMarks()
		buf := make([]byte, 4)
		n := encodeRune(buf, r)
		e.doc.Blocks[bi] = docInsert(e.doc.Blocks[bi], c.Offset, string(buf[:n]), marks)
		e.setCaret(testPos(bi, c.Offset+n))
	}
	if e.blockText(0) != "hello" {
		t.Fatalf("setup wrong: %q", e.blockText(0))
	}
	e.Undo()
	if e.blockText(0) != "" {
		t.Errorf("expected single undo to clear the typed run, got %q", e.blockText(0))
	}
	e.Redo()
	if e.blockText(0) != "hello" {
		t.Errorf("expected redo to restore typed run, got %q", e.blockText(0))
	}
}

func TestUndoBreaksOnCaretMove(t *testing.T) {
	e := newTestEditor("abc")
	// First typing run
	e.commitUndo(undoKindTyping)
	e.insertRune('X')
	// Break run, then second typing run
	e.breakUndoRun()
	e.insertRune('Y')
	if e.blockText(0) != "abcXY" && e.blockText(0) != "XYabc" && !contains(e.blockText(0), "X") {
		t.Logf("text after inserts: %q", e.blockText(0))
	}
	e.Undo()
	first := e.blockText(0)
	e.Undo()
	second := e.blockText(0)
	if first == second {
		t.Errorf("two distinct undo steps expected; got identical states %q / %q", first, second)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || (len(haystack) > 0 && (indexOf(haystack, needle) >= 0)))
}

func indexOf(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Test helpers to avoid pulling unicode/utf8 into the test file.
func encodeRune(buf []byte, r rune) int {
	if r < 0x80 {
		buf[0] = byte(r)
		return 1
	}
	// Multi-byte fall back to the std encoder.
	return utf8EncodeRune(buf, r)
}

func TestInsertReplacesSelection(t *testing.T) {
	e := newTestEditor("hello world\n")
	e.sel = doc.Selection{Anchor: pos(0, 6), Head: pos(0, 11)} // "world"
	e.insertRune('X')
	if e.blockText(0) != "hello X" {
		t.Errorf("got %q", e.blockText(0))
	}
}
