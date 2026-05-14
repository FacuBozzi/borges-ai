package editor

import (
	"testing"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

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

func TestInsertReplacesSelection(t *testing.T) {
	e := newTestEditor("hello world\n")
	e.sel = doc.Selection{Anchor: pos(0, 6), Head: pos(0, 11)} // "world"
	e.insertRune('X')
	if e.blockText(0) != "hello X" {
		t.Errorf("got %q", e.blockText(0))
	}
}
