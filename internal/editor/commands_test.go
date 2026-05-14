package editor

import (
	"testing"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// These tests exercise the pure-data mutation path (commands.go) without
// touching the renderer, so they don't need a Fyne app context.

func newTestEditor(initial string) *RichEditor {
	d := doc.ParseMarkdown(initial)
	return &RichEditor{doc: d, caret: d.Start(), preferredX: -1}
}

func TestInsertRune(t *testing.T) {
	e := newTestEditor("hello\n")
	e.caret.Offset = 5 // end of "hello"
	e.insertRune('!')
	got := e.blockText(0)
	if got != "hello!" {
		t.Errorf("got %q want %q", got, "hello!")
	}
	if e.caret.Offset != 6 {
		t.Errorf("caret at %d, want 6", e.caret.Offset)
	}
}

func TestBackspaceMergesBlocks(t *testing.T) {
	e := newTestEditor("first\n\nsecond\n")
	e.caret = doc.Position{Path: []int{1}, Inline: 0, Offset: 0}
	e.backspace()
	if len(e.doc.Blocks) != 1 {
		t.Fatalf("expected 1 block after merge, got %d", len(e.doc.Blocks))
	}
	if e.blockText(0) != "firstsecond" {
		t.Errorf("merged text = %q, want %q", e.blockText(0), "firstsecond")
	}
	if e.caret.Path[0] != 0 || e.caret.Offset != 5 {
		t.Errorf("caret should land between merged halves, got block %d off %d", e.caret.Path[0], e.caret.Offset)
	}
}

func TestSplitBlock(t *testing.T) {
	e := newTestEditor("hello world\n")
	e.caret.Offset = 6 // before "world"
	e.splitBlock()
	if len(e.doc.Blocks) != 2 {
		t.Fatalf("expected 2 blocks after split, got %d", len(e.doc.Blocks))
	}
	if e.blockText(0) != "hello " {
		t.Errorf("block 0 = %q", e.blockText(0))
	}
	if e.blockText(1) != "world" {
		t.Errorf("block 1 = %q", e.blockText(1))
	}
	if e.caret.Path[0] != 1 || e.caret.Offset != 0 {
		t.Errorf("caret should be at start of new block, got block %d off %d", e.caret.Path[0], e.caret.Offset)
	}
}

func TestDeleteForwardMergesBlocks(t *testing.T) {
	e := newTestEditor("first\n\nsecond\n")
	e.caret.Offset = 5 // end of "first"
	e.deleteForward()
	if len(e.doc.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(e.doc.Blocks))
	}
	if e.blockText(0) != "firstsecond" {
		t.Errorf("got %q", e.blockText(0))
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
		t.Errorf("round-trip:\ngot:  %q\nwant: %q", got, want)
	}
}
