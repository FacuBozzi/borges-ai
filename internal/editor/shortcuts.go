package editor

import (
	"fyne.io/fyne/v2"
)

// TypedShortcut handles registered shortcuts (cmd+C/X/V/A, etc.). Implements
// fyne.Shortcutable.
func (e *RichEditor) TypedShortcut(s fyne.Shortcut) {
	switch sc := s.(type) {
	case *fyne.ShortcutCopy:
		e.doCopy(sc.Clipboard)
	case *fyne.ShortcutCut:
		e.doCut(sc.Clipboard)
	case *fyne.ShortcutPaste:
		e.doPaste(sc.Clipboard)
	case *fyne.ShortcutSelectAll:
		e.selectAll()
	}
}

func (e *RichEditor) doCopy(cb fyne.Clipboard) {
	e.mu.Lock()
	text := e.selectionText()
	e.mu.Unlock()
	if text == "" || cb == nil {
		return
	}
	cb.SetContent(text)
}

func (e *RichEditor) doCut(cb fyne.Clipboard) {
	e.mu.Lock()
	text := e.selectionText()
	if text != "" && cb != nil {
		cb.SetContent(text)
	}
	if !e.sel.IsCollapsed() {
		e.deleteSelection()
	}
	e.mu.Unlock()
	if text != "" {
		e.invalidate()
	}
}

func (e *RichEditor) doPaste(cb fyne.Clipboard) {
	if cb == nil {
		return
	}
	text := cb.Content()
	if text == "" {
		return
	}
	e.mu.Lock()
	e.insertString(text)
	e.mu.Unlock()
	e.invalidate()
}
