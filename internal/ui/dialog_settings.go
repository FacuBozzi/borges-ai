package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// SettingsValues is the editable state shown in the Settings dialog.
// Models are per-provider rather than a single field so switching providers
// later doesn't lose the model you picked for the other one.
type SettingsValues struct {
	ActiveProvider string
	AnthropicModel string
	OpenAIModel    string
	ThemeVariant   string // "system" | "light" | "dark"
}

// SettingsOptions describes the choices to present for each picker.
// AvailableProviders is the set of providers that have keys configured.
// AnthropicModels / OpenAIModels are the suggestion lists; users can type
// a custom model name into the combobox.
type SettingsOptions struct {
	AvailableProviders []string
	AnthropicModels    []string
	OpenAIModels       []string
}

// ShowSettingsDialog opens a modal Settings dialog with the given current
// values and choice lists. onSave is invoked with the user's selections if
// they press Save; cancellation calls nothing.
func ShowSettingsDialog(win fyne.Window, current SettingsValues, opts SettingsOptions, onSave func(SettingsValues)) {
	providerSel := widget.NewSelect(opts.AvailableProviders, nil)
	if current.ActiveProvider != "" {
		providerSel.SetSelected(current.ActiveProvider)
	} else if len(opts.AvailableProviders) > 0 {
		providerSel.SetSelected(opts.AvailableProviders[0])
	}

	anthropicEntry := widget.NewSelectEntry(opts.AnthropicModels)
	anthropicEntry.SetText(current.AnthropicModel)
	openaiEntry := widget.NewSelectEntry(opts.OpenAIModels)
	openaiEntry.SetText(current.OpenAIModel)

	themeSel := widget.NewSelect([]string{"system", "light", "dark"}, nil)
	if current.ThemeVariant == "" {
		themeSel.SetSelected("system")
	} else {
		themeSel.SetSelected(current.ThemeVariant)
	}

	form := container.NewVBox(
		labeledRow("Active provider", providerSel),
		labeledRow("Anthropic model", anthropicEntry),
		labeledRow("OpenAI model", openaiEntry),
		labeledRow("Theme", themeSel),
	)

	dlg := dialog.NewCustomConfirm("Settings", "Save", "Cancel", form,
		func(ok bool) {
			if !ok {
				return
			}
			onSave(SettingsValues{
				ActiveProvider: providerSel.Selected,
				AnthropicModel: anthropicEntry.Text,
				OpenAIModel:    openaiEntry.Text,
				ThemeVariant:   themeSel.Selected,
			})
		}, win,
	)
	dlg.Resize(fyne.NewSize(460, 320))
	dlg.Show()
}

func labeledRow(label string, ctrl fyne.CanvasObject) fyne.CanvasObject {
	lbl := widget.NewLabel(label)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewBorder(nil, nil, container.NewGridWrap(fyne.NewSize(140, 36), lbl), nil, ctrl)
}
