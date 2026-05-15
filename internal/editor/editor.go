// Package editor implements the custom WYSIWYG rich-text widget.
//
// M1 scope: paragraphs only, typing, caret, click-to-position, file save/load.
// Selection, marks, and block formatting arrive in M2.
package editor

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// Visual + spacing constants. Read from theme in a later milestone.
const (
	fontSize          float32 = 15
	lineHeight        float32 = 24
	paragraphGap      float32 = 8
	editorHPadding    float32 = 32
	editorVPadding    float32 = 24
	caretWidth        float32 = 1.5
	minContentWidth   float32 = 300
)

// RichEditor is the editable rich-text widget. It implements fyne.Focusable +
// fyne.Tappable so it can receive keyboard input and reposition the caret on
// click.
type RichEditor struct {
	widget.BaseWidget

	mu       sync.Mutex
	doc      *doc.Document
	sel      doc.Selection
	focused  bool
	onChange func()
	lines    []visualLine // cached layout; rebuilt on doc/width change
	width    float32      // last width used for layout

	// preferredX is the screen X the caret would like to occupy when moving
	// up/down across lines of different lengths. Reset to -1 by any motion
	// other than caretUp/caretDown.
	preferredX float32

	// dragAnchor is the selection anchor captured at the start of a drag, so
	// each Dragged event extends from the original click rather than the
	// previous drag position.
	dragAnchor doc.Position
	dragging   bool

	// shiftHeld tracks whether either Shift key is currently pressed, so
	// arrow-key motion can extend the selection. Updated by KeyDown/KeyUp.
	shiftHeld bool
	// modHeld tracks whether cmd/ctrl/alt is currently held. When true,
	// TypedRune is suppressed because Fyne's glfw driver can deliver a char
	// event for shortcut keystrokes (e.g. cmd+K → 'k') even when the
	// shortcut itself fires; without this guard the editor would insert the
	// letter and overwrite the selection.
	modHeld bool

	// pendingMarks is the mark-set the next typed rune will inherit, set by
	// mark-toggle shortcuts (cmd+B etc.) while the selection is collapsed.
	// Cleared by any caret movement.
	pendingMarks    doc.Mark
	pendingMarksSet bool

	// undo/redo stacks. Lazily allocated on first commit.
	undo *undoStack
	redo *undoStack

	// activeAI is non-zero while an AI streaming replacement is in flight.
	// Used to dedup concurrent replacements.
	activeAI int64

	// ctxExtender, when set by the app layer, supplies extra context-menu
	// items (AI actions). See SetContextMenuExtender.
	ctxExtender ContextMenuExtender

	// issues are the active AI-check hints with resolved byte ranges. Edits
	// to the document run them through validateIssues() and drop any whose
	// anchor text no longer matches.
	issues          []Issue
	onIssuesChanged func()

	// comments are user-authored comments anchored to byte ranges. Same
	// invalidation flow as issues; rendered with a yellow background fill
	// instead of a wavy underline.
	comments          []Comment
	onCommentsChanged func()
}

// New creates a RichEditor populated with the given document. Pass doc.New()
// for an empty placeholder paragraph.
func New(d *doc.Document) *RichEditor {
	start := d.Start()
	e := &RichEditor{doc: d, sel: doc.Selection{Anchor: start, Head: start}}
	e.ExtendBaseWidget(e)
	return e
}

// caret returns the live caret position (the head of the selection).
func (e *RichEditor) caret() doc.Position { return e.sel.Head }

// HasSelection reports whether the current selection spans any text.
func (e *RichEditor) HasSelection() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return !e.sel.IsCollapsed()
}

// SelectionText returns the plain text of the current selection (or "" when
// collapsed). Safe to call from any goroutine.
func (e *RichEditor) SelectionText() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.selectionText()
}

// SelectionSingleBlockRange returns the byte range of the current selection
// when it lies inside one block. ok is false if there is no selection or the
// selection spans multiple blocks. Safe to call from any goroutine.
func (e *RichEditor) SelectionSingleBlockRange() (blockIdx, start, end int, ok bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sel.IsCollapsed() {
		return 0, 0, 0, false
	}
	lo, hi := e.selRange()
	if len(lo.Path) == 0 || len(hi.Path) == 0 || lo.Path[0] != hi.Path[0] {
		return 0, 0, 0, false
	}
	return lo.Path[0], lo.Offset, hi.Offset, true
}

// SetDocMeta replaces the document's metadata (background instructions etc.)
// and marks the doc dirty.
func (e *RichEditor) SetDocMeta(m doc.DocMeta) {
	e.mu.Lock()
	e.doc.Meta = m
	e.mu.Unlock()
	e.fireChanged()
}

// setCaret collapses the selection to a single point.
func (e *RichEditor) setCaret(p doc.Position) {
	e.sel = doc.Selection{Anchor: p, Head: p}
}

// SelectRange sets the selection to the given byte range inside one block.
// Used by the "Jump to comment" sidebar action. No-op when bounds are invalid.
func (e *RichEditor) SelectRange(blockIdx, start, end int) {
	e.mu.Lock()
	if blockIdx < 0 || blockIdx >= len(e.doc.Blocks) {
		e.mu.Unlock()
		return
	}
	plain := e.doc.Blocks[blockIdx].PlainText()
	if start < 0 || end > len(plain) || start > end {
		e.mu.Unlock()
		return
	}
	anchor := doc.Position{Path: []int{blockIdx}, Inline: 0, Offset: start}
	head := doc.Position{Path: []int{blockIdx}, Inline: 0, Offset: end}
	e.sel = doc.Selection{Anchor: anchor, Head: head}
	e.preferredX = -1
	e.mu.Unlock()
	e.Refresh()
}

// Document returns the current document. Caller must not mutate concurrently
// with editor input.
func (e *RichEditor) Document() *doc.Document {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.doc
}

// SetDocument replaces the current document and resets the caret.
func (e *RichEditor) SetDocument(d *doc.Document) {
	e.mu.Lock()
	e.doc = d
	start := d.Start()
	e.sel = doc.Selection{Anchor: start, Head: start}
	e.lines = nil
	e.preferredX = -1
	e.mu.Unlock()
	e.Refresh()
	e.fireChanged()
}

// ReplaceDocument is SetDocument but records the prior state as a single undo
// entry, so cmd+Z reverts the swap. Used by the M5 version-restore flow.
func (e *RichEditor) ReplaceDocument(d *doc.Document) {
	e.mu.Lock()
	e.commitUndo(undoKindOther)
	e.doc = d
	start := d.Start()
	e.sel = doc.Selection{Anchor: start, Head: start}
	e.lines = nil
	e.preferredX = -1
	e.mu.Unlock()
	e.Refresh()
	e.fireChanged()
}

// OnChanged registers a callback fired whenever the document is mutated.
// Used by the app to mark the title bar dirty.
func (e *RichEditor) OnChanged(fn func()) { e.onChange = fn }

func (e *RichEditor) fireChanged() {
	if e.onChange != nil {
		e.onChange()
	}
}

// CreateRenderer is the Fyne hook that builds the widget renderer. Both the
// renderer and the input handlers live in their own files in this package.
func (e *RichEditor) CreateRenderer() fyne.WidgetRenderer {
	r := newRenderer(e)
	return r
}

// FocusGained / FocusLost satisfy fyne.Focusable.
func (e *RichEditor) FocusGained() { e.focused = true; e.Refresh() }
func (e *RichEditor) FocusLost()   { e.focused = false; e.Refresh() }
