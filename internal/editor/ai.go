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
	// Insert chunk at h.to (no marks for AI output for now).
	bi := h.to.Path[0]
	e.doc.Blocks[bi] = doc.InsertText(e.doc.Blocks[bi], h.to.Offset, chunk, 0)
	h.to.Offset += len(chunk)
	e.setCaret(h.to)
	e.preferredX = -1
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
	e.doc.Blocks[bi] = doc.DeleteRange(e.doc.Blocks[bi], h.from.Offset, h.to.Offset)
	if h.original != "" {
		e.doc.Blocks[bi] = doc.InsertText(e.doc.Blocks[bi], h.from.Offset, h.original, 0)
	}
	e.setCaret(doc.Position{Path: []int{bi}, Inline: 0, Offset: h.from.Offset + len(h.original)})
	e.activeAI = 0
	e.preferredX = -1
	e.mu.Unlock()
	e.invalidate()
}

var aiCounter atomic.Int64

func nextAIID() int64 { return aiCounter.Add(1) }
