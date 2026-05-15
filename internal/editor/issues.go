package editor

import (
	"sync/atomic"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// Issue is one resolved grammar/clarity/style hint anchored to a specific
// byte range within one block. The anchor is stored both as a range and as
// the literal text so we can detect when document edits invalidate it.
type Issue struct {
	ID          int64
	BlockIndex  int
	Offset      int
	Length      int
	AnchorText  string
	Kind        string
	Severity    string
	Message     string
	Replacement string
}

var issueCounter atomic.Int64

// NextIssueID returns a fresh issue ID. Exposed so the orchestration layer
// can mint IDs while building the issue set.
func NextIssueID() int64 { return issueCounter.Add(1) }

// SetIssues replaces the active issue list. Caller mints IDs via NextIssueID.
func (e *RichEditor) SetIssues(issues []Issue) {
	e.mu.Lock()
	e.issues = append(e.issues[:0], issues...)
	e.mu.Unlock()
	e.fireIssuesChanged()
	e.Refresh()
}

// ClearIssues drops every issue. Equivalent to SetIssues(nil).
func (e *RichEditor) ClearIssues() {
	e.SetIssues(nil)
}

// Issues returns a copy of the current issue list.
func (e *RichEditor) Issues() []Issue {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Issue, len(e.issues))
	copy(out, e.issues)
	return out
}

// HasIssues reports whether any issues are currently active.
func (e *RichEditor) HasIssues() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.issues) > 0
}

// OnIssuesChanged registers a callback fired when the issue set changes
// (set, cleared, dismissed, or invalidated by an edit).
func (e *RichEditor) OnIssuesChanged(fn func()) { e.onIssuesChanged = fn }

func (e *RichEditor) fireIssuesChanged() {
	if e.onIssuesChanged != nil {
		e.onIssuesChanged()
	}
}

// ApplyIssueFix replaces the issue's anchor range with replacement and
// removes the issue. Single undo step.
func (e *RichEditor) ApplyIssueFix(id int64, replacement string) {
	e.mu.Lock()
	idx := e.indexOfIssue(id)
	if idx < 0 {
		e.mu.Unlock()
		return
	}
	iss := e.issues[idx]
	if iss.BlockIndex < 0 || iss.BlockIndex >= len(e.doc.Blocks) {
		e.mu.Unlock()
		return
	}
	e.commitUndo(undoKindOther)
	b := e.doc.Blocks[iss.BlockIndex]
	b = doc.DeleteRange(b, iss.Offset, iss.Offset+iss.Length)
	if replacement != "" {
		b = doc.InsertText(b, iss.Offset, replacement, 0)
	}
	e.doc.Blocks[iss.BlockIndex] = b
	e.issues = append(e.issues[:idx], e.issues[idx+1:]...)
	caret := doc.Position{Path: []int{iss.BlockIndex}, Inline: 0, Offset: iss.Offset + len(replacement)}
	e.setCaret(caret)
	e.preferredX = -1
	e.mu.Unlock()
	e.fireIssuesChanged()
	e.invalidate()
}

// DismissIssue removes an issue without changing the document text.
func (e *RichEditor) DismissIssue(id int64) {
	e.mu.Lock()
	idx := e.indexOfIssue(id)
	if idx < 0 {
		e.mu.Unlock()
		return
	}
	e.issues = append(e.issues[:idx], e.issues[idx+1:]...)
	e.mu.Unlock()
	e.fireIssuesChanged()
	e.Refresh()
}

func (e *RichEditor) indexOfIssue(id int64) int {
	for i, iss := range e.issues {
		if iss.ID == id {
			return i
		}
	}
	return -1
}

// validateIssues drops any issue whose anchor range no longer holds its
// original text (because the document was edited). Caller must NOT hold the
// mutex — this method locks internally and fires the change callback if any
// issues were removed.
func (e *RichEditor) validateIssues() {
	e.mu.Lock()
	if len(e.issues) == 0 {
		e.mu.Unlock()
		return
	}
	changed := false
	kept := e.issues[:0]
	for _, iss := range e.issues {
		if anchorStillMatches(e.doc.Blocks, iss.BlockIndex, iss.Offset, iss.Length, iss.AnchorText) {
			kept = append(kept, iss)
			continue
		}
		changed = true
	}
	e.issues = kept
	e.mu.Unlock()
	if changed {
		e.fireIssuesChanged()
	}
}

// anchorStillMatches reports whether the byte range (offset..offset+length)
// in blocks[blockIdx] still contains anchorText verbatim. Shared by issue
// and comment validation paths.
func anchorStillMatches(blocks []doc.Block, blockIdx, offset, length int, anchorText string) bool {
	if blockIdx < 0 || blockIdx >= len(blocks) {
		return false
	}
	plain := blocks[blockIdx].PlainText()
	end := offset + length
	if offset < 0 || end > len(plain) {
		return false
	}
	return plain[offset:end] == anchorText
}
