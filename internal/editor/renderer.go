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

	textObjs    []*canvas.Text      // pooled, one per styled run
	deco        []*canvas.Line      // underline / strikethrough lines
	issueDeco   []*canvas.Line      // wavy underline segments for AI-check issues
	commentBGs  []*canvas.Rectangle // yellow highlight rects for comment anchors
	gutterText  []*canvas.Text      // list bullets / numbers + block prefixes
	blockBars   []*canvas.Rectangle // left-bar for quotes
	blockBGs    []*canvas.Rectangle // background for code blocks
	hrLines     []*canvas.Line      // horizontal rules
	selRects    []*canvas.Rectangle
	caret       *canvas.Rectangle
	bg          *canvas.Rectangle

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
	issues := append([]Issue(nil), r.e.issues...)
	comments := append([]Comment(nil), r.e.comments...)
	r.e.mu.Unlock()

	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	r.syncContent(d, lines)
	r.syncBlockDecorations(d, lines)
	r.syncCommentHighlights(lines, comments)
	r.syncIssueUnderlines(lines, issues)
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
	out := make([]fyne.CanvasObject, 0, 2+len(r.selRects)+len(r.textObjs)+len(r.deco)+len(r.issueDeco)+len(r.commentBGs)+len(r.gutterText)+len(r.blockBars)+len(r.blockBGs)+len(r.hrLines))
	out = append(out, r.bg)
	for _, s := range r.blockBGs {
		out = append(out, s)
	}
	for _, s := range r.commentBGs {
		out = append(out, s)
	}
	for _, s := range r.selRects {
		out = append(out, s)
	}
	for _, s := range r.blockBars {
		out = append(out, s)
	}
	for _, t := range r.gutterText {
		out = append(out, t)
	}
	for _, t := range r.textObjs {
		out = append(out, t)
	}
	for _, ln := range r.deco {
		out = append(out, ln)
	}
	for _, ln := range r.issueDeco {
		out = append(out, ln)
	}
	for _, ln := range r.hrLines {
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
	issues := append([]Issue(nil), r.e.issues...)
	comments := append([]Comment(nil), r.e.comments...)
	r.e.mu.Unlock()

	r.syncContent(d, lines)
	r.syncBlockDecorations(d, lines)
	r.syncCommentHighlights(lines, comments)
	r.syncIssueUnderlines(lines, issues)
	r.syncSelectionRects(lines, sel)
	r.positionCaret(lines, sel, focused)
	for _, t := range r.textObjs {
		t.Refresh()
	}
	for _, t := range r.gutterText {
		t.Refresh()
	}
	for _, ln := range r.deco {
		ln.Refresh()
	}
	for _, ln := range r.issueDeco {
		ln.Refresh()
	}
	for _, s := range r.blockBars {
		s.Refresh()
	}
	for _, s := range r.blockBGs {
		s.Refresh()
	}
	for _, s := range r.commentBGs {
		s.Refresh()
	}
	for _, ln := range r.hrLines {
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
		t.TextSize = run.fontSize
		t.TextStyle = run.style
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

// syncBlockDecorations draws block-level affordances: list bullets/numbers
// in the gutter, the left-bar on blockquotes, the background fill on code
// blocks, and the centered line of an HR.
func (r *editorRenderer) syncBlockDecorations(d *doc.Document, lines []visualLine) {
	// Build a per-block y-range table from the visual lines.
	type rangeY struct{ y, h float32 }
	blockY := map[int]rangeY{}
	for _, ln := range lines {
		if ln.blockIdx < 0 || ln.blockIdx >= len(d.Blocks) {
			continue
		}
		ry, ok := blockY[ln.blockIdx]
		if !ok {
			blockY[ln.blockIdx] = rangeY{y: ln.y, h: ln.height}
			continue
		}
		// extend
		bottom := ry.y + ry.h
		newBottom := ln.y + ln.height
		if newBottom > bottom {
			bottom = newBottom
		}
		blockY[ln.blockIdx] = rangeY{y: ry.y, h: bottom - ry.y}
	}

	fg := theme.Color(theme.ColorNameForeground)
	muted := theme.Color(theme.ColorNamePlaceHolder)
	codeBG := theme.Color(theme.ColorNameInputBackground)

	var gutters []*canvas.Text
	var bars []*canvas.Rectangle
	var bgs []*canvas.Rectangle
	var hrs []*canvas.Line

	for bi, b := range d.Blocks {
		ry, ok := blockY[bi]
		if !ok {
			continue
		}
		style := doc.StyleForBlock(b)
		switch b.Type {
		case doc.BlockListItem:
			marker := "• "
			if listKind(b) == "ordered" {
				marker = listMarker(b)
			}
			t := r.takeGutter(&gutters)
			t.Text = marker
			t.Color = fg
			t.TextSize = style.FontSize
			t.TextStyle = fyne.TextStyle{}
			// Bullet sits just to the left of where text starts.
			mw := fyne.MeasureText(marker, style.FontSize, fyne.TextStyle{}).Width
			t.Move(fyne.NewPos(editorHPadding+style.Indent-mw-2, ry.y))
			t.Resize(fyne.NewSize(mw, style.LineHeight))
		case doc.BlockQuote:
			rc := r.takeBar(&bars)
			rc.FillColor = muted
			rc.StrokeWidth = 0
			rc.Move(fyne.NewPos(editorHPadding, ry.y))
			rc.Resize(fyne.NewSize(3, ry.h))
		case doc.BlockCodeBlock:
			rc := r.takeBG(&bgs)
			rc.FillColor = codeBG
			rc.StrokeWidth = 0
			rc.Move(fyne.NewPos(editorHPadding-8, ry.y-4))
			rc.Resize(fyne.NewSize(r.e.width-2*(editorHPadding-8), ry.h+8))
		case doc.BlockHR:
			ln := r.takeHR(&hrs)
			ln.StrokeColor = muted
			ln.StrokeWidth = 1
			mid := ry.y + ry.h/2
			ln.Position1 = fyne.NewPos(editorHPadding, mid)
			ln.Position2 = fyne.NewPos(r.e.width-editorHPadding, mid)
		}
	}

	// Hide unused pool entries.
	r.gutterText = trimAndHideText(r.gutterText, len(gutters))
	r.blockBars = trimAndHideRects(r.blockBars, len(bars))
	r.blockBGs = trimAndHideRects(r.blockBGs, len(bgs))
	r.hrLines = trimAndHideLines(r.hrLines, len(hrs))
}

func listMarker(b doc.Block) string {
	idx := listIndexFromMeta(b)
	if idx <= 0 {
		idx = 1
	}
	return formatOrderedMarker(idx)
}

func listKind(b doc.Block) string {
	if v, ok := b.Meta["list_kind"].(string); ok {
		return v
	}
	return "bullet"
}

func listIndexFromMeta(b doc.Block) int {
	if v, ok := b.Meta["index"].(int); ok {
		return v
	}
	return 0
}

func formatOrderedMarker(idx int) string {
	// Avoid fmt for the hot path; small integers only.
	const digits = "0123456789"
	if idx < 10 {
		return string(digits[idx]) + ". "
	}
	buf := make([]byte, 0, 6)
	for idx > 0 {
		buf = append([]byte{digits[idx%10]}, buf...)
		idx /= 10
	}
	return string(buf) + ". "
}

func (r *editorRenderer) takeGutter(used *[]*canvas.Text) *canvas.Text {
	idx := len(*used)
	if idx >= len(r.gutterText) {
		t := canvas.NewText("", theme.Color(theme.ColorNameForeground))
		r.gutterText = append(r.gutterText, t)
	}
	*used = append(*used, r.gutterText[idx])
	return r.gutterText[idx]
}

func (r *editorRenderer) takeBar(used *[]*canvas.Rectangle) *canvas.Rectangle {
	idx := len(*used)
	if idx >= len(r.blockBars) {
		rc := canvas.NewRectangle(theme.Color(theme.ColorNamePlaceHolder))
		r.blockBars = append(r.blockBars, rc)
	}
	*used = append(*used, r.blockBars[idx])
	return r.blockBars[idx]
}

func (r *editorRenderer) takeBG(used *[]*canvas.Rectangle) *canvas.Rectangle {
	idx := len(*used)
	if idx >= len(r.blockBGs) {
		rc := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
		r.blockBGs = append(r.blockBGs, rc)
	}
	*used = append(*used, r.blockBGs[idx])
	return r.blockBGs[idx]
}

func (r *editorRenderer) takeHR(used *[]*canvas.Line) *canvas.Line {
	idx := len(*used)
	if idx >= len(r.hrLines) {
		ln := canvas.NewLine(theme.Color(theme.ColorNamePlaceHolder))
		r.hrLines = append(r.hrLines, ln)
	}
	*used = append(*used, r.hrLines[idx])
	return r.hrLines[idx]
}

// trim helpers: keep the slice length the same but hide entries past `used`.
func trimAndHideText(pool []*canvas.Text, used int) []*canvas.Text {
	for i := used; i < len(pool); i++ {
		pool[i].Text = ""
		pool[i].Move(fyne.NewPos(-10000, -10000))
	}
	return pool
}

func trimAndHideRects(pool []*canvas.Rectangle, used int) []*canvas.Rectangle {
	for i := used; i < len(pool); i++ {
		pool[i].Hide()
	}
	for i := 0; i < used; i++ {
		pool[i].Show()
	}
	return pool
}

func trimAndHideLines(pool []*canvas.Line, used int) []*canvas.Line {
	for i := used; i < len(pool); i++ {
		pool[i].Hide()
	}
	for i := 0; i < used; i++ {
		pool[i].Show()
	}
	return pool
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

// syncIssueUnderlines paints a wavy underline beneath every visual-line slice
// that overlaps an AI-check issue's anchor range. Uses a separate canvas.Line
// pool so it doesn't interfere with the per-mark underline/strike pool.
func (r *editorRenderer) syncIssueUnderlines(lines []visualLine, issues []Issue) {
	col := theme.Color(theme.ColorNameError)
	const amplitude float32 = 1.6
	const period float32 = 5

	// Collect zigzag segments first, then allocate the line pool to fit.
	type segPair struct{ p1, p2 fyne.Position }
	var segs []segPair

	for _, iss := range issues {
		start := iss.Offset
		end := iss.Offset + iss.Length
		if end <= start {
			continue
		}
		for _, ln := range lines {
			if ln.blockIdx != iss.BlockIndex {
				continue
			}
			lo := start
			if ln.startByte > lo {
				lo = ln.startByte
			}
			hi := end
			if ln.endByte < hi {
				hi = ln.endByte
			}
			if lo >= hi {
				continue
			}
			x1 := xForOffset(ln, lo)
			x2 := xForOffset(ln, hi)
			if x2 <= x1 {
				continue
			}
			y := ln.y + ln.height - 3
			// Build a zigzag from x1 to x2 alternating above/below y.
			x := x1
			up := true
			for x < x2 {
				next := x + period/2
				if next > x2 {
					next = x2
				}
				var y1, y2 float32
				if up {
					y1, y2 = y+amplitude, y-amplitude
				} else {
					y1, y2 = y-amplitude, y+amplitude
				}
				segs = append(segs, segPair{
					p1: fyne.NewPos(x, y1),
					p2: fyne.NewPos(next, y2),
				})
				x = next
				up = !up
			}
		}
	}

	for len(r.issueDeco) < len(segs) {
		ln := canvas.NewLine(col)
		ln.StrokeWidth = 1.4
		r.issueDeco = append(r.issueDeco, ln)
	}
	for i, s := range segs {
		ln := r.issueDeco[i]
		ln.StrokeColor = col
		ln.StrokeWidth = 1.4
		ln.Position1 = s.p1
		ln.Position2 = s.p2
		ln.Show()
	}
	for i := len(segs); i < len(r.issueDeco); i++ {
		r.issueDeco[i].Hide()
	}
}

// commentHighlightColor is a soft warm yellow with low alpha — it reads as
// "annotated" on both light and dark backgrounds.
var commentHighlightColor = color.NRGBA{0xFF, 0xE5, 0x6B, 0x4D}

// syncCommentHighlights paints a translucent yellow rectangle behind every
// visual-line slice that overlaps a comment's anchor range. Reuses the same
// per-visual-line geometry as syncSelectionRects but draws behind selection
// so a comment + selection overlap still shows the selection color.
func (r *editorRenderer) syncCommentHighlights(lines []visualLine, comments []Comment) {
	var rects []selRect
	for _, c := range comments {
		start := c.Offset
		end := c.Offset + c.Length
		if end <= start {
			continue
		}
		for _, ln := range lines {
			if ln.blockIdx != c.BlockIndex {
				continue
			}
			lo := start
			if ln.startByte > lo {
				lo = ln.startByte
			}
			hi := end
			if ln.endByte < hi {
				hi = ln.endByte
			}
			if lo >= hi {
				continue
			}
			x1 := xForOffset(ln, lo)
			x2 := xForOffset(ln, hi)
			if x2 <= x1 {
				continue
			}
			rects = append(rects, selRect{x: x1, y: ln.y, w: x2 - x1, h: ln.height})
		}
	}

	for len(r.commentBGs) < len(rects) {
		s := canvas.NewRectangle(commentHighlightColor)
		s.StrokeWidth = 0
		r.commentBGs = append(r.commentBGs, s)
	}
	for i, rc := range rects {
		s := r.commentBGs[i]
		s.FillColor = commentHighlightColor
		s.Move(fyne.NewPos(rc.x, rc.y))
		s.Resize(fyne.NewSize(rc.w, rc.h))
		s.Show()
	}
	for i := len(rects); i < len(r.commentBGs); i++ {
		r.commentBGs[i].Hide()
	}
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
