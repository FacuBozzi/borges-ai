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

	editor       *editor.RichEditor
	titleLabel   *widget.Label
	currentPath  string // empty until first save
	dirty        bool
}

func New() (*App, error) {
	cfg := config.Load()
	st, err := store.Open()
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	reg := ai.NewRegistry(cfg)

	fa := fyneapp.NewWithID("dev.facubozzi.fynewriter")
	fa.Settings().SetTheme(ui.NewTheme())

	a := &App{fyne: fa, cfg: cfg, store: st, registry: reg}
	a.window = fa.NewWindow("Fyne Writer")
	a.window.Resize(fyne.NewSize(1100, 720))
	a.window.SetContent(a.buildContent())
	a.window.SetMainMenu(a.buildMenu())
	a.window.SetOnClosed(func() { _ = st.Close() })
	a.window.Canvas().Focus(a.editor)
	return a, nil
}

func (a *App) Run()                 { a.window.ShowAndRun() }
func (a *App) Registry() *ai.Registry { return a.registry }

func (a *App) buildContent() fyne.CanvasObject {
	a.editor = editor.New(doc.New())
	a.editor.OnChanged(a.onDocChanged)

	a.titleLabel = widget.NewLabel("Untitled")
	a.titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	provider := a.registry.ActiveName()
	status := widget.NewLabel(fmt.Sprintf("Provider: %s", provider))

	top := container.NewBorder(nil, nil, a.titleLabel, nil)
	bottom := container.NewBorder(nil, nil, status, nil)
	scroll := container.NewVScroll(a.editor)
	return container.NewBorder(top, bottom, nil, nil, scroll)
}

func (a *App) buildMenu() *fyne.MainMenu {
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New", a.fileNew),
		fyne.NewMenuItem("Open...", a.fileOpen),
		fyne.NewMenuItem("Save", a.fileSave),
		fyne.NewMenuItem("Save As...", a.fileSaveAs),
	)
	return fyne.NewMainMenu(fileMenu)
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
