package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// IssueRow is one item in the issues sidebar. The app layer translates from
// editor.Issue → IssueRow so this widget stays decoupled from the editor.
type IssueRow struct {
	ID          int64
	Kind        string
	Severity    string
	Message     string
	AnchorText  string
	Replacement string
}

// IssuesSidebar is a stateful Fyne widget showing the active issues. Built
// once at startup; the app calls SetIssues / SetRunning to mutate state.
type IssuesSidebar struct {
	widget.BaseWidget

	header    *widget.Label
	checkBtn  *widget.Button
	clearBtn  *widget.Button
	list      *fyne.Container
	scroll    *container.Scroll
	emptyMsg  *widget.Label
	progress  *widget.ProgressBarInfinite

	issues  []IssueRow
	running bool

	OnCheck   func()
	OnClear   func()
	OnAccept  func(id int64, replacement string)
	OnReject  func(id int64)
}

// NewIssuesSidebar builds the sidebar widget. Callers must wire OnCheck /
// OnAccept / OnReject before showing the widget.
func NewIssuesSidebar() *IssuesSidebar {
	s := &IssuesSidebar{
		header:   widget.NewLabelWithStyle("Document checks", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		list:     container.NewVBox(),
		emptyMsg: widget.NewLabel("Run a check to see grammar, clarity, and style hints here."),
		progress: widget.NewProgressBarInfinite(),
	}
	s.emptyMsg.Wrapping = fyne.TextWrapWord
	s.emptyMsg.Importance = widget.LowImportance
	s.progress.Hide()

	s.checkBtn = widget.NewButton("Check", func() {
		if s.OnCheck != nil {
			s.OnCheck()
		}
	})
	s.clearBtn = widget.NewButton("Clear", func() {
		if s.OnClear != nil {
			s.OnClear()
		}
	})
	s.clearBtn.Disable()

	s.scroll = container.NewVScroll(s.list)
	s.ExtendBaseWidget(s)
	s.refresh()
	return s
}

// CreateRenderer satisfies fyne.Widget by composing the sidebar layout.
func (s *IssuesSidebar) CreateRenderer() fyne.WidgetRenderer {
	buttons := container.NewGridWithColumns(2, s.checkBtn, s.clearBtn)
	top := container.NewVBox(s.header, buttons, s.progress)
	body := container.NewBorder(top, nil, nil, nil, s.scroll)
	return widget.NewSimpleRenderer(body)
}

// SetIssues replaces the displayed issues.
func (s *IssuesSidebar) SetIssues(rows []IssueRow) {
	s.issues = rows
	s.refresh()
}

// SetRunning toggles the progress bar + Check button state.
func (s *IssuesSidebar) SetRunning(running bool) {
	s.running = running
	if running {
		s.progress.Show()
		s.checkBtn.Disable()
	} else {
		s.progress.Hide()
		s.checkBtn.Enable()
	}
	s.refresh()
}

func (s *IssuesSidebar) refresh() {
	s.list.RemoveAll()
	if len(s.issues) == 0 {
		s.list.Add(s.emptyMsg)
		s.clearBtn.Disable()
	} else {
		s.clearBtn.Enable()
		s.header.SetText(fmt.Sprintf("Document checks  (%d)", len(s.issues)))
		for _, r := range s.issues {
			s.list.Add(s.buildRow(r))
		}
	}
	if len(s.issues) == 0 {
		s.header.SetText("Document checks")
	}
	s.list.Refresh()
}

func (s *IssuesSidebar) buildRow(r IssueRow) fyne.CanvasObject {
	badge := widget.NewLabel(fmt.Sprintf("[%s · %s]", emptyIfBlank(r.Kind, "issue"), emptyIfBlank(r.Severity, "med")))
	badge.Importance = severityImportance(r.Severity)

	anchor := widget.NewLabel("“" + r.AnchorText + "”")
	anchor.Importance = widget.LowImportance
	anchor.Wrapping = fyne.TextWrapWord

	msg := widget.NewLabel(r.Message)
	msg.Wrapping = fyne.TextWrapWord

	var actions fyne.CanvasObject
	if r.Replacement != "" {
		suggestion := widget.NewLabel("→ " + r.Replacement)
		suggestion.Wrapping = fyne.TextWrapWord
		acceptBtn := widget.NewButton("Accept", func() {
			if s.OnAccept != nil {
				s.OnAccept(r.ID, r.Replacement)
			}
		})
		acceptBtn.Importance = widget.HighImportance
		rejectBtn := widget.NewButton("Dismiss", func() {
			if s.OnReject != nil {
				s.OnReject(r.ID)
			}
		})
		actions = container.NewVBox(suggestion, container.NewGridWithColumns(2, acceptBtn, rejectBtn))
	} else {
		rejectBtn := widget.NewButton("Dismiss", func() {
			if s.OnReject != nil {
				s.OnReject(r.ID)
			}
		})
		actions = container.NewBorder(nil, nil, nil, rejectBtn, widget.NewLabel(""))
	}

	body := container.NewVBox(badge, anchor, msg, actions, widget.NewSeparator())
	return body
}

func emptyIfBlank(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func severityImportance(sev string) widget.Importance {
	switch sev {
	case "high":
		return widget.DangerImportance
	case "med":
		return widget.WarningImportance
	default:
		return widget.LowImportance
	}
}
