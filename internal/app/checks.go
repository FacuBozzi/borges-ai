package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/facubozzi/fyne-writer/internal/ai"
	"github.com/facubozzi/fyne-writer/internal/doc"
	"github.com/facubozzi/fyne-writer/internal/editor"
	"github.com/facubozzi/fyne-writer/internal/ui"
)

// runDocumentCheck triggers the AI document check. Runs in a goroutine and
// pushes the resolved issue list to the editor + sidebar on completion.
func (a *App) runDocumentCheck() {
	if a.checksRunning {
		return
	}
	a.checksRunning = true
	a.sidebar.SetRunning(true)

	d := a.editor.Document()
	plain := documentPlainText(d)
	if strings.TrimSpace(plain) == "" {
		a.checksRunning = false
		a.sidebar.SetRunning(false)
		dialog.ShowInformation("Check", "Document is empty.", a.window)
		return
	}
	context_ := d.Meta.Instructions
	provider := a.registry.Active()
	model := a.modelFor(provider.Name())

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		suggestions, err := ai.CheckDocument(ctx, provider, model, plain, context_)
		fyne.Do(func() {
			a.checksRunning = false
			a.sidebar.SetRunning(false)
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			issues := a.resolveSuggestions(suggestions)
			a.editor.SetIssues(issues)
			if len(issues) == 0 {
				dialog.ShowInformation("Check",
					"No issues found.",
					a.window)
			}
		})
	}()
}

// resolveSuggestions walks the document's blocks and tries to locate each
// suggestion's anchor_text. Issues whose anchor doesn't appear uniquely in
// some block are skipped — the model can't reliably point at multiple
// occurrences and guessing risks editing the wrong place.
func (a *App) resolveSuggestions(suggestions []ai.Suggestion) []editor.Issue {
	d := a.editor.Document()
	out := make([]editor.Issue, 0, len(suggestions))
	for _, s := range suggestions {
		anchor := strings.TrimSpace(s.AnchorText)
		if anchor == "" {
			continue
		}
		bi, offset, ok := findUniqueAnchor(d, anchor)
		if !ok {
			continue
		}
		out = append(out, editor.Issue{
			ID:          editor.NextIssueID(),
			BlockIndex:  bi,
			Offset:      offset,
			Length:      len(anchor),
			AnchorText:  anchor,
			Kind:        s.Type,
			Severity:    s.Severity,
			Message:     s.Message,
			Replacement: s.Replacement,
		})
	}
	return out
}

// findUniqueAnchor returns the (block index, byte offset) of anchor inside
// the document, but only if it appears in exactly one block at exactly one
// position. Returns ok=false otherwise.
func findUniqueAnchor(d *doc.Document, anchor string) (int, int, bool) {
	foundBlock := -1
	foundOffset := -1
	matches := 0
	for bi, b := range d.Blocks {
		plain := b.PlainText()
		from := 0
		for {
			i := strings.Index(plain[from:], anchor)
			if i < 0 {
				break
			}
			matches++
			if matches > 1 {
				return 0, 0, false
			}
			foundBlock = bi
			foundOffset = from + i
			from = from + i + 1
		}
	}
	if matches == 1 {
		return foundBlock, foundOffset, true
	}
	return 0, 0, false
}

// updateSidebarFromEditor reads the editor's current issues and pushes them
// to the sidebar. Called both directly after a check and from the editor's
// OnIssuesChanged callback so the panel stays in sync with invalidation.
func (a *App) updateSidebarFromEditor() {
	issues := a.editor.Issues()
	rows := make([]ui.IssueRow, 0, len(issues))
	for _, i := range issues {
		rows = append(rows, ui.IssueRow{
			ID:          i.ID,
			Kind:        i.Kind,
			Severity:    i.Severity,
			Message:     i.Message,
			AnchorText:  i.AnchorText,
			Replacement: i.Replacement,
		})
	}
	a.sidebar.SetIssues(rows)
}

// guard against unused imports when this file is the only consumer of fmt
var _ = fmt.Sprintf
