package editor

import (
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// shiftHeld is set by KeyDown/KeyUp so we know whether arrow keys should
// extend the selection. Fyne fires TypedKey without modifier metadata, so we
// track them via desktop.Keyable.
var _ desktop.Keyable = (*RichEditor)(nil)

func (e *RichEditor) KeyDown(ev *fyne.KeyEvent) {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		e.shiftHeld = true
	}
}

func (e *RichEditor) KeyUp(ev *fyne.KeyEvent) {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		e.shiftHeld = false
	}
}

// TypedRune handles single printable runes. fyne.Focusable.
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
// fyne.Focusable.
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
		e.moveCaret(caretLeft, e.shiftHeld)
	case fyne.KeyRight:
		e.moveCaret(caretRight, e.shiftHeld)
	case fyne.KeyUp:
		e.moveCaret(caretUp, e.shiftHeld)
	case fyne.KeyDown:
		e.moveCaret(caretDown, e.shiftHeld)
	case fyne.KeyHome:
		e.moveCaret(caretLineStart, e.shiftHeld)
	case fyne.KeyEnd:
		e.moveCaret(caretLineEnd, e.shiftHeld)
	}
}

// invalidate refreshes the renderer and notifies listeners.
func (e *RichEditor) invalidate() {
	e.Refresh()
	e.fireChanged()
}

type caretMove int

const (
	caretLeft caretMove = iota
	caretRight
	caretUp
	caretDown
	caretLineStart
	caretLineEnd
)

// moveCaret moves the selection head. If extend is true the anchor stays;
// otherwise the anchor collapses to the new head. preferredX is preserved
// only across consecutive caretUp/caretDown moves.
func (e *RichEditor) moveCaret(m caretMove, extend bool) {
	e.mu.Lock()
	prev := e.sel.Head
	switch m {
	case caretLeft:
		newPos := e.posLeft(prev)
		e.sel.Head = newPos
		e.preferredX = -1
	case caretRight:
		newPos := e.posRight(prev)
		e.sel.Head = newPos
		e.preferredX = -1
	case caretUp:
		e.sel.Head = e.posVertical(prev, -1)
	case caretDown:
		e.sel.Head = e.posVertical(prev, +1)
	case caretLineStart:
		e.sel.Head = e.posLineEdge(prev, -1)
		e.preferredX = -1
	case caretLineEnd:
		e.sel.Head = e.posLineEdge(prev, +1)
		e.preferredX = -1
	}
	if !extend {
		e.sel.Anchor = e.sel.Head
	}
	e.mu.Unlock()
	e.Refresh()
}

// selectAll sets the selection to span the entire document.
func (e *RichEditor) selectAll() {
	e.mu.Lock()
	if len(e.doc.Blocks) == 0 {
		e.mu.Unlock()
		return
	}
	last := len(e.doc.Blocks) - 1
	e.sel = doc.Selection{
		Anchor: doc.Position{Path: []int{0}, Inline: 0, Offset: 0},
		Head:   doc.Position{Path: []int{last}, Inline: 0, Offset: len(e.blockText(last))},
	}
	e.preferredX = -1
	e.mu.Unlock()
	e.Refresh()
}

func (e *RichEditor) posLeft(p doc.Position) doc.Position {
	bi := p.Path[0]
	if p.Offset > 0 {
		text := e.blockText(bi)
		_, size := utf8.DecodeLastRuneInString(text[:p.Offset])
		return doc.Position{Path: []int{bi}, Inline: 0, Offset: p.Offset - size}
	}
	if bi > 0 {
		return doc.Position{Path: []int{bi - 1}, Inline: 0, Offset: len(e.blockText(bi - 1))}
	}
	return p
}

func (e *RichEditor) posRight(p doc.Position) doc.Position {
	bi := p.Path[0]
	text := e.blockText(bi)
	if p.Offset < len(text) {
		_, size := utf8.DecodeRuneInString(text[p.Offset:])
		return doc.Position{Path: []int{bi}, Inline: 0, Offset: p.Offset + size}
	}
	if bi+1 < len(e.doc.Blocks) {
		return doc.Position{Path: []int{bi + 1}, Inline: 0, Offset: 0}
	}
	return p
}

func (e *RichEditor) posLineEdge(p doc.Position, dir int) doc.Position {
	if len(e.lines) == 0 {
		return p
	}
	li := lineForPosition(e.lines, p.Path[0], p.Offset)
	if li < 0 {
		return p
	}
	ln := e.lines[li]
	if dir < 0 {
		return doc.Position{Path: []int{ln.blockIdx}, Inline: 0, Offset: ln.startByte}
	}
	return doc.Position{Path: []int{ln.blockIdx}, Inline: 0, Offset: ln.endByte}
}

func (e *RichEditor) posVertical(p doc.Position, dir int) doc.Position {
	if len(e.lines) == 0 {
		return p
	}
	li := lineForPosition(e.lines, p.Path[0], p.Offset)
	if li < 0 {
		return p
	}
	want := e.preferredX
	if want < 0 {
		want = xForOffset(e.lines[li], p.Offset)
		e.preferredX = want
	}
	target := li + dir
	if target < 0 || target >= len(e.lines) {
		return p
	}
	dest := e.lines[target]
	return doc.Position{Path: []int{dest.blockIdx}, Inline: 0, Offset: dest.startByte + offsetAtX(dest, want)}
}
