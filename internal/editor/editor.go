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
	caret    doc.Position
	focused  bool
	onChange func()
	lines    []visualLine // cached layout; rebuilt on doc/width change
	width    float32      // last width used for layout

	// Public: width used by the caret movement code to remember a "preferred
	// column" when moving up/down across lines of different lengths.
	preferredX float32
}

// New creates a RichEditor populated with the given document. Pass doc.New()
// for an empty placeholder paragraph.
func New(d *doc.Document) *RichEditor {
	e := &RichEditor{doc: d, caret: d.Start()}
	e.ExtendBaseWidget(e)
	return e
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
	e.caret = d.Start()
	e.lines = nil
	e.preferredX = 0
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
