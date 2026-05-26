package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// CommentRow is one entry in the Comments sidebar. The app translates
// store.Comment + the editor's live anchor list into these.
type CommentRow struct {
	ID         int64
	Body       string
	AnchorText string
	AgoText    string
	Orphaned   bool // true when the editor couldn't anchor this comment
	Resolved   bool // true when the comment has been resolved
}

// CommentsSidebar lists open comments for the active document with per-row
// Jump / Resolve / Delete actions.
type CommentsSidebar struct {
	widget.BaseWidget

	header       *widget.Label
	showResolved *widget.Check
	list         *fyne.Container
	scroll       *container.Scroll
	emptyMsg     *widget.Label

	rows []CommentRow

	OnJump    func(id int64)
	OnResolve func(id int64)
	OnDelete  func(id int64)
	// OnFilterChanged fires when the "show resolved" toggle flips; the app
	// re-lists comments (reading ShowResolved) in response.
	OnFilterChanged func()
}

// NewCommentsSidebar builds the widget. Callers wire OnJump/OnResolve/OnDelete
// (and OnFilterChanged) before showing it.
func NewCommentsSidebar() *CommentsSidebar {
	s := &CommentsSidebar{
		header:   widget.NewLabelWithStyle("Comments", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		list:     container.NewVBox(),
		emptyMsg: widget.NewLabel("Select text and right-click → Add comment… to start."),
	}
	s.emptyMsg.Wrapping = fyne.TextWrapWord
	s.emptyMsg.Importance = widget.LowImportance

	s.showResolved = widget.NewCheck("Show resolved", func(bool) {
		if s.OnFilterChanged != nil {
			s.OnFilterChanged()
		}
	})

	s.scroll = container.NewVScroll(s.list)
	s.ExtendBaseWidget(s)
	s.refresh()
	return s
}

// ShowResolved reports whether resolved comments should be listed.
func (s *CommentsSidebar) ShowResolved() bool { return s.showResolved.Checked }

// CreateRenderer composes the sidebar layout.
func (s *CommentsSidebar) CreateRenderer() fyne.WidgetRenderer {
	top := container.NewVBox(s.header, s.showResolved)
	body := container.NewBorder(top, nil, nil, nil, s.scroll)
	return widget.NewSimpleRenderer(body)
}

// SetRows replaces the displayed comments.
func (s *CommentsSidebar) SetRows(rows []CommentRow) {
	s.rows = rows
	s.refresh()
}

func (s *CommentsSidebar) refresh() {
	s.list.RemoveAll()
	if len(s.rows) == 0 {
		s.list.Add(s.emptyMsg)
		s.header.SetText("Comments")
	} else {
		s.header.SetText(fmt.Sprintf("Comments  (%d)", len(s.rows)))
		for _, r := range s.rows {
			s.list.Add(s.buildRow(r))
		}
	}
	s.list.Refresh()
}

func (s *CommentsSidebar) buildRow(r CommentRow) fyne.CanvasObject {
	meta := widget.NewLabel(r.AgoText)
	meta.Importance = widget.LowImportance

	anchorText := "“" + truncate(r.AnchorText, 40) + "”"
	switch {
	case r.Resolved:
		anchorText += "  (resolved)"
	case r.Orphaned:
		anchorText += "  (orphaned)"
	}
	anchor := widget.NewLabel(anchorText)
	anchor.Importance = widget.LowImportance
	anchor.Wrapping = fyne.TextWrapWord

	body := widget.NewLabel(r.Body)
	body.Wrapping = fyne.TextWrapWord

	jumpBtn := widget.NewButton("Jump", func() {
		if s.OnJump != nil {
			s.OnJump(r.ID)
		}
	})
	if r.Orphaned || r.Resolved {
		jumpBtn.Disable()
	}
	deleteBtn := widget.NewButton("Delete", func() {
		if s.OnDelete != nil {
			s.OnDelete(r.ID)
		}
	})

	// Resolved comments have no Resolve action; the rest keep all three.
	var actions fyne.CanvasObject
	if r.Resolved {
		actions = container.NewGridWithColumns(2, jumpBtn, deleteBtn)
	} else {
		resolveBtn := widget.NewButton("Resolve", func() {
			if s.OnResolve != nil {
				s.OnResolve(r.ID)
			}
		})
		resolveBtn.Importance = widget.HighImportance
		actions = container.NewGridWithColumns(3, jumpBtn, resolveBtn, deleteBtn)
	}
	return container.NewVBox(meta, anchor, body, actions, widget.NewSeparator())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return s[:n-1] + "…"
}
