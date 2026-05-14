package editor

import (
	"fyne.io/fyne/v2"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// Compile-time assertion that we satisfy fyne.Draggable for drag-select.
var _ fyne.Draggable = (*RichEditor)(nil)

// Tapped implements fyne.Tappable. A click collapses the selection at the
// clicked position and focuses the editor.
func (e *RichEditor) Tapped(ev *fyne.PointEvent) {
	pos, ok := e.hitTest(ev.Position)
	if !ok {
		return
	}
	e.mu.Lock()
	e.setCaret(pos)
	e.preferredX = -1
	e.mu.Unlock()
	if c := fyne.CurrentApp().Driver().CanvasForObject(e); c != nil {
		c.Focus(e)
	}
	e.Refresh()
}

// TappedSecondary shows the right-click context menu. If the click landed
// outside the current selection we first reposition the caret so the menu's
// commands operate on a sensible target.
func (e *RichEditor) TappedSecondary(ev *fyne.PointEvent) {
	if pos, ok := e.hitTest(ev.Position); ok {
		e.mu.Lock()
		// Only move the caret if the click is OUTSIDE the current selection —
		// otherwise a right-click inside a selection should preserve it.
		inSelection := !e.sel.IsCollapsed() && !positionLess(pos, lower(e.sel)) && positionLess(pos, upper(e.sel))
		if !inSelection {
			e.setCaret(pos)
			e.preferredX = -1
		}
		e.mu.Unlock()
		if c := fyne.CurrentApp().Driver().CanvasForObject(e); c != nil {
			c.Focus(e)
		}
		e.Refresh()
	}
	e.showContextMenuAt(ev.Position)
}

func lower(s doc.Selection) doc.Position {
	if positionLess(s.Head, s.Anchor) {
		return s.Head
	}
	return s.Anchor
}

func upper(s doc.Selection) doc.Position {
	if positionLess(s.Head, s.Anchor) {
		return s.Anchor
	}
	return s.Head
}

// Dragged implements fyne.Draggable. The first event of a drag captures the
// anchor at the press location; subsequent events move the head.
func (e *RichEditor) Dragged(ev *fyne.DragEvent) {
	pos, ok := e.hitTest(ev.Position)
	if !ok {
		return
	}
	e.mu.Lock()
	if !e.dragging {
		// Compute the press point by subtracting the accumulated drag delta.
		anchorPos := fyne.NewPos(ev.Position.X-ev.Dragged.DX, ev.Position.Y-ev.Dragged.DY)
		if anchor, ok := e.hitTestLocked(anchorPos); ok {
			e.dragAnchor = anchor
		} else {
			e.dragAnchor = pos
		}
		e.dragging = true
	}
	e.sel = doc.Selection{Anchor: e.dragAnchor, Head: pos}
	e.preferredX = -1
	e.mu.Unlock()
	if c := fyne.CurrentApp().Driver().CanvasForObject(e); c != nil {
		c.Focus(e)
	}
	e.Refresh()
}

// DragEnd is fired when the mouse button is released after a drag.
func (e *RichEditor) DragEnd() {
	e.mu.Lock()
	e.dragging = false
	e.mu.Unlock()
}

// hitTest maps a widget-relative point to a document position. Returns
// false when there is no line table yet (very early in widget life).
func (e *RichEditor) hitTest(p fyne.Position) (doc.Position, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.hitTestLocked(p)
}

func (e *RichEditor) hitTestLocked(p fyne.Position) (doc.Position, bool) {
	if len(e.lines) == 0 {
		return doc.Position{}, false
	}
	li := lineAtY(e.lines, p.Y)
	if li < 0 {
		return doc.Position{}, false
	}
	ln := e.lines[li]
	local := offsetAtX(ln, p.X)
	return doc.Position{
		Path:   []int{ln.blockIdx},
		Inline: 0,
		Offset: ln.startByte + local,
	}, true
}
