package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// TaskView is one row in the AI tasks overlay. The app layer translates from
// its internal task records → TaskView so this widget stays decoupled.
type TaskView struct {
	ID    int64
	Label string
}

// TasksOverlay is the small floating card that lists in-flight AI tasks. It's
// built once and the app calls SetTasks to mutate state. The card hides itself
// when there are no tasks so it stays out of the way. Anchoring it to a corner
// (and keeping it non-modal) is the caller's job — see app.buildContent.
type TasksOverlay struct {
	widget.BaseWidget

	list *fyne.Container
	card *widget.Card

	// OnCancel is invoked with a task id when its ✕ button is clicked.
	OnCancel func(int64)
}

// NewTasksOverlay builds the overlay widget, hidden until SetTasks adds rows.
func NewTasksOverlay() *TasksOverlay {
	o := &TasksOverlay{list: container.NewVBox()}
	o.card = widget.NewCard("", "", o.list)
	o.ExtendBaseWidget(o)
	o.Hide()
	return o
}

// CreateRenderer satisfies fyne.Widget.
func (o *TasksOverlay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(o.card)
}

// SetTasks replaces the displayed rows. Tiny lists, so we rebuild wholesale
// (same approach as IssuesSidebar.refresh). UI-thread only.
func (o *TasksOverlay) SetTasks(tasks []TaskView) {
	o.list.RemoveAll()
	for _, t := range tasks {
		o.list.Add(o.buildRow(t))
	}
	o.list.Refresh()
	if len(tasks) == 0 {
		o.Hide()
	} else {
		o.Show()
	}
}

func (o *TasksOverlay) buildRow(t TaskView) fyne.CanvasObject {
	spinner := widget.NewActivity()
	spinner.Start() // unlike ProgressBarInfinite, Activity needs an explicit start

	label := widget.NewLabel(t.Label)

	id := t.ID
	cancel := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		if o.OnCancel != nil {
			o.OnCancel(id)
		}
	})
	cancel.Importance = widget.LowImportance

	return container.NewBorder(nil, nil, spinner, cancel, label)
}
