package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const onboardingBody = `## 1. Add your API key
Copy ` + "`.env.example`" + ` to ` + "`.env`" + ` and set ` + "`ANTHROPIC_API_KEY`" + ` and/or ` + "`OPENAI_API_KEY`" + `. Without a key the app still runs with a mock provider that echoes input.

## 2. Learn the shortcuts
Press **⌘/** anytime for the full keyboard cheatsheet. **⌘K** opens the AI command palette; **⌘⇧K** checks the document.

## 3. Read the docs
See **README.md** for the full feature tour and architecture notes.`

// ShowOnboarding shows the first-run welcome modal. onDone fires when the user
// clicks "Got it" (the caller persists the "seen" flag).
func ShowOnboarding(win fyne.Window, onDone func()) {
	var closePopup func()

	title := widget.NewLabelWithStyle("Welcome to Fyne Writer",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	steps := widget.NewRichTextFromMarkdown(onboardingBody)
	steps.Wrapping = fyne.TextWrapWord

	gotIt := widget.NewButton("Got it", func() {
		if onDone != nil {
			onDone()
		}
		closePopup()
	})
	gotIt.Importance = widget.HighImportance

	body := container.NewBorder(title, gotIt, nil, nil, container.NewVScroll(steps))
	_, closePopup = showModal(win, fyne.NewSize(520, 460), body, nil)
}
