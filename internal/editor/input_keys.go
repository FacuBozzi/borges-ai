package editor

import (
	"unicode/utf8"

	"fyne.io/fyne/v2"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// TypedRune handles single printable runes entered by the user. Implements
// fyne.Focusable.
func (e *RichEditor) TypedRune(r rune) {
	if r == 0 {
		return
	}
	e.mu.Lock()
	e.insertRune(r)
	e.mu.Unlock()
	e.invalidate()
}

// TypedKey handles non-printable keys: navigation, deletion, return.
// Implements fyne.Focusable.
func (e *RichEditor) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeyBackspace:
		e.mu.Lock()
		e.backspace()
		e.mu.Unlock()
		e.invalidate()
	case fyne.KeyDelete:
		e.mu.Lock()
		e.deleteForward()
		e.mu.Unlock()
		e.invalidate()
	case fyne.KeyReturn, fyne.KeyEnter:
		e.mu.Lock()
		e.splitBlock()
		e.mu.Unlock()
		e.invalidate()
	case fyne.KeyLeft:
		e.moveCaret(caretLeft)
	case fyne.KeyRight:
		e.moveCaret(caretRight)
	case fyne.KeyUp:
		e.moveCaret(caretUp)
	case fyne.KeyDown:
		e.moveCaret(caretDown)
	case fyne.KeyHome:
		e.moveCaret(caretLineStart)
	case fyne.KeyEnd:
		e.moveCaret(caretLineEnd)
	}
}

// invalidate is called after every mutation to refresh the renderer and
// notify the app of changes.
func (e *RichEditor) invalidate() {
	e.Refresh()
	e.fireChanged()
}

// caretMove is the kind of caret motion the user requested. Implemented as
// an enum so the switch in moveCaret stays compact.
type caretMove int

const (
	caretLeft caretMove = iota
	caretRight
	caretUp
	caretDown
	caretLineStart
	caretLineEnd
)

func (e *RichEditor) moveCaret(m caretMove) {
	e.mu.Lock()
	switch m {
	case caretLeft:
		e.caretMoveLeft()
		e.preferredX = -1
	case caretRight:
		e.caretMoveRight()
		e.preferredX = -1
	case caretUp:
		e.caretMoveVertical(-1)
	case caretDown:
		e.caretMoveVertical(+1)
	case caretLineStart:
		e.caretMoveLineEdge(-1)
		e.preferredX = -1
	case caretLineEnd:
		e.caretMoveLineEdge(+1)
		e.preferredX = -1
	}
	e.mu.Unlock()
	e.Refresh()
}

func (e *RichEditor) caretMoveLeft() {
	bi := e.caret.Path[0]
	if e.caret.Offset > 0 {
		text := e.blockText(bi)
		_, size := utf8.DecodeLastRuneInString(text[:e.caret.Offset])
		e.caret.Offset -= size
		return
	}
	if bi > 0 {
		prev := bi - 1
		e.caret = doc.Position{Path: []int{prev}, Inline: 0, Offset: len(e.blockText(prev))}
	}
}

func (e *RichEditor) caretMoveRight() {
	bi := e.caret.Path[0]
	text := e.blockText(bi)
	if e.caret.Offset < len(text) {
		_, size := utf8.DecodeRuneInString(text[e.caret.Offset:])
		e.caret.Offset += size
		return
	}
	if bi+1 < len(e.doc.Blocks) {
		e.caret = doc.Position{Path: []int{bi + 1}, Inline: 0, Offset: 0}
	}
}

func (e *RichEditor) caretMoveLineEdge(dir int) {
	// Find current line within the cached line table.
	if len(e.lines) == 0 || len(e.caret.Path) == 0 {
		return
	}
	li := lineForPosition(e.lines, e.caret.Path[0], e.caret.Offset)
	if li < 0 {
		return
	}
	ln := e.lines[li]
	if dir < 0 {
		e.caret.Offset = ln.startByte
	} else {
		e.caret.Offset = ln.endByte
	}
}

// caretMoveVertical moves the caret up (dir=-1) or down (dir=+1) by one
// visual line, preserving the user's preferred X column across runs of up/
// down moves through ragged paragraphs.
func (e *RichEditor) caretMoveVertical(dir int) {
	if len(e.lines) == 0 || len(e.caret.Path) == 0 {
		return
	}
	li := lineForPosition(e.lines, e.caret.Path[0], e.caret.Offset)
	if li < 0 {
		return
	}
	want := e.preferredX
	if want < 0 {
		want = xForOffset(e.lines[li], e.caret.Offset)
		e.preferredX = want
	}
	target := li + dir
	if target < 0 || target >= len(e.lines) {
		return
	}
	dest := e.lines[target]
	e.caret = doc.Position{
		Path:   []int{dest.blockIdx},
		Inline: 0,
		Offset: dest.startByte + offsetAtX(dest, want),
	}
}
