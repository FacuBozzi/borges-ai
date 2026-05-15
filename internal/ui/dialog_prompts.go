package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// PromptRow is the data shown for one prompt in the library list.
// We keep the UI layer's data type minimal so callers (the app layer) can
// translate to/from their own store types without leaking SQL details here.
type PromptRow struct {
	ID                int64
	Name              string
	Description       string
	Template          string
	Hotkey            string
	RequiresSelection bool
}

// PromptsLibraryCallbacks bundles the app-side actions the library invokes
// on user edits. The dialog itself owns no state — every change round-trips
// through these callbacks and the dialog calls `reload()` to refresh.
type PromptsLibraryCallbacks struct {
	List   func() []PromptRow
	Save   func(PromptRow) error // Create when ID == 0, Update otherwise
	Delete func(id int64) error
}

// ShowPromptsLibrary opens the modal CRUD dialog. The dialog repeatedly
// calls cb.List() to render rows and cb.Save / cb.Delete to mutate state.
func ShowPromptsLibrary(win fyne.Window, cb PromptsLibraryCallbacks) {
	list := container.NewVBox()
	scroll := container.NewVScroll(list)
	scroll.SetMinSize(fyne.NewSize(560, 360))

	var dlg dialog.Dialog
	var refresh func()

	refresh = func() {
		list.RemoveAll()
		rows := cb.List()
		if len(rows) == 0 {
			empty := widget.NewLabel("No custom prompts yet. Click \"New prompt...\" to add one.")
			empty.Importance = widget.LowImportance
			list.Add(empty)
		}
		for _, r := range rows {
			r := r
			title := widget.NewLabel(r.Name)
			title.TextStyle = fyne.TextStyle{Bold: true}
			meta := r.Description
			if r.Hotkey != "" {
				if meta != "" {
					meta = meta + "  ·  "
				}
				meta = meta + "Hotkey: " + r.Hotkey
			}
			sub := widget.NewLabel(meta)
			sub.Importance = widget.LowImportance
			sub.Wrapping = fyne.TextWrapWord

			editBtn := widget.NewButton("Edit", func() {
				showPromptEditor(win, r, func(updated PromptRow) {
					if err := cb.Save(updated); err != nil {
						dialog.ShowError(err, win)
						return
					}
					refresh()
				})
			})
			delBtn := widget.NewButton("Delete", func() {
				dialog.ShowConfirm("Delete prompt",
					"Delete \""+r.Name+"\"? This cannot be undone.",
					func(ok bool) {
						if !ok {
							return
						}
						if err := cb.Delete(r.ID); err != nil {
							dialog.ShowError(err, win)
							return
						}
						refresh()
					}, win)
			})
			buttons := container.NewHBox(editBtn, delBtn)
			row := container.NewBorder(nil, nil, nil, buttons,
				container.NewVBox(title, sub),
			)
			list.Add(row)
			list.Add(widget.NewSeparator())
		}
		list.Refresh()
	}

	newBtn := widget.NewButton("New prompt...", func() {
		showPromptEditor(win, PromptRow{RequiresSelection: true}, func(p PromptRow) {
			if err := cb.Save(p); err != nil {
				dialog.ShowError(err, win)
				return
			}
			refresh()
		})
	})
	top := container.NewBorder(nil, nil, nil, newBtn,
		widget.NewLabelWithStyle("Custom prompts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	content := container.NewBorder(top, nil, nil, nil, scroll)
	dlg = dialog.NewCustom("Prompts Library", "Close", content, win)
	dlg.Resize(fyne.NewSize(640, 480))
	refresh()
	dlg.Show()
}

// showPromptEditor opens the inner dialog used to create or edit a single
// prompt. onSave is invoked with the populated row when the user clicks Save.
func showPromptEditor(win fyne.Window, current PromptRow, onSave func(PromptRow)) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(current.Name)
	nameEntry.SetPlaceHolder("e.g. Make persuasive")

	descEntry := widget.NewEntry()
	descEntry.SetText(current.Description)
	descEntry.SetPlaceHolder("Short description shown in the palette")

	hotkeyEntry := widget.NewEntry()
	hotkeyEntry.SetText(current.Hotkey)
	hotkeyEntry.SetPlaceHolder("e.g. Cmd+Shift+P  (leave empty for none)")

	reqCheck := widget.NewCheck("Requires selection", nil)
	reqCheck.Checked = current.RequiresSelection

	tmplEntry := widget.NewMultiLineEntry()
	tmplEntry.SetText(current.Template)
	tmplEntry.SetPlaceHolder("Prompt template. Variables:\n  {{.Selection}}  selected text\n  {{.Document}}   plain-text body\n  {{.Word}}       clicked word (right-click only)\n  {{.Context}}    background instructions")
	tmplEntry.Wrapping = fyne.TextWrapWord
	tmplScroll := container.NewVScroll(tmplEntry)
	tmplScroll.SetMinSize(fyne.NewSize(520, 220))

	form := container.NewVBox(
		labeledRow("Name", nameEntry),
		labeledRow("Description", descEntry),
		labeledRow("Hotkey", hotkeyEntry),
		labeledRow("", reqCheck),
		widget.NewLabelWithStyle("Template", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		tmplScroll,
	)

	title := "New prompt"
	if current.ID != 0 {
		title = "Edit prompt"
	}
	dlg := dialog.NewCustomConfirm(title, "Save", "Cancel", form,
		func(ok bool) {
			if !ok {
				return
			}
			onSave(PromptRow{
				ID:                current.ID,
				Name:              nameEntry.Text,
				Description:       descEntry.Text,
				Template:          tmplEntry.Text,
				Hotkey:            hotkeyEntry.Text,
				RequiresSelection: reqCheck.Checked,
			})
		}, win,
	)
	dlg.Resize(fyne.NewSize(620, 520))
	dlg.Show()
}
