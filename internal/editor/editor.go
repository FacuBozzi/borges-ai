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
