package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/facubozzi/fyne-writer/internal/ai"
	"github.com/facubozzi/fyne-writer/internal/doc"
	"github.com/facubozzi/fyne-writer/internal/editor"
	"github.com/facubozzi/fyne-writer/internal/ui"
)

// parseSynonyms accepts either a JSON object {"synonyms":[...]} or a bare
// JSON array. Returns the list of synonyms; missing/malformed → empty.
func parseSynonyms(text string) []string {
	text = strings.TrimSpace(text)
	// Strip optional ```json fences.
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	type wrap struct {
		Synonyms []string `json:"synonyms"`
	}
	var w wrap
	if err := json.Unmarshal([]byte(text), &w); err == nil && len(w.Synonyms) > 0 {
		return dedupeStrings(w.Synonyms)
	}
	var arr []string
	if err := json.Unmarshal([]byte(text), &arr); err == nil && len(arr) > 0 {
		return dedupeStrings(arr)
	}
	return nil
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// showSynonymPicker pops a small dialog with one button per synonym. Click
// a button to replace the word and dismiss.
func showSynonymPicker(win fyne.Window, word string, syns []string, replace func(string)) {
	var dlg dialog.Dialog
	buttons := make([]fyne.CanvasObject, 0, len(syns))
	for _, s := range syns {
		s := s
		btn := widget.NewButton(s, func() {
			replace(s)
			if dlg != nil {
				dlg.Hide()
			}
		})
		buttons = append(buttons, btn)
	}
	content := container.NewVBox(
		widget.NewLabelWithStyle(fmt.Sprintf("Synonyms for \"%s\"", word), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVBox(buttons...),
	)
	dlg = dialog.NewCustom("Synonyms", "Close", content, win)
	dlg.Resize(fyne.NewSize(320, 360))
	dlg.Show()
}

// openCommandPalette shows the cmd+K overlay. The palette mixes AI commands
// (which require a selection) with always-available app commands (file,
// settings, background instructions) so it's useful in both states.
func (a *App) openCommandPalette() {
	hasSelection := a.editor.HasSelection()
	var cmds []ui.PaletteCommand

	// Ask AI leads — the free-form instruction box is the headline use case.
	askHint := ""
	if !hasSelection {
		askHint = "select some text first"
	}
	cmds = append(cmds, ui.PaletteCommand{
		Title:        "Ask AI…",
		Subtitle:     "Give the AI a free-form instruction for the selected text.",
		Disabled:     !hasSelection,
		DisabledHint: askHint,
		Run:          a.openAskAI,
	})

	// Built-in AI commands next.
	for _, s := range ai.BuiltinCommands() {
		s := s
		disabled := s.NeedsRange && !hasSelection
		hint := ""
		if disabled {
			hint = "select some text first"
		}
		cmds = append(cmds, ui.PaletteCommand{
			Title:        s.Title,
			Subtitle:     s.Description,
			Disabled:     disabled,
			DisabledHint: hint,
			Run:          func() { a.runAICommand(s.Kind) },
		})
	}

	// User-defined prompts come next, after the built-ins.
	if customs, err := a.store.ListPrompts(); err == nil {
		for _, p := range customs {
			p := p
			disabled := p.RequiresSelection && !hasSelection
			hint := ""
			if disabled {
				hint = "select some text first"
			}
			subtitle := p.Description
			if p.Hotkey != "" {
				if subtitle != "" {
					subtitle = subtitle + "  ·  "
				}
				subtitle = subtitle + p.Hotkey
			}
			cmds = append(cmds, ui.PaletteCommand{
				Title:        p.Name,
				Subtitle:     subtitle,
				Disabled:     disabled,
				DisabledHint: hint,
				Run:          func() { a.runCustomPrompt(p) },
			})
		}
	}

	// Always-available app commands.
	cmds = append(cmds,
		ui.PaletteCommand{
			Title:    "Check Document",
			Subtitle: "Scan the document for grammar, clarity, and style issues.",
			Run:      a.runDocumentCheck,
		},
		ui.PaletteCommand{
			Title:    "Background Instructions...",
			Subtitle: "Edit the per-document AI guidance (audience, voice, etc.).",
			Run:      a.editContext,
		},
		ui.PaletteCommand{
			Title:    "New Document",
			Subtitle: "Discard current document and start fresh.",
			Run:      a.fileNew,
		},
		ui.PaletteCommand{
			Title:    "Open File...",
			Subtitle: "Open a .md file from disk.",
			Run:      a.fileOpen,
		},
		ui.PaletteCommand{
			Title:    "Save",
			Subtitle: "Save the current document.",
			Run:      a.fileSave,
		},
		ui.PaletteCommand{
			Title:    "Save As...",
			Subtitle: "Save the current document to a new file.",
			Run:      a.fileSaveAs,
		},
		ui.PaletteCommand{
			Title:    "Prompts Library...",
			Subtitle: "Create, edit, or delete your custom AI prompts.",
			Run:      a.openPromptsLibrary,
		},
		ui.PaletteCommand{
			Title:    "Settings...",
			Subtitle: "Choose provider, model, and theme.",
			Run:      a.openSettings,
		},
	)

	ui.ShowCommandPalette(a.window, cmds)
}

// runAICommand executes a built-in AI command on the current selection,
// streaming the response into the editor.
func (a *App) runAICommand(kind ai.CommandKind) {
	d := a.editor.Document()
	selection := a.editor.SelectionText()
	if selection == "" {
		dialog.ShowInformation("AI", "Select some text first.", a.window)
		return
	}
	model := a.modelFor(a.registry.Active().Name())
	in := ai.PromptInputs{
		Selection: selection,
		Document:  documentPlainText(d),
		Context:   d.Meta.Instructions,
	}
	a.streamReplaceSelection(commandLabel(kind), ai.BuildRequest(kind, in, model))
}

// openAskAI prompts for a free-form instruction, then rewrites the current
// selection per that instruction (WRI-8). The result replaces the selection,
// same as the built-in commands.
func (a *App) openAskAI() {
	if a.editor.SelectionText() == "" {
		dialog.ShowInformation("Ask AI", "Select some text first.", a.window)
		return
	}
	var dlg dialog.Dialog
	var entry *submitEntry
	submit := func() {
		instruction := strings.TrimSpace(entry.Text)
		if instruction == "" {
			return
		}
		dlg.Hide()
		a.runAskAI(instruction)
	}
	entry = newSubmitEntry("e.g. finish this article in the same writing style (Shift+Enter for a new line)", submit)
	dlg = dialog.NewCustomConfirm("Ask AI", "Send", "Cancel", entry,
		func(ok bool) {
			if ok {
				submit()
			}
		},
		a.window,
	)
	dlg.Resize(fyne.NewSize(520, 240))
	dlg.Show()
	// Focus the entry so the user can type immediately without clicking in.
	a.window.Canvas().Focus(entry)
}

// runAskAI streams the AI's response to a free-form instruction into the
// current selection.
func (a *App) runAskAI(instruction string) {
	if instruction == "" {
		return
	}
	d := a.editor.Document()
	selection := a.editor.SelectionText()
	if selection == "" {
		dialog.ShowInformation("Ask AI", "Select some text first.", a.window)
		return
	}
	model := a.modelFor(a.registry.Active().Name())
	in := ai.PromptInputs{
		Selection:   selection,
		Document:    documentPlainText(d),
		Context:     d.Meta.Instructions,
		Instruction: instruction,
	}
	a.streamReplaceSelection("Ask AI", ai.BuildRequest(ai.CmdAskAI, in, model))
}

// streamReplaceSelection replaces the current selection with streamed AI
// output for req, as a single undo step. Shared by the built-in commands and
// Ask AI. label names the operation in the floating task card.
func (a *App) streamReplaceSelection(label string, req ai.Request) {
	provider := a.registry.Active()
	handle := a.editor.BeginAIReplace()
	if handle == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	id := a.tasks.start(label, cancel)

	go func() {
		stream, err := provider.Stream(ctx, req)
		if err != nil {
			fyne.Do(func() {
				a.tasks.finish(id)
				handle.Cancel()
				if ctx.Err() == nil { // not a user cancel — surface the failure
					dialog.ShowError(err, a.window)
				}
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
			a.tasks.finish(id)
			if ctx.Err() != nil { // user cancelled: revert partial text
				handle.Cancel()
				return
			}
			if failed != nil {
				handle.Cancel()
				dialog.ShowError(failed, a.window)
				return
			}
			handle.Commit()
		})
	}()
}

// commandLabel returns the built-in command's display title for use as a task
// label. Falls back to "AI" for kinds without a builtin spec.
func commandLabel(kind ai.CommandKind) string {
	for _, s := range ai.BuiltinCommands() {
		if s.Kind == kind {
			return s.Title
		}
	}
	return "AI"
}

// modelFor returns the configured default model for the given provider.
// User overrides from the Settings dialog take precedence over .env defaults.
func (a *App) modelFor(provider string) string {
	switch provider {
	case ai.ProviderAnthropic:
		if a.anthropicModel != "" {
			return a.anthropicModel
		}
		return a.cfg.AnthropicModel
	case ai.ProviderOpenAI:
		if a.openaiModel != "" {
			return a.openaiModel
		}
		return a.cfg.OpenAIModel
	default:
		return ""
	}
}

// documentPlainText returns a plain-text rendering of the document for use
// as context in AI prompts (NOT the same as the markdown serialization).
// We just concatenate block texts with blank lines.
func documentPlainText(d *doc.Document) string {
	if d == nil {
		return ""
	}
	parts := make([]string, 0, len(d.Blocks))
	for _, b := range d.Blocks {
		t := strings.TrimSpace(b.PlainText())
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

// editContext shows the background-instructions dialog.
func (a *App) editContext() {
	current := a.editor.Document().Meta.Instructions
	entry := newMultilineEntry(current, "Background instructions for this document. The AI uses these on every call (paraphrase, synonyms, ...). Examples:\nWrite in a formal academic tone.\nAudience: senior engineers.")
	dlg := dialog.NewCustomConfirm("Background Instructions", "Save", "Cancel",
		entry,
		func(ok bool) {
			if !ok {
				return
			}
			a.editor.SetDocMeta(doc.DocMeta{Instructions: strings.TrimSpace(entry.Text)})
		},
		a.window,
	)
	dlg.Resize(fyne.NewSize(520, 320))
	dlg.Show()
}

// installContextMenuExtender hooks AI items into the editor's right-click
// menu: Paraphrase / Shorten / Expand / Fix Tone when there's a selection,
// Synonyms... when the click was on a word. Custom prompts that require a
// selection are appended after the built-ins so users can launch their own
// commands the same way.
func (a *App) installContextMenuExtender() {
	a.editor.SetContextMenuExtender(func(t editor.ContextMenuTarget) []*fyne.MenuItem {
		if t.HasSelection {
			items := []*fyne.MenuItem{
				fyne.NewMenuItem("Ask AI…", a.openAskAI),
				fyne.NewMenuItem("Paraphrase", func() { a.runAICommand(ai.CmdParaphrase) }),
				fyne.NewMenuItem("Shorten", func() { a.runAICommand(ai.CmdShorten) }),
				fyne.NewMenuItem("Expand", func() { a.runAICommand(ai.CmdExpand) }),
				fyne.NewMenuItem("Fix tone (formal)", func() { a.runAICommand(ai.CmdFixTone) }),
				fyne.NewMenuItem("Add comment…", a.openAddCommentDialog),
			}
			if customs, err := a.store.ListPrompts(); err == nil {
				for _, p := range customs {
					if !p.RequiresSelection {
						continue
					}
					p := p
					items = append(items, fyne.NewMenuItem(p.Name, func() { a.runCustomPrompt(p) }))
				}
			}
			return items
		}
		if t.Word == "" || t.ReplaceWord == nil {
			return nil
		}
		return []*fyne.MenuItem{a.synonymsMenuItem(t.Word, t.Sentence, t.ReplaceWord)}
	})
}

// synonymsMenuItem builds the "Synonyms" right-click entry. When synonyms for
// this word+sentence are already cached (from a prior lookup), they appear as
// clickable children inline; otherwise the submenu offers a "Look up…" item
// that runs the async popup. No speculative API calls are made.
func (a *App) synonymsMenuItem(word, sentence string, replace func(string)) *fyne.MenuItem {
	parent := fyne.NewMenuItem(fmt.Sprintf("Synonyms for \"%s\"", word), nil)
	var children []*fyne.MenuItem
	if cached := a.synonymCache[synonymKey(word, sentence)]; len(cached) > 0 {
		for _, s := range cached {
			s := s
			children = append(children, fyne.NewMenuItem(s, func() { replace(s) }))
		}
		children = append(children,
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Look up again…", func() { a.showSynonyms(word, sentence, replace) }),
		)
	} else {
		children = append(children,
			fyne.NewMenuItem("Look up synonyms…", func() { a.showSynonyms(word, sentence, replace) }),
		)
	}
	parent.ChildMenu = fyne.NewMenu("", children...)
	return parent
}

// synonymKey keys the synonym cache by word + surrounding sentence, since
// synonyms are context-aware.
func synonymKey(word, sentence string) string { return word + "\x00" + sentence }

// showSynonyms fires an AI call for context-aware synonyms of `word` in
// `sentence` and opens a popup of clickable replacements.
func (a *App) showSynonyms(word, sentence string, replace func(string)) {
	provider := a.registry.Active()
	model := a.modelFor(provider.Name())
	d := a.editor.Document()

	ctx, cancel := context.WithCancel(context.Background())
	id := a.tasks.start("Synonyms: "+word, cancel)

	go func() {
		req := ai.BuildRequest(ai.CmdSynonyms, ai.PromptInputs{
			Selection: word,
			Sentence:  sentence,
			Context:   d.Meta.Instructions,
		}, model)
		resp, err := provider.Generate(ctx, req)
		fyne.Do(func() {
			a.tasks.finish(id)
			if err != nil {
				if ctx.Err() == nil { // not a user cancel
					dialog.ShowError(err, a.window)
				}
				return
			}
			synonyms := parseSynonyms(resp.Text)
			if len(synonyms) == 0 {
				dialog.ShowInformation("Synonyms", "No suggestions returned.", a.window)
				return
			}
			if a.synonymCache == nil {
				a.synonymCache = map[string][]string{}
			}
			a.synonymCache[synonymKey(word, sentence)] = synonyms
			showSynonymPicker(a.window, word, synonyms, replace)
		})
	}()
}

// fmt import guard: ensure package compiles when nothing else uses fmt.
var _ = fmt.Sprintf
