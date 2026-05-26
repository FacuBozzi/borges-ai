package ui

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// FileDialogConfig carries what the app supplies to the custom file pickers.
// OnDirChange lets the app persist the last-used directory without coupling
// internal/ui to the store.
type FileDialogConfig struct {
	Window      fyne.Window
	StartDir    string // last-used dir; falls back to home when empty/invalid
	Extensions  []string
	OnDirChange func(dir string)
}

// ShowOpenFile opens a modal picker for an existing file. onChosen is called
// with the absolute path only when the user confirms; cancel/Esc just close.
func ShowOpenFile(cfg FileDialogConfig, onChosen func(path string)) {
	var closePopup func()
	canvas := cfg.Window.Canvas()

	dl := newDirList(cfg.Extensions)
	pathLabel := newPathLabel()
	filter := newEscEntry(func() { closePopup() })
	filter.SetPlaceHolder("Filter…")

	refocus := func() { canvas.Focus(filter) }
	dl.onDir = func(dir string) {
		pathLabel.SetText(dir)
		if cfg.OnDirChange != nil {
			cfg.OnDirChange(dir)
		}
		refocus()
	}
	dl.onErr = func(err error) { pathLabel.SetText("⚠ " + err.Error()) }
	dl.onFile = func(string) { refocus() }

	confirm := func() {
		p := dl.chosenFile()
		if p == "" {
			return
		}
		closePopup()
		onChosen(p)
	}
	filter.OnChanged = func(q string) { dl.setFilter(q) }
	filter.OnSubmitted = func(string) { confirm() }

	openBtn := widget.NewButton("Open", confirm)
	openBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", func() { closePopup() })

	top := container.NewVBox(pathLabel, filter)
	footer := container.NewBorder(nil, nil, nil, container.NewHBox(cancelBtn, openBtn))
	body := container.NewBorder(top, footer, nil, nil, dl.list)

	_, closePopup = showModal(cfg.Window, fyne.NewSize(620, 460), body, filter)
	dl.navigate(resolveStartDir(cfg))
}

// ShowSaveFile opens a modal picker for a destination directory + filename.
// defaultName seeds the filename field. onChosen receives the joined absolute
// path (with a ".md" extension added when none is typed); cancel/Esc close.
func ShowSaveFile(cfg FileDialogConfig, defaultName string, onChosen func(path string)) {
	var closePopup func()

	dl := newDirList(cfg.Extensions)
	pathLabel := newPathLabel()
	nameEntry := newEscEntry(func() { closePopup() })
	nameEntry.SetText(defaultName)

	dl.onDir = func(dir string) {
		pathLabel.SetText(dir)
		if cfg.OnDirChange != nil {
			cfg.OnDirChange(dir)
		}
	}
	dl.onErr = func(err error) { pathLabel.SetText("⚠ " + err.Error()) }
	dl.onFile = func(p string) { nameEntry.SetText(filepath.Base(p)) }

	confirm := func() {
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			return
		}
		if filepath.Ext(name) == "" {
			name += ".md"
		}
		full := filepath.Join(dl.dir, name)
		commit := func() {
			closePopup()
			onChosen(full)
		}
		if _, err := os.Stat(full); err == nil {
			dialog.NewConfirm("Overwrite?",
				filepath.Base(full)+" already exists. Overwrite it?",
				func(ok bool) {
					if ok {
						commit()
					}
				}, cfg.Window).Show()
			return
		}
		commit()
	}
	nameEntry.OnSubmitted = func(string) { confirm() }

	saveBtn := widget.NewButton("Save", confirm)
	saveBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", func() { closePopup() })

	footer := container.NewBorder(nil, nil, nil,
		container.NewHBox(cancelBtn, saveBtn), nameEntry)
	body := container.NewBorder(pathLabel, footer, nil, nil, dl.list)

	_, closePopup = showModal(cfg.Window, fyne.NewSize(620, 460), body, nameEntry)
	dl.navigate(resolveStartDir(cfg))
}

func newPathLabel() *widget.Label {
	l := widget.NewLabel("")
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

func resolveStartDir(cfg FileDialogConfig) string {
	if cfg.StartDir != "" {
		if fi, err := os.Stat(cfg.StartDir); err == nil && fi.IsDir() {
			return cfg.StartDir
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}
