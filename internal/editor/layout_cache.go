package editor

import "github.com/facubozzi/fyne-writer/internal/doc"

// wrapKey identifies a block's wrap result. Wrapping depends only on the
// block's plain text, its base font metrics (size + bold/mono — wrap measures
// at the base style, ignoring inline marks), its indent, and the content
// width. Keying on the plain-text string (a value) means cloned documents from
// undo/redo/restore hit the cache with no pointer bookkeeping.
type wrapKey struct {
	text     string
	fontSize float32
	indent   float32
	width    float32
	bold     bool
	mono     bool
}

// wrapCache memoizes wrapBlock output so layout doesn't re-wrap unchanged
// blocks on every Refresh. Entries hold the relative visualLines (before x/y/
// height/style stamping); the layout loop re-stamps absolute geometry on every
// call, so only the width-sensitive wrap result is cached.
type wrapCache struct {
	width   float32
	entries map[wrapKey][]visualLine
}

// layoutCached is the cached equivalent of layout(d, contentWidth). It mirrors
// layout's stamping loop but memoizes wrapBlock per block. Called only from the
// renderer's Layout/Refresh (main thread), so it needs no locking.
func (e *RichEditor) layoutCached(d *doc.Document, contentWidth float32) []visualLine {
	if contentWidth < minContentWidth {
		contentWidth = minContentWidth
	}
	c := e.wrapCache
	if c == nil || c.width != contentWidth {
		// A width change invalidates every block's wrap; rebuild the map.
		c = &wrapCache{width: contentWidth, entries: map[wrapKey][]visualLine{}}
		e.wrapCache = c
	}

	var lines []visualLine
	y := editorVPadding
	for bi, b := range d.Blocks {
		style := doc.StyleForBlock(b)
		ts := blockTextStyle(style)
		k := wrapKey{
			text:     b.PlainText(),
			fontSize: style.FontSize,
			indent:   style.Indent,
			width:    contentWidth,
			bold:     ts.Bold,
			mono:     ts.Monospace,
		}
		wrapped, ok := c.entries[k]
		if !ok {
			wrapped = wrapBlock(bi, b, style, contentWidth)
			c.entries[k] = wrapped
		}
		// Copy each cached (relative) line and stamp absolute geometry +
		// blockIdx for this block. The same text at a different index reuses
		// the wrap but gets its own blockIdx.
		for _, rel := range wrapped {
			ln := rel
			ln.blockIdx = bi
			ln.x = editorHPadding + style.Indent
			ln.y = y
			ln.height = style.LineHeight
			ln.style = style
			lines = append(lines, ln)
			y += style.LineHeight
		}
		y += style.GapBelow
	}
	return lines
}

// InvalidateLayoutCache drops the memoized wrap results. Call when something
// that affects text measurement but not block content changes — notably a font
// or theme swap, since the wrap key has no notion of font family.
func (e *RichEditor) InvalidateLayoutCache() { e.wrapCache = nil }
