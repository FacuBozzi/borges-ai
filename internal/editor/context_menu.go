package editor

import (
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// ContextMenuTarget describes what the user right-clicked. The app's
// extension builder uses this to decide which AI items to add.
type ContextMenuTarget struct {
	HasSelection  bool
	SelectionText string
	Word          string // word at the click position (when no selection)
	Sentence      string // surrounding sentence
	// ReplaceWord swaps the current word with the given replacement. Set on
	// every target where Word is non-empty so the synonyms picker can call
	// it directly. Caller must run it on the UI thread.
	ReplaceWord func(replacement string)
}

// ContextMenuExtender is the app-supplied function that returns extra menu
// items to splice into the editor's right-click menu. Items are placed
// above the built-in Cut/Copy/Paste section.
type ContextMenuExtender func(ContextMenuTarget) []*fyne.MenuItem

// SetContextMenuExtender installs (or clears) the extension hook.
func (e *RichEditor) SetContextMenuExtender(fn ContextMenuExtender) {
	e.mu.Lock()
	e.ctxExtender = fn
	e.mu.Unlock()
}

// buildContextMenu constructs the right-click menu. Items operate on the
// current selection (Cut/Copy require non-empty; Paste requires clipboard
// content; Format submenu toggles marks). App-supplied AI items are
// prepended via the extension hook (e.g. Synonyms, Paraphrase).
func (e *RichEditor) buildContextMenu() *fyne.Menu {
	canvas := fyne.CurrentApp().Driver().CanvasForObject(e)
	var clipboard fyne.Clipboard
	if canvas != nil {
		clipboard = fyne.CurrentApp().Clipboard()
	}

	e.mu.Lock()
	hasSelection := !e.sel.IsCollapsed()
	extender := e.ctxExtender
	target := ContextMenuTarget{HasSelection: hasSelection}
	if hasSelection {
		target.SelectionText = e.selectionText()
	} else {
		target.Word, target.Sentence = e.wordAndSentenceAtCaret()
		if target.Word != "" {
			ed := e
			wordStart, wordEnd := e.wordRangeAtCaret()
			bi := e.sel.Head.Path[0]
			target.ReplaceWord = func(repl string) {
				ed.mu.Lock()
				ed.commitUndo(undoKindOther)
				ed.doc.Blocks[bi] = doc.DeleteRange(ed.doc.Blocks[bi], wordStart, wordEnd)
				ed.doc.Blocks[bi] = doc.InsertText(ed.doc.Blocks[bi], wordStart, repl, 0)
				ed.setCaret(doc.Position{Path: []int{bi}, Inline: 0, Offset: wordStart + len(repl)})
				ed.preferredX = -1
				ed.mu.Unlock()
				ed.invalidate()
			}
		}
	}
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

	items := []*fyne.MenuItem{}
	if extender != nil {
		extra := extender(target)
		if len(extra) > 0 {
			items = append(items, extra...)
			items = append(items, fyne.NewMenuItemSeparator())
		}
	}
	items = append(items,
		cut, copy, paste,
		fyne.NewMenuItemSeparator(),
		selectAllItem,
		fyne.NewMenuItemSeparator(),
		format,
	)
	return fyne.NewMenu("", items...)
}

// wordRangeAtCaret returns the byte range [start, end) of the word the
// caret is currently inside. Empty range when caret isn't in a word.
func (e *RichEditor) wordRangeAtCaret() (int, int) {
	bi := e.sel.Head.Path[0]
	if bi < 0 || bi >= len(e.doc.Blocks) {
		return 0, 0
	}
	text := e.blockText(bi)
	off := e.sel.Head.Offset
	if off < 0 || off > len(text) {
		return 0, 0
	}
	start := off
	for start > 0 && isWordByte(text[start-1]) {
		start--
	}
	end := off
	for end < len(text) && isWordByte(text[end]) {
		end++
	}
	return start, end
}

// wordAndSentenceAtCaret returns the word at the caret and the surrounding
// sentence (stretches to the nearest sentence terminator or block edge).
func (e *RichEditor) wordAndSentenceAtCaret() (string, string) {
	bi := e.sel.Head.Path[0]
	if bi < 0 || bi >= len(e.doc.Blocks) {
		return "", ""
	}
	text := e.blockText(bi)
	ws, we := e.wordRangeAtCaret()
	if ws == we {
		return "", ""
	}
	// Expand to sentence: scan left for a terminator + space (or block start),
	// scan right for a terminator (or block end).
	ss := ws
	for ss > 0 {
		r := text[ss-1]
		if isSentenceTerminator(r) && (ss == 1 || text[ss-2] != '.') {
			break
		}
		ss--
	}
	for ss < len(text) && (text[ss] == ' ' || text[ss] == '\n' || isSentenceTerminator(text[ss])) {
		ss++
	}
	se := we
	for se < len(text) {
		if isSentenceTerminator(text[se]) {
			se++
			break
		}
		se++
	}
	if ss > we {
		ss = ws
	}
	if se < ws {
		se = we
	}
	if se > len(text) {
		se = len(text)
	}
	return text[ws:we], text[ss:se]
}

func isWordByte(b byte) bool {
	r := rune(b)
	return unicode.IsLetter(r) || unicode.IsDigit(r) || b == '\''
}

func isSentenceTerminator(b byte) bool {
	return b == '.' || b == '!' || b == '?'
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
