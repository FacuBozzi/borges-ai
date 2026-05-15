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
}

// CommentsSidebar lists open comments for the active document with per-row
// Jump / Resolve / Delete actions.
type CommentsSidebar struct {
	widget.BaseWidget

	header   *widget.Label
	list     *fyne.Container
	scroll   *container.Scroll
	emptyMsg *widget.Label

	rows []CommentRow

	OnJump    func(id int64)
	OnResolve func(id int64)
	OnDelete  func(id int64)
}

// NewCommentsSidebar builds the widget. Callers wire OnJump/OnResolve/OnDelete
// before showing it.
func NewCommentsSidebar() *CommentsSidebar {
	s := &CommentsSidebar{
		header:   widget.NewLabelWithStyle("Comments", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		list:     container.NewVBox(),
		emptyMsg: widget.NewLabel("Select text and right-click → Add comment… to start."),
	}
	s.emptyMsg.Wrapping = fyne.TextWrapWord
	s.emptyMsg.Importance = widget.LowImportance

	s.scroll = container.NewVScroll(s.list)
	s.ExtendBaseWidget(s)
	s.refresh()
	return s
}

// CreateRenderer composes the sidebar layout.
func (s *CommentsSidebar) CreateRenderer() fyne.WidgetRenderer {
	body := container.NewBorder(s.header, nil, nil, nil, s.scroll)
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

	anchor := widget.NewLabel("“" + truncate(r.AnchorText, 40) + "”")
	anchor.Importance = widget.LowImportance
	anchor.Wrapping = fyne.TextWrapWord

	body := widget.NewLabel(r.Body)
	body.Wrapping = fyne.TextWrapWord

	jumpBtn := widget.NewButton("Jump", func() {
		if s.OnJump != nil {
			s.OnJump(r.ID)
		}
	})
	if r.Orphaned {
		jumpBtn.Disable()
	}
	resolveBtn := widget.NewButton("Resolve", func() {
		if s.OnResolve != nil {
			s.OnResolve(r.ID)
		}
	})
	resolveBtn.Importance = widget.HighImportance
	deleteBtn := widget.NewButton("Delete", func() {
		if s.OnDelete != nil {
			s.OnDelete(r.ID)
		}
	})

	actions := container.NewGridWithColumns(3, jumpBtn, resolveBtn, deleteBtn)

	if r.Orphaned {
		anchor.SetText("“" + truncate(r.AnchorText, 40) + "”  (orphaned)")
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
