package editor

import (
	"image/color"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

const caretBlinkPeriod = 530 * time.Millisecond

type editorRenderer struct {
	e *RichEditor

	textObjs []*canvas.Text
	caret    *canvas.Rectangle
	bg       *canvas.Rectangle

	// Caret blink state. running is a sticky flag to prevent multiple goroutines.
	running atomic.Bool
}

func newRenderer(e *RichEditor) *editorRenderer {
	r := &editorRenderer{
		e:     e,
		caret: canvas.NewRectangle(theme.Color(theme.ColorNamePrimary)),
		bg:    canvas.NewRectangle(theme.Color(theme.ColorNameBackground)),
	}
	r.caret.StrokeWidth = 0
	r.startBlink()
	return r
}

func (r *editorRenderer) Destroy() {}

// Layout is the parent's request to lay out the widget at the given size.
// We word-wrap the document at this width and rebuild the canvas objects.
func (r *editorRenderer) Layout(size fyne.Size) {
	r.e.mu.Lock()
	r.e.width = size.Width
	r.e.lines = layout(r.e.doc, size.Width)
	lines := r.e.lines
	caret := r.e.caret
	focused := r.e.focused
	r.e.mu.Unlock()

	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	r.syncTextObjects(lines)
	r.positionCaret(lines, caret, focused)
}

// MinSize reports the height needed to render the whole document plus
// margins. The width is a generous minimum so the editor isn't squished by
// other panels; the parent ScrollContainer can grant more.
func (r *editorRenderer) MinSize() fyne.Size {
	r.e.mu.Lock()
	lines := r.e.lines
	r.e.mu.Unlock()
	height := editorVPadding * 2
	if len(lines) > 0 {
		last := lines[len(lines)-1]
		height = last.y + last.height + editorVPadding
	}
	return fyne.NewSize(minContentWidth, height)
}

// Objects returns the list of canvas primitives to composite, in z-order
// (back to front).
func (r *editorRenderer) Objects() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, 2+len(r.textObjs))
	out = append(out, r.bg)
	for _, t := range r.textObjs {
		out = append(out, t)
	}
	out = append(out, r.caret)
	return out
}

// Refresh recolors objects (e.g. after theme change) and re-renders.
func (r *editorRenderer) Refresh() {
	r.bg.FillColor = theme.Color(theme.ColorNameBackground)
	r.bg.Refresh()
	r.caret.FillColor = theme.Color(theme.ColorNamePrimary)

	r.e.mu.Lock()
	if r.e.width > 0 {
		r.e.lines = layout(r.e.doc, r.e.width)
	}
	lines := r.e.lines
	caret := r.e.caret
	focused := r.e.focused
	r.e.mu.Unlock()

	r.syncTextObjects(lines)
	r.positionCaret(lines, caret, focused)
	for _, t := range r.textObjs {
		t.Refresh()
	}
	r.caret.Refresh()
	canvas.Refresh(r.e)
}

// syncTextObjects resizes the pool of canvas.Text objects to match the line
// table and updates each text's content and position.
func (r *editorRenderer) syncTextObjects(lines []visualLine) {
	fg := theme.Color(theme.ColorNameForeground)
	for len(r.textObjs) < len(lines) {
		t := canvas.NewText("", fg)
		t.TextSize = fontSize
		t.TextStyle = fyne.TextStyle{}
		r.textObjs = append(r.textObjs, t)
	}
	for i, ln := range lines {
		t := r.textObjs[i]
		t.Text = ln.text
		t.Color = fg
		t.TextSize = fontSize
		t.Move(fyne.NewPos(ln.x, ln.y))
		t.Resize(fyne.NewSize(ln.width, ln.height))
	}
	// Hide unused objects by clearing text so they take no visible space.
	for i := len(lines); i < len(r.textObjs); i++ {
		r.textObjs[i].Text = ""
		r.textObjs[i].Move(fyne.NewPos(-10000, -10000))
	}
}

func (r *editorRenderer) positionCaret(lines []visualLine, caret doc.Position, focused bool) {
	if !focused || len(lines) == 0 || len(caret.Path) == 0 {
		r.caret.Hide()
		return
	}
	li := lineForPosition(lines, caret.Path[0], caret.Offset)
	if li < 0 {
		r.caret.Hide()
		return
	}
	ln := lines[li]
	x := xForOffset(ln, caret.Offset)
	r.caret.Move(fyne.NewPos(x, ln.y))
	r.caret.Resize(fyne.NewSize(caretWidth, ln.height))
	r.caret.Show()
}

// startBlink launches the caret blink loop the first time the renderer is
// created. The loop toggles the caret's visibility on the canvas thread.
func (r *editorRenderer) startBlink() {
	if !r.running.CompareAndSwap(false, true) {
		return
	}
	go func() {
		ticker := time.NewTicker(caretBlinkPeriod)
		defer ticker.Stop()
		visible := true
		for range ticker.C {
			r.e.mu.Lock()
			focused := r.e.focused
			r.e.mu.Unlock()
			if !focused {
				if !visible {
					fyne.Do(func() {
						r.caret.Show()
						canvas.Refresh(r.caret)
					})
					visible = true
				}
				continue
			}
			visible = !visible
			nextVisible := visible
			fyne.Do(func() {
				if nextVisible {
					r.caret.FillColor = theme.Color(theme.ColorNamePrimary)
				} else {
					r.caret.FillColor = color.Transparent
				}
				canvas.Refresh(r.caret)
			})
		}
	}()
}

