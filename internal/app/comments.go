package app

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/facubozzi/fyne-writer/internal/doc"
	"github.com/facubozzi/fyne-writer/internal/editor"
	"github.com/facubozzi/fyne-writer/internal/store"
	"github.com/facubozzi/fyne-writer/internal/ui"
)

// loadCommentsForDoc fetches comments for the current doc, resolves each to
// a live byte range, pushes the resolved set into the editor, and refreshes
// the sidebar. No-op when there is no active document on disk.
func (a *App) loadCommentsForDoc() {
	if a.editor != nil {
		a.editor.ClearComments()
	}
	a.refreshCommentsSidebar()
	if a.store == nil || a.currentPath == "" {
		return
	}
	rows, err := a.store.ListComments(a.currentPath, true)
	if err != nil {
		return
	}
	d := a.editor.Document()
	if d == nil {
		return
	}
	resolved := make([]editor.Comment, 0, len(rows))
	for _, row := range rows {
		if row.Resolved {
			continue
		}
		bi, off, ok := resolveCommentAnchor(d, row)
		if !ok {
			continue
		}
		if bi != row.BlockIndex || off != row.RangeStartHint {
			_ = a.store.UpdateCommentAnchorHint(row.ID, bi, off, off+len(row.AnchorText))
		}
		resolved = append(resolved, editor.Comment{
			ID:         row.ID,
			BlockIndex: bi,
			Offset:     off,
			Length:     len(row.AnchorText),
			AnchorText: row.AnchorText,
			Body:       row.Body,
		})
	}
	a.editor.SetComments(resolved)
	a.refreshCommentsSidebar()
}

// resolveCommentAnchor first checks whether the stored hint still matches,
// then falls back to a substring search across all blocks. Returns ok=false
// when the anchor text can't be located.
func resolveCommentAnchor(d *doc.Document, row store.Comment) (blockIdx, offset int, ok bool) {
	if row.BlockIndex >= 0 && row.BlockIndex < len(d.Blocks) {
		plain := d.Blocks[row.BlockIndex].PlainText()
		end := row.RangeStartHint + len(row.AnchorText)
		if row.RangeStartHint >= 0 && end <= len(plain) && plain[row.RangeStartHint:end] == row.AnchorText {
			return row.BlockIndex, row.RangeStartHint, true
		}
	}
	if row.AnchorText == "" {
		return 0, 0, false
	}
	return findUniqueAnchor(d, row.AnchorText)
}

// addCommentAtSelection persists a new comment for the editor's current
// selection and pushes it into the editor's decoration list. Returns ok=true
// only when the selection lay inside one block (multi-block selections are
// rejected for v1).
func (a *App) addCommentAtSelection(body string) bool {
	if a.editor == nil || a.store == nil {
		return false
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return false
	}
	bi, start, end, ok := a.editor.SelectionSingleBlockRange()
	if !ok {
		return false
	}
	d := a.editor.Document()
	if d == nil || bi < 0 || bi >= len(d.Blocks) {
		return false
	}
	plain := d.Blocks[bi].PlainText()
	if start < 0 || end > len(plain) || start >= end {
		return false
	}
	anchor := plain[start:end]
	if a.currentPath == "" {
		return false
	}
	row := store.Comment{
		DocPath:        a.currentPath,
		AnchorText:     anchor,
		BlockIndex:     bi,
		RangeStartHint: start,
		RangeEndHint:   end,
		Body:           body,
	}
	id, err := a.store.InsertComment(row)
	if err != nil {
		return false
	}
	a.editor.AddComment(editor.Comment{
		ID:         id,
		BlockIndex: bi,
		Offset:     start,
		Length:     end - start,
		AnchorText: anchor,
		Body:       body,
	})
	a.refreshCommentsSidebar()
	return true
}

// resolveComment marks the comment resolved in the store and removes it from
// the editor's decoration list.
func (a *App) resolveComment(id int64) {
	if a.store == nil {
		return
	}
	if err := a.store.SetCommentResolved(id, true); err != nil {
		return
	}
	a.editor.RemoveComment(id)
	a.refreshCommentsSidebar()
}

// deleteCommentByID removes the comment from the store and the editor.
func (a *App) deleteCommentByID(id int64) {
	if a.store == nil {
		return
	}
	if err := a.store.DeleteComment(id); err != nil {
		return
	}
	a.editor.RemoveComment(id)
	a.refreshCommentsSidebar()
}

// refreshCommentsSidebar reloads the active set of comments into the
// sidebar widget. Comments dropped from the editor by edits remain in the
// store; the sidebar shows them as orphaned (no anchor to jump to).
func (a *App) refreshCommentsSidebar() {
	if a.commentsSidebar == nil {
		return
	}
	if a.store == nil || a.currentPath == "" {
		a.commentsSidebar.SetRows(nil)
		return
	}
	includeResolved := a.commentsSidebar.ShowResolved()
	rows, err := a.store.ListComments(a.currentPath, includeResolved)
	if err != nil {
		a.commentsSidebar.SetRows(nil)
		return
	}
	live := map[int64]bool{}
	for _, c := range a.editor.Comments() {
		live[c.ID] = true
	}
	now := time.Now()
	out := make([]ui.CommentRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ui.CommentRow{
			ID:         r.ID,
			Body:       r.Body,
			AnchorText: r.AnchorText,
			AgoText:    formatAgo(now.Sub(r.CreatedAt)),
			Orphaned:   !r.Resolved && !live[r.ID],
			Resolved:   r.Resolved,
		})
	}
	a.commentsSidebar.SetRows(out)
}

// jumpToComment selects the editor range backing the comment, so the user
// can see exactly what was annotated. Skip orphaned comments (no live anchor).
func (a *App) jumpToComment(id int64) {
	for _, c := range a.editor.Comments() {
		if c.ID != id {
			continue
		}
		a.editor.SelectRange(c.BlockIndex, c.Offset, c.Offset+c.Length)
		return
	}
}

// openAddCommentDialog shows a small modal with a multi-line textarea so the
// user can type the comment body. On Save the comment is persisted +
// decorated. Requires currentPath because comments are keyed per file.
func (a *App) openAddCommentDialog() {
	if a.currentPath == "" {
		dialog.ShowInformation("Comment", "Save the document first — comments are stored per file.", a.window)
		return
	}
	if _, _, _, ok := a.editor.SelectionSingleBlockRange(); !ok {
		dialog.ShowInformation("Comment", "Select text within a single paragraph first.", a.window)
		return
	}
	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder("Comment…")
	entry.Wrapping = fyne.TextWrapWord
	content := entry
	dlg := dialog.NewCustomConfirm("Add comment", "Save", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		a.addCommentAtSelection(entry.Text)
	}, a.window)
	dlg.Resize(fyne.NewSize(420, 200))
	dlg.Show()
	a.window.Canvas().Focus(entry)
}
