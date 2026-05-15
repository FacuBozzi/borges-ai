package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// VersionRow is one entry in the Versions sidebar. The app builds these from
// store.Version + the current document's content hash.
type VersionRow struct {
	ID        int64
	ShortHash string
	AgoText   string
	IsCurrent bool
}

// VersionsSidebar shows the snapshot list for the active document and a
// preview pane with a unified diff vs the current text. Restore swaps the
// in-memory document for the selected snapshot.
type VersionsSidebar struct {
	widget.BaseWidget

	header     *widget.Label
	refreshBtn *widget.Button
	restoreBtn *widget.Button
	list       *fyne.Container
	scroll     *container.Scroll
	emptyMsg   *widget.Label
	diffEntry  *widget.Entry

	rows         []VersionRow
	selectedID   int64
	hasSelection bool

	OnSelect  func(id int64)
	OnRestore func(id int64)
	OnRefresh func()
}

// NewVersionsSidebar builds the widget. Callers must wire OnRefresh, OnSelect,
// and OnRestore before showing it.
func NewVersionsSidebar() *VersionsSidebar {
	s := &VersionsSidebar{
		header:    widget.NewLabelWithStyle("Versions", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		list:      container.NewVBox(),
		emptyMsg:  widget.NewLabel("Save the document to create the first snapshot."),
		diffEntry: widget.NewMultiLineEntry(),
	}
	s.emptyMsg.Wrapping = fyne.TextWrapWord
	s.emptyMsg.Importance = widget.LowImportance

	s.diffEntry.TextStyle = fyne.TextStyle{Monospace: true}
	s.diffEntry.Wrapping = fyne.TextWrapOff
	s.diffEntry.Disable()

	s.refreshBtn = widget.NewButton("Refresh", func() {
		if s.OnRefresh != nil {
			s.OnRefresh()
		}
	})
	s.restoreBtn = widget.NewButton("Restore", func() {
		if s.hasSelection && s.OnRestore != nil {
			s.OnRestore(s.selectedID)
		}
	})
	s.restoreBtn.Importance = widget.HighImportance
	s.restoreBtn.Disable()

	s.scroll = container.NewVScroll(s.list)
	s.ExtendBaseWidget(s)
	s.refresh()
	return s
}

// CreateRenderer satisfies fyne.Widget by composing the sidebar layout.
func (s *VersionsSidebar) CreateRenderer() fyne.WidgetRenderer {
	top := container.NewVBox(s.header, s.refreshBtn)
	listArea := container.NewBorder(top, nil, nil, nil, s.scroll)
	diffArea := container.NewBorder(nil, s.restoreBtn, nil, nil, s.diffEntry)
	split := container.NewVSplit(listArea, diffArea)
	split.SetOffset(0.45)
	return widget.NewSimpleRenderer(split)
}

// SetRows replaces the displayed snapshots. Drops the current selection if
// the previously-selected id is no longer present.
func (s *VersionsSidebar) SetRows(rows []VersionRow) {
	s.rows = rows
	if s.hasSelection {
		found := false
		for _, r := range rows {
			if r.ID == s.selectedID {
				found = true
				break
			}
		}
		if !found {
			s.hasSelection = false
			s.selectedID = 0
			s.diffEntry.SetText("")
			s.restoreBtn.Disable()
		}
	}
	s.refresh()
}

// SetDiff sets the diff preview text. Called from the OnSelect callback after
// the app has computed the unified diff.
func (s *VersionsSidebar) SetDiff(text string) {
	s.diffEntry.SetText(text)
}

func (s *VersionsSidebar) refresh() {
	s.list.RemoveAll()
	if len(s.rows) == 0 {
		s.list.Add(s.emptyMsg)
		s.header.SetText("Versions")
	} else {
		s.header.SetText(fmt.Sprintf("Versions  (%d)", len(s.rows)))
		for _, r := range s.rows {
			s.list.Add(s.buildRow(r))
		}
	}
	s.list.Refresh()
}

func (s *VersionsSidebar) buildRow(r VersionRow) fyne.CanvasObject {
	label := r.AgoText + "  ·  " + r.ShortHash
	if r.IsCurrent {
		label += "  (current)"
	}
	btn := widget.NewButton(label, func() {
		s.selectedID = r.ID
		s.hasSelection = true
		s.restoreBtn.Enable()
		if s.OnSelect != nil {
			s.OnSelect(r.ID)
		}
		s.refresh()
	})
	btn.Alignment = widget.ButtonAlignLeading
	if r.ID == s.selectedID && s.hasSelection {
		btn.Importance = widget.HighImportance
	}
	return btn
}
