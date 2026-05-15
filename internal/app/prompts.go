package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/facubozzi/fyne-writer/internal/ai"
	"github.com/facubozzi/fyne-writer/internal/store"
	"github.com/facubozzi/fyne-writer/internal/ui"
)

// openPromptsLibrary shows the CRUD dialog. Saves go through the store; on
// every change we re-register canvas hotkeys so prompts with new shortcuts
// take effect immediately.
func (a *App) openPromptsLibrary() {
	cb := ui.PromptsLibraryCallbacks{
		List: func() []ui.PromptRow {
			prompts, err := a.store.ListPrompts()
			if err != nil {
				dialog.ShowError(err, a.window)
				return nil
			}
			out := make([]ui.PromptRow, 0, len(prompts))
			for _, p := range prompts {
				out = append(out, promptToRow(p))
			}
			return out
		},
		Save: func(r ui.PromptRow) error {
			if strings.TrimSpace(r.Name) == "" {
				return errors.New("name is required")
			}
			if strings.TrimSpace(r.Template) == "" {
				return errors.New("template is required")
			}
			if r.Hotkey != "" {
				if _, _, err := parseHotkey(r.Hotkey); err != nil {
					return fmt.Errorf("hotkey: %w", err)
				}
			}
			if _, err := template.New("p").Parse(r.Template); err != nil {
				return fmt.Errorf("template: %w", err)
			}
			p := rowToPrompt(r)
			if p.ID == 0 {
				_, err := a.store.CreatePrompt(p)
				if err != nil {
					return err
				}
			} else {
				if err := a.store.UpdatePrompt(p); err != nil {
					return err
				}
			}
			a.refreshPromptShortcuts()
			return nil
		},
		Delete: func(id int64) error {
			if err := a.store.DeletePrompt(id); err != nil {
				return err
			}
			a.refreshPromptShortcuts()
			return nil
		},
	}
	ui.ShowPromptsLibrary(a.window, cb)
}

// refreshPromptShortcuts unregisters any previously-registered prompt
// shortcuts and re-registers from the current store contents. Safe to call
// repeatedly; called once at startup and on every library mutation.
func (a *App) refreshPromptShortcuts() {
	c := a.window.Canvas()
	for _, s := range a.promptShortcuts {
		c.RemoveShortcut(s)
	}
	a.promptShortcuts = a.promptShortcuts[:0]

	prompts, err := a.store.ListPrompts()
	if err != nil {
		return
	}
	for _, p := range prompts {
		if p.Hotkey == "" {
			continue
		}
		key, mod, err := parseHotkey(p.Hotkey)
		if err != nil {
			continue
		}
		sc := &desktop.CustomShortcut{KeyName: key, Modifier: mod}
		id := p.ID
		c.AddShortcut(sc, func(fyne.Shortcut) { a.runCustomPromptByID(id) })
		a.promptShortcuts = append(a.promptShortcuts, sc)
	}
}

// runCustomPromptByID looks up a prompt by ID and runs it.
func (a *App) runCustomPromptByID(id int64) {
	prompts, err := a.store.ListPrompts()
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	for _, p := range prompts {
		if p.ID == id {
			a.runCustomPrompt(p)
			return
		}
	}
}

// runCustomPrompt executes a user-defined prompt. Selection requirements are
// enforced here so hotkey invocations don't silently no-op. Prompts run as
// streaming replacements when a selection exists, falling back to a generated
// modal preview otherwise.
func (a *App) runCustomPrompt(p store.Prompt) {
	hasSelection := a.editor.HasSelection()
	if p.RequiresSelection && !hasSelection {
		dialog.ShowInformation("AI", "Select some text first.", a.window)
		return
	}

	d := a.editor.Document()
	inputs := ai.PromptInputs{
		Selection: a.editor.SelectionText(),
		Document:  documentPlainText(d),
		Context:   d.Meta.Instructions,
	}
	rendered, err := renderTemplate(p.Template, inputs)
	if err != nil {
		dialog.ShowError(fmt.Errorf("template: %w", err), a.window)
		return
	}

	provider := a.registry.Active()
	model := a.modelFor(provider.Name())
	req := ai.Request{
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: rendered}},
		Model:       model,
		MaxTokens:   1024,
		Temperature: 0.7,
	}

	if !hasSelection {
		a.runCustomPromptPreview(provider, req, p.Name)
		return
	}

	handle := a.editor.BeginAIReplace()
	if handle == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	_ = cancel
	go func() {
		stream, err := provider.Stream(ctx, req)
		if err != nil {
			fyne.Do(func() {
				handle.Cancel()
				dialog.ShowError(err, a.window)
			})
			return
		}
		var failed error
		for chunk := range stream {
			if chunk.Err != nil {
				failed = chunk.Err
				break
			}
			if chunk.Delta != "" {
				delta := chunk.Delta
				fyne.Do(func() { handle.Append(delta) })
			}
		}
		fyne.Do(func() {
			if failed != nil {
				handle.Cancel()
				dialog.ShowError(failed, a.window)
				return
			}
			handle.Commit()
		})
	}()
}

