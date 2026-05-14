package editor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// buildContextMenu constructs the right-click menu. Items operate on the
// current selection (Cut/Copy require non-empty; Paste requires clipboard
// content; Format submenu toggles marks). Future M3 work adds AI actions.
func (e *RichEditor) buildContextMenu() *fyne.Menu {
	canvas := fyne.CurrentApp().Driver().CanvasForObject(e)
	var clipboard fyne.Clipboard
	if canvas != nil {
		clipboard = fyne.CurrentApp().Clipboard()
	}

	e.mu.Lock()
	hasSelection := !e.sel.IsCollapsed()
	e.mu.Unlock()

	cut := fyne.NewMenuItem("Cut", func() {
		if clipboard != nil {
			e.doCut(clipboard)
		}
	})
	cut.Disabled = !hasSelection

	copy := fyne.NewMenuItem("Copy", func() {
		if clipboard != nil {
			e.doCopy(clipboard)
		}
	})
	copy.Disabled = !hasSelection

	paste := fyne.NewMenuItem("Paste", func() {
		if clipboard != nil {
			e.doPaste(clipboard)
		}
	})

	selectAllItem := fyne.NewMenuItem("Select All", func() { e.selectAll() })

	format := fyne.NewMenuItem("Format", nil)
	format.ChildMenu = fyne.NewMenu("",
		markItem("Bold", doc.MarkBold, e),
		markItem("Italic", doc.MarkItalic, e),
		markItem("Underline", doc.MarkUnderline, e),
		markItem("Code", doc.MarkCode, e),
		markItem("Strikethrough", doc.MarkStrike, e),
	)
	format.Disabled = !hasSelection

	return fyne.NewMenu("",
		cut, copy, paste,
		fyne.NewMenuItemSeparator(),
		selectAllItem,
		fyne.NewMenuItemSeparator(),
		format,
	)
}

func markItem(label string, m doc.Mark, e *RichEditor) *fyne.MenuItem {
	return fyne.NewMenuItem(label, func() { e.ToggleMark(m) })
}

// showContextMenuAt pops up the right-click menu at the given widget-local
// position. We translate to canvas coordinates via the driver.
func (e *RichEditor) showContextMenuAt(localPos fyne.Position) {
	c := fyne.CurrentApp().Driver().CanvasForObject(e)
	if c == nil {
		return
	}
	menu := e.buildContextMenu()
	widget.ShowPopUpMenuAtPosition(menu, c, absolutePosition(e, localPos))
}

// absolutePosition converts a widget-local position to a canvas-absolute
// position.
func absolutePosition(obj fyne.CanvasObject, local fyne.Position) fyne.Position {
	abs := fyne.CurrentApp().Driver().AbsolutePositionForObject(obj)
	return fyne.NewPos(abs.X+local.X, abs.Y+local.Y)
}
