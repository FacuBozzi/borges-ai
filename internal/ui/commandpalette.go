package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// PaletteCommand is one row in the command palette. Title is the primary
// label, Subtitle is dim hint text. Run is invoked when the user picks it.
type PaletteCommand struct {
	Title    string
	Subtitle string
	Disabled bool // greyed out, can't be picked
	Run      func()
}

// ShowCommandPalette opens the cmd+K overlay. The palette is a centered
// modal with a search entry and a filterable list. Enter runs the focused
// command; Esc cancels.
func ShowCommandPalette(win fyne.Window, commands []PaletteCommand) {
	if len(commands) == 0 {
		return
	}
	canvas := win.Canvas()

	entry := widget.NewEntry()
	entry.SetPlaceHolder("Type a command...")

	state := &paletteState{commands: commands}
	state.filtered = filterCommands(commands, "")

	list := widget.NewList(
		func() int { return len(state.filtered) },
		func() fyne.CanvasObject {
			title := widget.NewLabel("title")
			title.TextStyle = fyne.TextStyle{Bold: true}
			sub := widget.NewLabel("sub")
			sub.Importance = widget.LowImportance
			return container.NewVBox(title, sub)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			cmd := state.filtered[i]
			box.Objects[0].(*widget.Label).SetText(cmd.Title)
			box.Objects[1].(*widget.Label).SetText(cmd.Subtitle)
		},
	)

	var pop *widget.PopUp
	closePopup := func() {
		if pop != nil {
			pop.Hide()
		}
	}

	pickIndex := func(i int) {
		if i < 0 || i >= len(state.filtered) {
			return
		}
		cmd := state.filtered[i]
		if cmd.Disabled || cmd.Run == nil {
			return
		}
		closePopup()
		cmd.Run()
	}

	list.OnSelected = func(i widget.ListItemID) { pickIndex(i) }

	entry.OnChanged = func(q string) {
		state.filtered = filterCommands(state.commands, q)
		list.Refresh()
		if len(state.filtered) > 0 {
			list.Select(0)
		}
	}
	entry.OnSubmitted = func(string) {
		// "Selected()" isn't directly available; we always run the first
		// filtered command on Enter — matches typical command-palette UX.
		pickIndex(0)
	}

	content := container.NewBorder(entry, nil, nil, nil, list)
	// Constrain the popup to a comfortable size.
	wrapper := container.NewGridWrap(fyne.NewSize(520, 340), content)

	pop = widget.NewModalPopUp(wrapper, canvas)
	pop.Show()
	canvas.Focus(entry)
	if len(state.filtered) > 0 {
		list.Select(0)
	}
}

type paletteState struct {
	commands []PaletteCommand
	filtered []PaletteCommand
}

func filterCommands(all []PaletteCommand, q string) []PaletteCommand {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return all
	}
	var out []PaletteCommand
	for _, c := range all {
		if strings.Contains(strings.ToLower(c.Title), q) ||
			strings.Contains(strings.ToLower(c.Subtitle), q) {
			out = append(out, c)
		}
	}
	return out
}
