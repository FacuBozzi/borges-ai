package app

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/facubozzi/fyne-writer/internal/ai"
	"github.com/facubozzi/fyne-writer/internal/config"
	"github.com/facubozzi/fyne-writer/internal/doc"
	"github.com/facubozzi/fyne-writer/internal/editor"
	"github.com/facubozzi/fyne-writer/internal/store"
	"github.com/facubozzi/fyne-writer/internal/ui"
)

type App struct {
	fyne     fyne.App
	window   fyne.Window
	cfg      config.Config
	store    *store.Store
	registry *ai.Registry

	editor          *editor.RichEditor
	sidebar         *ui.IssuesSidebar
	versionsSidebar *ui.VersionsSidebar
	sidebarTabs     *container.AppTabs
	titleLabel      *widget.Label
	statusLabel  *widget.Label
	currentPath  string // empty until first save
	dirty        bool
	checksRunning bool

	// User-overridable model names. Empty string means fall back to cfg.
	anthropicModel string
	openaiModel    string
	themeVariant   string // "" | "system" | "light" | "dark"

	// Currently-registered custom-prompt shortcuts, tracked so we can
	// unregister them when the library changes.
	promptShortcuts []fyne.Shortcut
}

func New() (*App, error) {
	cfg := config.Load()
	st, err := store.Open()
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	reg := ai.NewRegistry(cfg)

	fa := fyneapp.NewWithID("dev.facubozzi.fynewriter")

	a := &App{fyne: fa, cfg: cfg, store: st, registry: reg}
	a.loadPersistedSettings()
	fa.Settings().SetTheme(ui.NewThemeWithVariant(ui.ThemeVariant(a.themeVariant)))

	a.window = fa.NewWindow("Fyne Writer")
	a.window.Resize(fyne.NewSize(1100, 720))
	a.window.SetContent(a.buildContent())
	a.window.SetMainMenu(a.buildMenu())
	a.window.SetOnClosed(func() { _ = st.Close() })
	a.window.Canvas().Focus(a.editor)
	a.registerEditorShortcuts()
	a.installContextMenuExtender()
	a.refreshPromptShortcuts()
	return a, nil
}

// loadPersistedSettings reads previously-saved provider/model/theme choices
// from the SQLite settings table and applies them to the registry + app
// state. Missing keys leave the .env defaults in place.
func (a *App) loadPersistedSettings() {
	if v, _ := a.store.GetSetting(store.KeyActiveProvider); v != "" {
		a.registry.SetActive(v)
	}
	a.anthropicModel, _ = a.store.GetSetting(store.KeyAnthropicModel)
	a.openaiModel, _ = a.store.GetSetting(store.KeyOpenAIModel)
	a.themeVariant, _ = a.store.GetSetting(store.KeyThemeVariant)
}

// registerEditorShortcuts wires the editor's mark-toggle shortcuts
// (cmd+B/I/U/E + cmd+shift+X) and block-type shortcuts (cmd+1/2/3 for
// headings, cmd+0 for paragraph) onto the window canvas so the glfw driver
// constructs CustomShortcut objects on match and dispatches them to the
// focused editor.
func (a *App) registerEditorShortcuts() {
	c := a.window.Canvas()
	for _, b := range editor.MarkShortcutBindings() {
		mark := b.Mark
		c.AddShortcut(b.Shortcut, func(fyne.Shortcut) {
			a.editor.ToggleMark(mark)
		})
	}
	for _, b := range editor.BlockShortcutBindings() {
		apply := b.Apply
		c.AddShortcut(b.Shortcut, func(fyne.Shortcut) {
			apply(a.editor)
		})
	}
}

func (a *App) Run()                 { a.window.ShowAndRun() }
func (a *App) Registry() *ai.Registry { return a.registry }

func (a *App) buildContent() fyne.CanvasObject {
	a.editor = editor.New(doc.New())
	a.editor.OnChanged(a.onDocChanged)
	a.editor.OnIssuesChanged(func() { fyne.Do(a.updateSidebarFromEditor) })

	a.sidebar = ui.NewIssuesSidebar()
	a.sidebar.OnCheck = a.runDocumentCheck
	a.sidebar.OnClear = func() { a.editor.ClearIssues() }
	a.sidebar.OnAccept = func(id int64, replacement string) {
		a.editor.ApplyIssueFix(id, replacement)
	}
	a.sidebar.OnReject = func(id int64) { a.editor.DismissIssue(id) }

	a.versionsSidebar = ui.NewVersionsSidebar()
	a.versionsSidebar.OnRefresh = a.refreshVersionsSidebar
	a.versionsSidebar.OnSelect = a.previewVersion
	a.versionsSidebar.OnRestore = a.restoreVersion

	commentsPlaceholder := widget.NewLabel("Comments arrive in M5 Phase B.")
	commentsPlaceholder.Importance = widget.LowImportance

	a.sidebarTabs = container.NewAppTabs(
		container.NewTabItem("Issues", a.sidebar),
		container.NewTabItem("Comments", commentsPlaceholder),
		container.NewTabItem("Versions", a.versionsSidebar),
	)
	a.sidebarTabs.SetTabLocation(container.TabLocationTop)

	a.titleLabel = widget.NewLabel("Untitled")
	a.titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	a.statusLabel = widget.NewLabel("")
	a.refreshStatus()

	top := container.NewBorder(nil, nil, a.titleLabel, nil)
	bottom := container.NewBorder(nil, nil, a.statusLabel, nil)
	scroll := container.NewVScroll(a.editor)
	split := container.NewHSplit(scroll, a.sidebarTabs)
	split.SetOffset(0.74)
	return container.NewBorder(top, bottom, nil, nil, split)
}

