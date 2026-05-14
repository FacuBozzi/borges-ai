package editor

import (
	"fyne.io/fyne/v2"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// Tapped implements fyne.Tappable. A click moves the caret to the clicked
// position and focuses the editor so subsequent key input is captured.
func (e *RichEditor) Tapped(ev *fyne.PointEvent) {
	e.positionCaretAt(ev.Position)
	if c := fyne.CurrentApp().Driver().CanvasForObject(e); c != nil {
		c.Focus(e)
	}
}

// TappedSecondary will host the right-click context menu in M2. For now it's
// a no-op so we satisfy the interface and prevent the default paste-only
// menu from interfering with our future implementation.
func (e *RichEditor) TappedSecondary(_ *fyne.PointEvent) {}

func (e *RichEditor) positionCaretAt(p fyne.Position) {
	e.mu.Lock()
	lines := e.lines
	if len(lines) == 0 {
		e.mu.Unlock()
		return
	}
	li := lineAtY(lines, p.Y)
	if li < 0 {
		e.mu.Unlock()
		return
	}
	ln := lines[li]
	local := offsetAtX(ln, p.X)
	e.caret = doc.Position{
		Path:   []int{ln.blockIdx},
		Inline: 0,
		Offset: ln.startByte + local,
	}
	e.preferredX = -1
	e.mu.Unlock()
	e.Refresh()
}
