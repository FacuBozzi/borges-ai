package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ShortcutItem is one keyboard-shortcut row: a key combo and what it does.
type ShortcutItem struct {
	Keys   string
	Action string
}

// ShortcutGroup is a titled cluster of shortcuts (e.g. "File", "Formatting").
type ShortcutGroup struct {
	Title string
	Items []ShortcutItem
}

// ShowShortcuts opens a modal cheatsheet of the given groups with a filter box.
// Esc (or the focused filter) closes it.
func ShowShortcuts(win fyne.Window, groups []ShortcutGroup) {
	var closePopup func()

	list := container.NewVBox()
	render := func(q string) {
		q = strings.ToLower(strings.TrimSpace(q))
		list.RemoveAll()
		for _, g := range groups {
			var rows []fyne.CanvasObject
			for _, it := range g.Items {
				if q != "" &&
					!strings.Contains(strings.ToLower(it.Action), q) &&
					!strings.Contains(strings.ToLower(it.Keys), q) {
					continue
				}
				keys := widget.NewLabelWithStyle(it.Keys, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
				rows = append(rows, container.NewBorder(nil, nil, keys, nil, widget.NewLabel(it.Action)))
			}
			if len(rows) == 0 {
				continue
			}
			list.Add(widget.NewLabelWithStyle(g.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
			for _, r := range rows {
				list.Add(r)
			}
		}
		list.Refresh()
	}

	filter := newEscEntry(func() { closePopup() })
	filter.SetPlaceHolder("Filter shortcuts…")
	filter.OnChanged = render
	render("")

	body := container.NewBorder(filter, nil, nil, nil, container.NewVScroll(list))
	_, closePopup = showModal(win, fyne.NewSize(460, 520), body, filter)
}
