package editor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// TypedShortcut handles registered shortcuts: built-in cmd+C/X/V/A plus our
// custom mark toggles (cmd+B/I/U/E, cmd+shift+X). The Fyne glfw driver
// dispatches shortcuts directly to the focused widget's TypedShortcut and
// bypasses canvas-level callbacks — so this method must own the mark
// dispatch even though the bindings are also registered on the canvas (which
// is what tells the driver these key combos are shortcuts in the first
// place). Implements fyne.Shortcutable.
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
	case *desktop.CustomShortcut:
		if mark, ok := markForShortcut(sc); ok {
			e.toggleMark(mark)
		}
	}
}

// markForShortcut maps the registered CustomShortcuts back to their mark.
func markForShortcut(sc *desktop.CustomShortcut) (doc.Mark, bool) {
	for _, b := range MarkShortcutBindings() {
		ks, ok := b.Shortcut.(*desktop.CustomShortcut)
		if !ok {
			continue
		}
		if ks.Key() == sc.Key() && ks.Mod() == sc.Mod() {
			return b.Mark, true
		}
	}
	return 0, false
}

// MarkShortcuts is the list of custom shortcut bindings the app wires up on
// the window canvas. Each tuple is (shortcut, mark). Returned as a function
// to avoid an import cycle (desktop is internal to editor).
func MarkShortcutBindings() []MarkBinding {
	return []MarkBinding{
		{Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyB, Modifier: fyne.KeyModifierShortcutDefault}, Mark: doc.MarkBold},
		{Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyI, Modifier: fyne.KeyModifierShortcutDefault}, Mark: doc.MarkItalic},
		{Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyU, Modifier: fyne.KeyModifierShortcutDefault}, Mark: doc.MarkUnderline},
		{Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyE, Modifier: fyne.KeyModifierShortcutDefault}, Mark: doc.MarkCode},
		{Shortcut: &desktop.CustomShortcut{KeyName: fyne.KeyX, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}, Mark: doc.MarkStrike},
	}
}

// MarkBinding pairs a Fyne shortcut with the editor mark it should toggle.
type MarkBinding struct {
	Shortcut fyne.Shortcut
	Mark     doc.Mark
}

// ToggleMark is the public entry point invoked by canvas-level shortcut
// callbacks.
func (e *RichEditor) ToggleMark(m doc.Mark) { e.toggleMark(m) }

// toggleMark turns mark m on or off. With a non-empty selection: applies the
// inverse of whatever the start of the selection currently has, to all bytes
// in the range. With a collapsed caret: flips the pending-marks state so the
// next typed character inherits the toggle.
func (e *RichEditor) toggleMark(m doc.Mark) {
	e.mu.Lock()
	if e.sel.IsCollapsed() {
		current := doc.MarksAt(e.doc.Blocks[e.sel.Head.Path[0]], e.sel.Head.Offset)
		if e.pendingMarksSet {
			current = e.pendingMarks
		}
		e.pendingMarks = current.Toggle(m)
		e.pendingMarksSet = true
		e.mu.Unlock()
		// No content change, but refresh so any future "mark indicator" UI
		// could update. Skipping fireChanged because the doc didn't change.
		e.Refresh()
		return
	}
	lo, hi := e.selRange()
	// Same-block fast path. Cross-block applies block-by-block.
	if lo.Path[0] == hi.Path[0] {
		bi := lo.Path[0]
		set := !marksHaveAcross(e.doc.Blocks[bi], lo.Offset, hi.Offset, m)
		e.doc.Blocks[bi] = doc.ApplyMark(e.doc.Blocks[bi], lo.Offset, hi.Offset, m, set)
	} else {
		// Decide set/clear by inspecting the start of the selection.
		startBlk := e.doc.Blocks[lo.Path[0]]
		startEnd := len(startBlk.PlainText())
		set := !marksHaveAcross(startBlk, lo.Offset, startEnd, m)
		e.doc.Blocks[lo.Path[0]] = doc.ApplyMark(startBlk, lo.Offset, startEnd, m, set)
		for i := lo.Path[0] + 1; i < hi.Path[0]; i++ {
			full := len(e.doc.Blocks[i].PlainText())
			e.doc.Blocks[i] = doc.ApplyMark(e.doc.Blocks[i], 0, full, m, set)
		}
		e.doc.Blocks[hi.Path[0]] = doc.ApplyMark(e.doc.Blocks[hi.Path[0]], 0, hi.Offset, m, set)
	}
	e.pendingMarksSet = false
	e.mu.Unlock()
	e.invalidate()
}

// marksHaveAcross reports whether every byte in [from, to) of b has mark m.
// Used to decide whether toggle should set or clear.
func marksHaveAcross(b doc.Block, from, to int, m doc.Mark) bool {
	if from >= to {
		return false
	}
	consumed := 0
	for _, in := range b.Inlines {
		start := consumed
		end := consumed + len(in.Text)
		consumed = end
		if end <= from {
			continue
		}
		if start >= to {
			break
		}
		// This inline intersects [from, to).
		if !in.Marks.Has(m) {
			return false
		}
	}
	return true
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
