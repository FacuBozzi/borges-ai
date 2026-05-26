package app

import (
	"fyne.io/fyne/v2/dialog"

	"github.com/facubozzi/fyne-writer/internal/ai"
	"github.com/facubozzi/fyne-writer/internal/store"
	"github.com/facubozzi/fyne-writer/internal/ui"
)

// openSettings shows the Settings dialog and, on Save, persists choices to
// SQLite and hot-applies them to the registry + theme.
func (a *App) openSettings() {
	opts := ui.SettingsOptions{
		AvailableProviders: a.registry.Available(),
		AnthropicModels:    a.providerModels(ai.ProviderAnthropic),
		OpenAIModels:       a.providerModels(ai.ProviderOpenAI),
	}
	current := ui.SettingsValues{
		ActiveProvider: a.registry.ActiveName(),
		AnthropicModel: a.modelFor(ai.ProviderAnthropic),
		OpenAIModel:    a.modelFor(ai.ProviderOpenAI),
		ThemeVariant:   a.themeVariant,
	}
	if current.ThemeVariant == "" {
		current.ThemeVariant = string(ui.VariantSystem)
	}

	ui.ShowSettingsDialog(a.window, current, opts, func(v ui.SettingsValues) {
		if err := a.applySettings(v); err != nil {
			dialog.ShowError(err, a.window)
		}
	})
}

// applySettings persists v to the store and hot-applies it. Returns the
// first persistence error encountered; hot-apply still happens for the
// fields that did persist.
func (a *App) applySettings(v ui.SettingsValues) error {
	var firstErr error
	save := func(key, value string) {
		if err := a.store.SetSetting(key, value); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if v.ActiveProvider != "" && v.ActiveProvider != a.registry.ActiveName() {
		if a.registry.SetActive(v.ActiveProvider) {
			save(store.KeyActiveProvider, v.ActiveProvider)
		}
	} else if v.ActiveProvider != "" {
		save(store.KeyActiveProvider, v.ActiveProvider)
	}

	if v.AnthropicModel != a.modelFor(ai.ProviderAnthropic) {
		a.anthropicModel = v.AnthropicModel
		save(store.KeyAnthropicModel, v.AnthropicModel)
	}
	if v.OpenAIModel != a.modelFor(ai.ProviderOpenAI) {
		a.openaiModel = v.OpenAIModel
		save(store.KeyOpenAIModel, v.OpenAIModel)
	}

	if v.ThemeVariant != a.themeVariant {
		a.themeVariant = v.ThemeVariant
		a.fyne.Settings().SetTheme(ui.NewThemeWithVariant(ui.ThemeVariant(v.ThemeVariant)))
		// Safety valve: the wrap cache keys on size/style but not font family,
		// so any theme swap that could alter text metrics must bust it.
		a.editor.InvalidateLayoutCache()
		save(store.KeyThemeVariant, v.ThemeVariant)
	}

	a.refreshStatus()
	return firstErr
}

// providerModels returns the suggestion list (provider's Models()) when the
// provider is configured, or nil. Used to populate the combobox.
func (a *App) providerModels(name string) []string {
	for _, n := range a.registry.Available() {
		if n != name {
			continue
		}
		if p := a.registry.Get(name); p != nil {
			return p.Models()
		}
	}
	return nil
}
