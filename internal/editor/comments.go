package editor

// Comment is one user-authored comment anchored to a specific byte range
// within one block. The anchor mirrors Issue's storage: a range plus the
// literal text, so document edits can invalidate it.
type Comment struct {
	ID         int64
	BlockIndex int
	Offset     int
	Length     int
	AnchorText string
	Body       string
}

// SetComments replaces the active comment list. Caller mints IDs from the
// app/store layer (the SQLite row ID is reused).
func (e *RichEditor) SetComments(comments []Comment) {
	e.mu.Lock()
	e.comments = append(e.comments[:0], comments...)
	e.mu.Unlock()
	e.fireCommentsChanged()
	e.Refresh()
}

// AddComment appends a single comment without disturbing the existing list.
// Used by the right-click "Add comment..." flow once the store has assigned
// a row ID.
func (e *RichEditor) AddComment(c Comment) {
	e.mu.Lock()
	e.comments = append(e.comments, c)
	e.mu.Unlock()
	e.fireCommentsChanged()
	e.Refresh()
}

// ClearComments drops every comment. Equivalent to SetComments(nil).
func (e *RichEditor) ClearComments() { e.SetComments(nil) }

// Comments returns a copy of the current comment list.
func (e *RichEditor) Comments() []Comment {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Comment, len(e.comments))
	copy(out, e.comments)
	return out
}

// HasComments reports whether any comments are currently active.
func (e *RichEditor) HasComments() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.comments) > 0
}

// OnCommentsChanged registers a callback fired when the comment set changes
// (set, cleared, removed, or invalidated by an edit).
func (e *RichEditor) OnCommentsChanged(fn func()) { e.onCommentsChanged = fn }

func (e *RichEditor) fireCommentsChanged() {
	if e.onCommentsChanged != nil {
		e.onCommentsChanged()
	}
}

// RemoveComment drops the comment with the given id. No-op if not found.
func (e *RichEditor) RemoveComment(id int64) {
	e.mu.Lock()
	idx := e.indexOfComment(id)
	if idx < 0 {
		e.mu.Unlock()
		return
	}
	e.comments = append(e.comments[:idx], e.comments[idx+1:]...)
	e.mu.Unlock()
	e.fireCommentsChanged()
	e.Refresh()
}

func (e *RichEditor) indexOfComment(id int64) int {
	for i, c := range e.comments {
		if c.ID == id {
			return i
		}
	}
	return -1
}

// validateComments drops any comment whose anchor range no longer matches
// its original text. Caller must NOT hold the mutex.
func (e *RichEditor) validateComments() {
	e.mu.Lock()
	if len(e.comments) == 0 {
		e.mu.Unlock()
		return
	}
	changed := false
	kept := e.comments[:0]
	for _, c := range e.comments {
		if anchorStillMatches(e.doc.Blocks, c.BlockIndex, c.Offset, c.Length, c.AnchorText) {
			kept = append(kept, c)
			continue
		}
		changed = true
	}
	e.comments = kept
	e.mu.Unlock()
	if changed {
		e.fireCommentsChanged()
	}
}
