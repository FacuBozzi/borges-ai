package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// showModal centers body in a fixed-size modal popup over win, shows it, and
// focuses target (may be nil). It returns the popup plus a close function.
//
// Callers that need the close function while building body (e.g. to wire an
// escEntry's onEsc before the popup exists) should declare `var close func()`
// up front, reference it indirectly inside the body, then assign it from this
// call's second return value — the indirection defers the lookup to click time.
func showModal(win fyne.Window, size fyne.Size, body fyne.CanvasObject, target fyne.Focusable) (*widget.PopUp, func()) {
	canvas := win.Canvas()
	wrapper := container.NewGridWrap(size, body)
	pop := widget.NewModalPopUp(wrapper, canvas)
	pop.Show()
	if target != nil {
		canvas.Focus(target)
	}
	return pop, func() { pop.Hide() }
}
