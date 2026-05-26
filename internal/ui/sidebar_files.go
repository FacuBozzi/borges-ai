package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// FilesSidebar is a lightweight file browser tab: a dirList rooted at a
// directory, where clicking a markdown file fires OnOpen. Unlike the file
// dialog, a single click opens immediately (no confirm button).
type FilesSidebar struct {
	widget.BaseWidget

	header    *widget.Label
	pathLabel *widget.Label
	dl        *dirList

	OnOpen func(path string)
}

// NewFilesSidebar builds the widget. Callers wire OnOpen and call SetRoot to
// point it at a directory before (or after) showing it.
func NewFilesSidebar() *FilesSidebar {
	s := &FilesSidebar{
		header:    widget.NewLabelWithStyle("Files", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		pathLabel: newPathLabel(),
	}
	s.dl = newDirList([]string{".md", ".markdown", ".txt"})
	s.dl.onDir = func(dir string) { s.pathLabel.SetText(dir) }
	s.dl.onErr = func(err error) { s.pathLabel.SetText("⚠ " + err.Error()) }
	s.dl.onFile = func(p string) {
		s.dl.list.UnselectAll() // allow re-clicking the same file later
		if s.OnOpen != nil {
			s.OnOpen(p)
		}
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetRoot points the browser at dir and refreshes the listing.
func (s *FilesSidebar) SetRoot(dir string) { s.dl.navigate(dir) }

// CreateRenderer satisfies fyne.Widget.
func (s *FilesSidebar) CreateRenderer() fyne.WidgetRenderer {
	top := container.NewVBox(s.header, s.pathLabel)
	body := container.NewBorder(top, nil, nil, nil, s.dl.list)
	return widget.NewSimpleRenderer(body)
}
