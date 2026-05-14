package editor

import "github.com/facubozzi/fyne-writer/internal/doc"

const undoCapacity = 500

// undoEntry is a snapshot of the editor state taken before a mutation.
type undoEntry struct {
	doc *doc.Document
	sel doc.Selection
	// kind classifies the action so consecutive entries of the same kind
	// (typing runs) can coalesce into a single undo step.
	kind undoKind
}

type undoKind int

const (
	undoKindOther undoKind = iota
	undoKindTyping
	undoKindDelete
)

// undoStack is a fixed-capacity ring of snapshots used for undo.
type undoStack struct {
	entries []undoEntry
	cap     int
}

func newUndoStack(capacity int) *undoStack { return &undoStack{cap: capacity} }

func (s *undoStack) push(e undoEntry) {
	s.entries = append(s.entries, e)
	if len(s.entries) > s.cap {
		s.entries = s.entries[len(s.entries)-s.cap:]
	}
}

func (s *undoStack) pop() (undoEntry, bool) {
	if len(s.entries) == 0 {
		return undoEntry{}, false
	}
	last := s.entries[len(s.entries)-1]
	s.entries = s.entries[:len(s.entries)-1]
	return last, true
}

func (s *undoStack) peek() (undoEntry, bool) {
	if len(s.entries) == 0 {
		return undoEntry{}, false
	}
	return s.entries[len(s.entries)-1], true
}

func (s *undoStack) clear() { s.entries = nil }

// commitUndo captures the current state into the undo stack before a
// mutation. Consecutive same-kind operations (e.g. a run of typed runes)
// share a single undo entry so cmd+Z reverts a typed word rather than one
// character at a time.
//
// Caller must hold e.mu.
func (e *RichEditor) commitUndo(kind undoKind) {
	if e.undo == nil {
		e.undo = newUndoStack(undoCapacity)
		e.redo = newUndoStack(undoCapacity)
	}
	if last, ok := e.undo.peek(); ok && last.kind == kind && kind != undoKindOther {
		// Coalesce — don't push a new snapshot for the next char in a run.
		return
	}
	e.undo.push(undoEntry{
		doc:  e.doc.Clone(),
		sel:  e.sel,
		kind: kind,
	})
	// Any new edit invalidates the redo trail.
	e.redo.clear()
}

// breakUndoRun ends any active typing/delete coalescing so the next mutation
// starts a fresh undo entry. Called on caret movement, save, and other
// "scene changes".
func (e *RichEditor) breakUndoRun() {
	if e.undo == nil {
		return
	}
	// Replace the kind of the top entry so the next commitUndo can't coalesce.
	if last, ok := e.undo.peek(); ok {
		last.kind = undoKindOther
		e.undo.entries[len(e.undo.entries)-1] = last
	}
}

// Undo restores the previous snapshot. Saves the current state to the redo
// stack so cmd+shift+Z can reapply.
func (e *RichEditor) Undo() {
	e.mu.Lock()
	if e.undo == nil {
		e.mu.Unlock()
		return
	}
	entry, ok := e.undo.pop()
	if !ok {
		e.mu.Unlock()
		return
	}
	e.redo.push(undoEntry{doc: e.doc.Clone(), sel: e.sel, kind: undoKindOther})
	e.doc = entry.doc
	e.sel = entry.sel
	e.preferredX = -1
	e.pendingMarksSet = false
	e.mu.Unlock()
	e.invalidate()
}

// Redo reapplies the most recent undone state.
func (e *RichEditor) Redo() {
	e.mu.Lock()
	if e.redo == nil {
		e.mu.Unlock()
		return
	}
	entry, ok := e.redo.pop()
	if !ok {
		e.mu.Unlock()
		return
	}
	e.undo.push(undoEntry{doc: e.doc.Clone(), sel: e.sel, kind: undoKindOther})
	e.doc = entry.doc
	e.sel = entry.sel
	e.preferredX = -1
	e.pendingMarksSet = false
	e.mu.Unlock()
	e.invalidate()
}
