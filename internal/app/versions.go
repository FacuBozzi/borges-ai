package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/facubozzi/fyne-writer/internal/doc"
	"github.com/facubozzi/fyne-writer/internal/ui"
)

// versionsRetention caps how many snapshots we keep per document. Picked to
// match the README design notes — see "Watch out for" under M5 design notes.
const versionsRetention = 50

// snapshotCurrent persists the in-memory document as a new version row.
// No-ops if the doc has never been saved (no path → no anchor for snapshots).
// Errors are silently swallowed: a missed snapshot is not a user-facing failure.
func (a *App) snapshotCurrent() {
	if a.store == nil || a.currentPath == "" {
		return
	}
	d := a.editor.Document()
	if d == nil {
		return
	}
	data, err := json.Marshal(d)
	if err != nil {
		return
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	_, inserted, err := a.store.InsertVersion(a.currentPath, string(data), hash)
	if err != nil || !inserted {
		return
	}
	_ = a.store.GCVersions(a.currentPath, versionsRetention)
	a.refreshVersionsSidebar()
}

// refreshVersionsSidebar reloads the sidebar's row list from the store.
// Called after snapshots land and when the active document changes.
func (a *App) refreshVersionsSidebar() {
	if a.versionsSidebar == nil {
		return
	}
	if a.store == nil || a.currentPath == "" {
		a.versionsSidebar.SetRows(nil)
		return
	}
	versions, err := a.store.ListVersions(a.currentPath)
	if err != nil {
		a.versionsSidebar.SetRows(nil)
		return
	}
	currentHash := a.currentDocHash()
	now := time.Now()
	rows := make([]ui.VersionRow, 0, len(versions))
	for _, v := range versions {
		short := v.ContentHash
		if len(short) > 7 {
			short = short[:7]
		}
		rows = append(rows, ui.VersionRow{
			ID:        v.ID,
			ShortHash: short,
			AgoText:   formatAgo(now.Sub(v.CreatedAt)),
			IsCurrent: v.ContentHash == currentHash,
		})
	}
	a.versionsSidebar.SetRows(rows)
}

// previewVersion loads the snapshot for id and renders a unified diff against
// the current document into the sidebar's preview pane.
func (a *App) previewVersion(id int64) {
	if a.store == nil {
		return
	}
	v, err := a.store.LoadVersion(id)
	if err != nil {
		a.versionsSidebar.SetDiff("(failed to load snapshot)")
		return
	}
	snap, err := decodeSnapshot(v.SnapshotJSON)
	if err != nil {
		a.versionsSidebar.SetDiff("(snapshot decode failed)")
		return
	}
	snapMD := doc.WriteMarkdown(snap)
	currMD := doc.WriteMarkdown(a.editor.Document())
	if snapMD == currMD {
		a.versionsSidebar.SetDiff("(no differences from current document)")
		return
	}
	udiff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(snapMD),
		B:        difflib.SplitLines(currMD),
		FromFile: "snapshot",
		ToFile:   "current",
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(udiff)
	if err != nil || text == "" {
		a.versionsSidebar.SetDiff("(no differences from current document)")
		return
	}
	a.versionsSidebar.SetDiff(text)
}

// restoreVersion swaps the in-memory document for the chosen snapshot,
// recording one undo entry so cmd+Z reverts. The document is marked dirty
// because the user has to save again to persist the restore.
func (a *App) restoreVersion(id int64) {
	if a.store == nil {
		return
	}
	v, err := a.store.LoadVersion(id)
	if err != nil {
		return
	}
	snap, err := decodeSnapshot(v.SnapshotJSON)
	if err != nil {
		return
	}
	a.editor.ReplaceDocument(snap)
	a.dirty = true
	a.refreshTitle()
	a.refreshVersionsSidebar()
}

func decodeSnapshot(s string) (*doc.Document, error) {
	var d doc.Document
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (a *App) currentDocHash() string {
	d := a.editor.Document()
	if d == nil {
		return ""
	}
	data, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// formatAgo returns a short relative-time label like "3m ago" or "2d ago".
func formatAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	default:
		return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
	}
}
