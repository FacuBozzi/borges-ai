package editor

import (
	"testing"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

func TestShiftPos(t *testing.T) {
	tests := []struct {
		name                         string
		p                            doc.Position
		bi, start, removed, inserted int
		want                         doc.Position
	}{
		{"other block unchanged", pos(1, 5), 0, 2, 0, 4, pos(1, 5)},
		{"before splice unchanged", pos(0, 1), 0, 2, 0, 4, pos(0, 1)},
		{"at splice point unchanged", pos(0, 2), 0, 2, 0, 4, pos(0, 2)},
		{"after pure insert shifts right", pos(0, 5), 0, 2, 0, 4, pos(0, 9)},
		{"after replace shifts by net delta", pos(0, 10), 0, 2, 3, 5, pos(0, 12)},
		{"inside removed region collapses", pos(0, 4), 0, 2, 5, 1, pos(0, 2)},
	}
	for _, tt := range tests {
		got := shiftPos(tt.p, tt.bi, tt.start, tt.removed, tt.inserted)
		if !got.Equal(tt.want) {
			t.Errorf("%s: shiftPos = off %d, want off %d", tt.name, got.Offset, tt.want.Offset)
		}
	}
}

// TestAIReplaceKeepsUserCaret reproduces the reported bug: the user's caret,
// parked after a later word, must stay after that word while AI text streams
// into an earlier selection and changes its length — and after a cancel revert.
func TestAIReplaceKeepsUserCaret(t *testing.T) {
	e := newTestEditor("alpha beta gamma\n")
	e.sel = doc.Selection{Anchor: pos(0, 6), Head: pos(0, 10)} // select "beta"

	h := e.BeginAIReplace()
	if h == nil {
		t.Fatal("BeginAIReplace returned nil")
	}

	// User moves their caret to the end, after "gamma".
	e.setCaret(pos(0, len(e.blockText(0))))

	h.Append("Hello")
	if got := e.blockText(0); got != "alpha Hello gamma" {
		t.Fatalf("after append block = %q", got)
	}
	if want := len(e.blockText(0)); e.sel.Head.Offset != want {
		t.Errorf("after append caret at %d, want %d (still after gamma)", e.sel.Head.Offset, want)
	}

	h.Cancel()
	if got := e.blockText(0); got != "alpha beta gamma" {
		t.Fatalf("after cancel block = %q", got)
	}
	if want := len(e.blockText(0)); e.sel.Head.Offset != want {
		t.Errorf("after cancel caret at %d, want %d (still after gamma)", e.sel.Head.Offset, want)
	}
}
