package app

import (
	"fyne.io/fyne/v2"
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