// runCustomPromptPreview is the no-selection path: shows the response in a
// modal so prompts can be used as one-off queries without overwriting any
// text. Kept simple — paste-back is not yet wired.
func (a *App) runCustomPromptPreview(provider ai.Provider, req ai.Request, name string) {
	loading := dialog.NewCustomWithoutButtons(name, widgetLoading("Generating..."), a.window)
	loading.Show()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60_000_000_000)
		defer cancel()
		resp, err := provider.Generate(ctx, req)
		fyne.Do(func() {
			loading.Hide()
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			out := newMultilineEntry(resp.Text, "")
			out.Wrapping = fyne.TextWrapWord
			dlg := dialog.NewCustom(name, "Close", out, a.window)
			dlg.Resize(fyne.NewSize(620, 420))
			dlg.Show()
		})
	}()
}

// renderTemplate executes p.Template against PromptInputs. Errors surface to
// the user since they signal a malformed user template, not an internal bug.
func renderTemplate(tmpl string, in ai.PromptInputs) (string, error) {
	t, err := template.New("p").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, in); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// promptToRow / rowToPrompt translate between the store type and the dialog
// type so the UI package doesn't depend on internal/store.
func promptToRow(p store.Prompt) ui.PromptRow {
	return ui.PromptRow{
		ID:                p.ID,
		Name:              p.Name,
		Description:       p.Description,
		Template:          p.Template,
		Hotkey:            p.Hotkey,
		RequiresSelection: p.RequiresSelection,
	}
}
func rowToPrompt(r ui.PromptRow) store.Prompt {
	return store.Prompt{
		ID:                r.ID,
		Name:              strings.TrimSpace(r.Name),
		Description:       strings.TrimSpace(r.Description),
		Template:          r.Template,
		Hotkey:            strings.TrimSpace(r.Hotkey),
		RequiresSelection: r.RequiresSelection,
	}
}

// parseHotkey turns a string like "Cmd+Shift+P" into a fyne KeyName + modifier
// bitmask. Recognized modifiers: Cmd/Meta/Mod (→ ShortcutDefault), Ctrl,
// Shift, Alt/Option. The key must be a single A–Z letter or 0–9 digit.
func parseHotkey(s string) (fyne.KeyName, fyne.KeyModifier, error) {
	parts := strings.Split(s, "+")
	if len(parts) < 2 {
		return "", 0, errors.New("expected at least one modifier and a key, e.g. Cmd+Shift+P")
	}
	var mod fyne.KeyModifier
	for _, raw := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "cmd", "meta", "super", "mod":
			mod |= fyne.KeyModifierShortcutDefault
		case "ctrl", "control":
			mod |= fyne.KeyModifierControl
		case "shift":
			mod |= fyne.KeyModifierShift
		case "alt", "option", "opt":
			mod |= fyne.KeyModifierAlt
		default:
			return "", 0, fmt.Errorf("unknown modifier %q", raw)
		}
	}
	keyStr := strings.TrimSpace(parts[len(parts)-1])
	if len(keyStr) != 1 {
		return "", 0, fmt.Errorf("key must be a single letter or digit, got %q", keyStr)
	}
	r := []rune(strings.ToUpper(keyStr))[0]
	switch {
	case r >= 'A' && r <= 'Z':
		return fyne.KeyName(string(r)), mod, nil
	case r >= '0' && r <= '9':
		return fyne.KeyName(string(r)), mod, nil
	}
	return "", 0, fmt.Errorf("unsupported key %q", keyStr)
}
