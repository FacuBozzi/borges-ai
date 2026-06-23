package app

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// newMultilineEntry returns a multi-line entry pre-filled with current text
// and a placeholder. Used by dialogs that take a chunk of free text.
func newMultilineEntry(current, placeholder string) *widget.Entry {
	e := widget.NewMultiLineEntry()
	e.SetPlaceHolder(placeholder)
	e.SetText(current)
	e.Wrapping = fyne.TextWrapWord
	return e
}

// submitEntry is a multi-line entry where Enter submits (fires onSubmit) and
// Shift+Enter inserts a newline. Used by the Ask AI dialog so the keyboard,
// not the mouse, drives submission. We track Shift ourselves because
// fyne.KeyEvent carries no modifier state in TypedKey.
type submitEntry struct {
	widget.Entry
	shiftHeld bool
	onSubmit  func()
}

func newSubmitEntry(placeholder string, onSubmit func()) *submitEntry {
	e := &submitEntry{onSubmit: onSubmit}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.SetPlaceHolder(placeholder)
	e.ExtendBaseWidget(e)
	return e
}

func (e *submitEntry) KeyDown(key *fyne.KeyEvent) {
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.shiftHeld = true
	}
	e.Entry.KeyDown(key)
}

func (e *submitEntry) KeyUp(key *fyne.KeyEvent) {
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.shiftHeld = false
	}
	e.Entry.KeyUp(key)
}

func (e *submitEntry) TypedKey(key *fyne.KeyEvent) {
	if (key.Name == fyne.KeyReturn || key.Name == fyne.KeyEnter) && !e.shiftHeld {
		if e.onSubmit != nil {
			e.onSubmit()
		}
		return
	}
	e.Entry.TypedKey(key)
}
