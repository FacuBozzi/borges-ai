package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// PaletteCommand is one row in the command palette. Title is the primary
// label, Subtitle is dim hint text. When Disabled is true the row is
// greyed and DisabledHint is shown next to the title so users understand
// why it can't be picked.
type PaletteCommand struct {
	Title        string
	Subtitle     string
	Disabled     bool
	DisabledHint string // shown beside the title when Disabled
	Run          func()
}

// ShowCommandPalette opens the cmd+K overlay. The palette is a centered
// modal with a search entry and a filterable list. Enter runs the first
// match; Esc cancels.
func ShowCommandPalette(win fyne.Window, commands []PaletteCommand) {
	if len(commands) == 0 {
		return
	}
	canvas := win.Canvas()

	var pop *widget.PopUp
	closePopup := func() {
		if pop != nil {
			pop.Hide()
		}
	}

	entry := newEscEntry(closePopup)
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
			sub.Wrapping = fyne.TextWrapWord
			return container.NewVBox(title, sub)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			cmd := state.filtered[i]
			titleLabel := box.Objects[0].(*widget.Label)
			subLabel := box.Objects[1].(*widget.Label)
			title := cmd.Title
			subtitle := cmd.Subtitle
			if cmd.Disabled {
				if cmd.DisabledHint != "" {
					title = title + "  ·  " + cmd.DisabledHint
				}
				titleLabel.Importance = widget.LowImportance
				subLabel.Importance = widget.LowImportance
			} else {
				titleLabel.Importance = widget.MediumImportance
				subLabel.Importance = widget.LowImportance
			}
			titleLabel.SetText(title)
			subLabel.SetText(subtitle)
		},
	)

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

	list.OnSelected = func(i widget.ListItemID) {
		pickIndex(i)
		list.UnselectAll()
	}

	entry.OnChanged = func(q string) {
		state.filtered = filterCommands(state.commands, q)
		list.Refresh()
	}
	entry.OnSubmitted = func(string) {
		// Run the first non-disabled match on Enter.
		for i, c := range state.filtered {
			if !c.Disabled {
				pickIndex(i)
				return
			}
		}
	}

	content := container.NewBorder(entry, nil, nil, nil, list)
	wrapper := container.NewGridWrap(fyne.NewSize(560, 380), content)
	pop = widget.NewModalPopUp(wrapper, canvas)
	pop.Show()
	canvas.Focus(entry)
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

// escEntry is a widget.Entry that closes its host popup on Esc.
type escEntry struct {
	widget.Entry
	onEsc func()
}

func newEscEntry(onEsc func()) *escEntry {
	e := &escEntry{onEsc: onEsc}
	e.ExtendBaseWidget(e)
	return e
}

func (e *escEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		if e.onEsc != nil {
			e.onEsc()
		}
		return
	}
	e.Entry.TypedKey(ev)
}
