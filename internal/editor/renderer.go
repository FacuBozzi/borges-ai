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

	textObjs  []*canvas.Text     // pooled, one per styled run
	deco      []*canvas.Line     // underline / strikethrough lines
	selRects  []*canvas.Rectangle
	caret     *canvas.Rectangle
	bg        *canvas.Rectangle

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

func (r *editorRenderer) Layout(size fyne.Size) {
	r.e.mu.Lock()
	r.e.width = size.Width
	r.e.lines = layout(r.e.doc, size.Width)
	d := r.e.doc
	lines := r.e.lines
	sel := r.e.sel
	focused := r.e.focused
	r.e.mu.Unlock()

	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	r.syncContent(d, lines)
	r.syncSelectionRects(lines, sel)
	r.positionCaret(lines, sel, focused)
}

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

func (r *editorRenderer) Objects() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, 2+len(r.selRects)+len(r.textObjs)+len(r.deco))
	out = append(out, r.bg)
	for _, s := range r.selRects {
		out = append(out, s)
	}
	for _, t := range r.textObjs {
		out = append(out, t)
	}
	for _, ln := range r.deco {
		out = append(out, ln)
	}
	out = append(out, r.caret)
	return out
}

func (r *editorRenderer) Refresh() {
	r.bg.FillColor = theme.Color(theme.ColorNameBackground)
	r.bg.Refresh()
	r.caret.FillColor = theme.Color(theme.ColorNamePrimary)

	r.e.mu.Lock()
	if r.e.width > 0 {
		r.e.lines = layout(r.e.doc, r.e.width)
	}
	d := r.e.doc
	lines := r.e.lines
	sel := r.e.sel
	focused := r.e.focused
	r.e.mu.Unlock()

	r.syncContent(d, lines)
	r.syncSelectionRects(lines, sel)
	r.positionCaret(lines, sel, focused)
	for _, t := range r.textObjs {
		t.Refresh()
	}
	for _, ln := range r.deco {
		ln.Refresh()
	}
	for _, s := range r.selRects {
		s.Refresh()
	}
	r.caret.Refresh()
	canvas.Refresh(r.e)
}

// syncContent walks lines, decomposes each into styled runs, and updates the
// pooled canvas objects.
func (r *editorRenderer) syncContent(d *doc.Document, lines []visualLine) {
	fg := theme.Color(theme.ColorNameForeground)
	var runs []styleRun
	for _, ln := range lines {
		if ln.blockIdx < 0 || ln.blockIdx >= len(d.Blocks) {
			continue
		}
		runs = append(runs, decomposeLine(ln, d.Blocks[ln.blockIdx])...)
	}

	// Grow text pool.
	for len(r.textObjs) < len(runs) {
		t := canvas.NewText("", fg)
		t.TextSize = fontSize
		r.textObjs = append(r.textObjs, t)
	}
	// Count how many decoration lines we'll need (underline + strike per run).
	needDeco := 0
	for _, run := range runs {
		if run.marks.Has(doc.MarkUnderline) {
			needDeco++
		}
		if run.marks.Has(doc.MarkStrike) {
			needDeco++
		}
	}
	for len(r.deco) < needDeco {
		ln := canvas.NewLine(fg)
		ln.StrokeWidth = 1
		r.deco = append(r.deco, ln)
	}

	decoIdx := 0
	for i, run := range runs {
		t := r.textObjs[i]
		t.Text = run.text
		t.Color = fg
		t.TextSize = fontSize
		t.TextStyle = textStyleFor(run.marks)
		t.Move(fyne.NewPos(run.x, run.y))
		t.Resize(fyne.NewSize(run.w, run.h))

		if run.marks.Has(doc.MarkUnderline) {
			ln := r.deco[decoIdx]
			decoIdx++
			ln.StrokeColor = fg
			ln.StrokeWidth = 1
			y := run.y + run.h - 4
			ln.Position1 = fyne.NewPos(run.x, y)
			ln.Position2 = fyne.NewPos(run.x+run.w, y)
			ln.Show()
		}
		if run.marks.Has(doc.MarkStrike) {
			ln := r.deco[decoIdx]
			decoIdx++
			ln.StrokeColor = fg
			ln.StrokeWidth = 1
			y := run.y + run.h*0.55
			ln.Position1 = fyne.NewPos(run.x, y)
			ln.Position2 = fyne.NewPos(run.x+run.w, y)
			ln.Show()
		}
	}
	// Hide leftover pool entries.
	for i := len(runs); i < len(r.textObjs); i++ {
		r.textObjs[i].Text = ""
		r.textObjs[i].Move(fyne.NewPos(-10000, -10000))
	}
	for i := decoIdx; i < len(r.deco); i++ {
		r.deco[i].Hide()
	}
}

// syncSelectionRects draws a highlight rectangle for each visual line the
// selection spans.
func (r *editorRenderer) syncSelectionRects(lines []visualLine, sel doc.Selection) {
	highlight := theme.Color(theme.ColorNameSelection)
	rects := r.computeSelectionRects(lines, sel)

	for len(r.selRects) < len(rects) {
		s := canvas.NewRectangle(highlight)
		s.StrokeWidth = 0
		r.selRects = append(r.selRects, s)
	}
	for i, rc := range rects {
		s := r.selRects[i]
		s.FillColor = highlight
		s.Move(fyne.NewPos(rc.x, rc.y))
		s.Resize(fyne.NewSize(rc.w, rc.h))
		s.Show()
	}
	for i := len(rects); i < len(r.selRects); i++ {
		r.selRects[i].Hide()
	}
}

type selRect struct{ x, y, w, h float32 }

func (r *editorRenderer) computeSelectionRects(lines []visualLine, sel doc.Selection) []selRect {
	if sel.IsCollapsed() || len(lines) == 0 {
		return nil
	}
	lo, hi := sel.Anchor, sel.Head
	if positionLess(hi, lo) {
		lo, hi = hi, lo
	}
	var out []selRect
	for _, ln := range lines {
		if ln.blockIdx < lo.Path[0] || ln.blockIdx > hi.Path[0] {
			continue
		}
		startByte := ln.startByte
		endByte := ln.endByte
		if ln.blockIdx == lo.Path[0] && startByte < lo.Offset {
			startByte = lo.Offset
		}
		if ln.blockIdx == hi.Path[0] && endByte > hi.Offset {
			endByte = hi.Offset
		}
		if startByte >= endByte {
			if ln.blockIdx > lo.Path[0] && ln.blockIdx < hi.Path[0] && len(ln.text) == 0 {
				out = append(out, selRect{x: ln.x, y: ln.y, w: 8, h: ln.height})
			}
			continue
		}
		x1 := xForOffset(ln, startByte)
		x2 := xForOffset(ln, endByte)
		if ln.blockIdx < hi.Path[0] || endByte == ln.endByte {
			x2 = ln.x + ln.width
			if x2 == ln.x {
				x2 = ln.x + 4
			}
		}
		out = append(out, selRect{x: x1, y: ln.y, w: x2 - x1, h: ln.height})
	}
	return out
}

func (r *editorRenderer) positionCaret(lines []visualLine, sel doc.Selection, focused bool) {
	if !focused || len(lines) == 0 {
		r.caret.Hide()
		return
	}
	head := sel.Head
	li := lineForPosition(lines, head.Path[0], head.Offset)
	if li < 0 {
		r.caret.Hide()
		return
	}
	ln := lines[li]
	x := xForOffset(ln, head.Offset)
	r.caret.Move(fyne.NewPos(x, ln.y))
	r.caret.Resize(fyne.NewSize(caretWidth, ln.height))
	r.caret.Show()
}

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