func (a *App) refreshStatus() {
	if a.statusLabel == nil {
		return
	}
	a.statusLabel.SetText(fmt.Sprintf("Provider: %s  ·  Model: %s",
		a.registry.ActiveName(), a.modelFor(a.registry.ActiveName())))
}

func (a *App) buildMenu() *fyne.MainMenu {
	newItem := fyne.NewMenuItem("New", a.fileNew)
	newItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierShortcutDefault}
	openItem := fyne.NewMenuItem("Open...", a.fileOpen)
	openItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierShortcutDefault}
	saveItem := fyne.NewMenuItem("Save", a.fileSave)
	saveItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierShortcutDefault}
	saveAsItem := fyne.NewMenuItem("Save As...", a.fileSaveAs)
	saveAsItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}

	paletteItem := fyne.NewMenuItem("Command Palette...", a.openCommandPalette)
	paletteItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyK, Modifier: fyne.KeyModifierShortcutDefault}
	checkItem := fyne.NewMenuItem("Check Document", a.runDocumentCheck)
	checkItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyK, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}
	contextItem := fyne.NewMenuItem("Background Instructions...", a.editContext)
	promptsItem := fyne.NewMenuItem("Prompts Library...", a.openPromptsLibrary)
	settingsItem := fyne.NewMenuItem("Settings...", a.openSettings)
	settingsItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyComma, Modifier: fyne.KeyModifierShortcutDefault}

	return fyne.NewMainMenu(
		fyne.NewMenu("File", newItem, openItem, saveItem, saveAsItem),
		fyne.NewMenu("AI", paletteItem, checkItem, contextItem, promptsItem, fyne.NewMenuItemSeparator(), settingsItem),
	)
}

func (a *App) onDocChanged() {
	if !a.dirty {
		a.dirty = true
		a.refreshTitle()
	}
}

func (a *App) refreshTitle() {
	name := "Untitled"
	if a.currentPath != "" {
		name = filepath.Base(a.currentPath)
	}
	if a.dirty {
		name = name + " •"
	}
	a.titleLabel.SetText(name)
}

func (a *App) fileNew() {
	a.editor.SetDocument(doc.New())
	a.currentPath = ""
	a.dirty = false
	a.refreshTitle()
	a.refreshVersionsSidebar()
}

func (a *App) fileOpen() {
	d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil || rc == nil {
			return
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		a.editor.SetDocument(doc.ParseMarkdown(string(data)))
		a.currentPath = rc.URI().Path()
		a.dirty = false
		a.refreshTitle()
		a.refreshVersionsSidebar()
	}, a.window)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".md", ".markdown", ".txt"}))
	d.Show()
}

func (a *App) fileSave() {
	if a.currentPath == "" {
		a.fileSaveAs()
		return
	}
	if err := a.writeCurrent(a.currentPath); err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	a.dirty = false
	a.refreshTitle()
	a.snapshotCurrent()
}

func (a *App) fileSaveAs() {
	d := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
		if err != nil || wc == nil {
			return
		}
		defer wc.Close()
		md := doc.WriteMarkdown(a.editor.Document())
		if _, err := wc.Write([]byte(md)); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		a.currentPath = wc.URI().Path()
		a.dirty = false
		a.refreshTitle()
		a.snapshotCurrent()
	}, a.window)
	d.SetFileName("Untitled.md")
	d.Show()
}

func (a *App) writeCurrent(path string) error {
	md := doc.WriteMarkdown(a.editor.Document())
	uri := storage.NewFileURI(path)
	wc, err := storage.Writer(uri)
	if err != nil {
		return err
	}
	defer wc.Close()
	_, err = wc.Write([]byte(md))
	return err
}

// CheckAI is invoked by the --check-ai CLI flag.
func (a *App) CheckAI(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := a.registry.Active().Generate(ctx, ai.Request{
		Messages:  []ai.Message{{Role: ai.RoleUser, Content: "Reply with exactly: ok"}},
		MaxTokens: 16,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("provider=%s model=%s reply=%q", a.registry.ActiveName(), resp.Model, resp.Text), nil
}
