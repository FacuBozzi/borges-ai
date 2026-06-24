package editor

import (
	"sync/atomic"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// AIReplace is a handle returned by BeginAIReplace. Callers push streamed
// text via Append, then end the operation with Commit or Cancel.
//
// The whole replacement collapses to a single undo step regardless of how
// many Append calls happen.
type AIReplace struct {
	editor   *RichEditor
	id       int64
	from     doc.Position
	to       doc.Position // updated as we stream
	original string
	done     atomic.Bool
}

// BeginAIReplace captures the current selection (or whole doc if collapsed)
// as the replacement target, snapshots the original text for cancellation,
// pushes a single undo entry, and immediately deletes the range so the
// caret sits where the streamed text will land.
//
// If there is no selection the call is a no-op and returns nil.
func (e *RichEditor) BeginAIReplace() *AIReplace {
	e.mu.Lock()
	if e.sel.IsCollapsed() {
		e.mu.Unlock()
		return nil
	}
	lo, _ := e.selRange()
	original := e.selectionText()
	e.commitUndo(undoKindOther)
	e.deleteSelection()
	// After deleteSelection, the caret sits at lo.
	handle := &AIReplace{
		editor:   e,
		id:       nextAIID(),
		from:     lo,
		to:       lo,
		original: original,
	}
	e.activeAI = handle.id
	e.mu.Unlock()
	e.invalidate()
	return handle
}

// Append inserts a streamed chunk at the end of the in-progress replacement.
// Safe to call from any goroutine.
func (h *AIReplace) Append(chunk string) {
	if h == nil || chunk == "" || h.done.Load() {
		return
	}
	e := h.editor
	e.mu.Lock()
	if e.activeAI != h.id {
		e.mu.Unlock()
		return
	}
	// Insert chunk at the stream cursor (no marks for AI output for now).
	bi := h.to.Path[0]
	start := h.to.Offset
	e.doc.Blocks[bi] = doc.InsertText(e.doc.Blocks[bi], start, chunk, 0)
	// Keep the user's caret/selection at the same logical spot instead of
	// dragging it along with the stream — they may be working elsewhere while
	// the AI runs in the background.
	e.sel.Anchor = shiftPos(e.sel.Anchor, bi, start, 0, len(chunk))
	e.sel.Head = shiftPos(e.sel.Head, bi, start, 0, len(chunk))
	h.to.Offset = start + len(chunk)
	e.mu.Unlock()
	e.invalidate()
}

// Commit finalizes the replacement. After Commit the handle is inert.
func (h *AIReplace) Commit() {
	if h == nil || !h.done.CompareAndSwap(false, true) {
		return
	}
	e := h.editor
	e.mu.Lock()
	if e.activeAI == h.id {
		e.activeAI = 0
	}
	e.mu.Unlock()
}

// Cancel reverts to the original text. Caret is restored to the end of the
// reverted range.
func (h *AIReplace) Cancel() {
	if h == nil || !h.done.CompareAndSwap(false, true) {
		return
	}
	e := h.editor
	e.mu.Lock()
	if e.activeAI != h.id {
		e.mu.Unlock()
		return
	}
	// Delete whatever's been streamed in so far, then re-insert the original.
	bi := h.from.Path[0]
	start := h.from.Offset
	removed := h.to.Offset - h.from.Offset
	e.doc.Blocks[bi] = doc.DeleteRange(e.doc.Blocks[bi], h.from.Offset, h.to.Offset)
	if h.original != "" {
		e.doc.Blocks[bi] = doc.InsertText(e.doc.Blocks[bi], h.from.Offset, h.original, 0)
	}
	// Transform the user's caret/selection across the revert (streamed text
	// removed, original restored) so it stays put logically.
	e.sel.Anchor = shiftPos(e.sel.Anchor, bi, start, removed, len(h.original))
	e.sel.Head = shiftPos(e.sel.Head, bi, start, removed, len(h.original))
	e.activeAI = 0
	e.preferredX = -1
	e.mu.Unlock()
	e.invalidate()
}

// shiftPos transforms a position across a splice in block bi: at byte offset
// start, `removed` bytes were deleted and `inserted` bytes inserted. Positions
// in other blocks, or at/before the splice point, are unchanged; positions
// after the removed region shift by the net length delta; positions inside the
// removed region collapse to the splice point. This keeps the user's caret at
// the same logical spot while AI text streams into the document.
func shiftPos(p doc.Position, bi, start, removed, inserted int) doc.Position {
	if len(p.Path) == 0 || p.Path[0] != bi {
		return p
	}
	switch {
	case p.Offset <= start:
		// at or before the splice point: unchanged
	case p.Offset >= start+removed:
		p.Offset += inserted - removed
	default:
		// inside the removed region: collapse to the splice point
		p.Offset = start
	}
	return p
}

var aiCounter atomic.Int64

func nextAIID() int64 { return aiCounter.Add(1) }
