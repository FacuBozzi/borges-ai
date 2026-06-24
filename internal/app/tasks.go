package app

import (
	"context"

	"github.com/facubozzi/fyne-writer/internal/ui"
)

// aiTask is one in-flight AI operation tracked by taskManager.
type aiTask struct {
	id     int64
	label  string
	cancel context.CancelFunc
}

// taskManager tracks in-flight AI operations so the status overlay can list
// them and the user can cancel them.
//
// Threading: all state here is UI-thread-confined (like App.synonymCache) —
// start() runs before its goroutine launches, finish() runs inside the
// goroutine's fyne.Do block, and cancel() fires from a UI button. The only
// cross-thread handoff is each task's context.CancelFunc, which is itself
// goroutine-safe. So no mutex is needed: a mutex would guard this int+slice
// but give no protection for the real hazard (off-thread widget mutation),
// which fyne.Do already handles.
type taskManager struct {
	nextID   int64
	tasks    []*aiTask
	onChange func([]ui.TaskView) // pushes a snapshot to the overlay
}

// start registers a task and returns its id. cancel is invoked if the user
// cancels the task from the overlay.
func (m *taskManager) start(label string, cancel context.CancelFunc) int64 {
	m.nextID++
	id := m.nextID
	m.tasks = append(m.tasks, &aiTask{id: id, label: label, cancel: cancel})
	m.notify()
	return id
}

// finish removes a completed task. No-op if the id was already removed (e.g.
// the user cancelled it while the goroutine was still draining).
func (m *taskManager) finish(id int64) { m.remove(id) }

// cancel aborts a task's underlying work and removes its row immediately so
// the overlay reflects the click without waiting for the goroutine to drain.
func (m *taskManager) cancel(id int64) {
	for _, t := range m.tasks {
		if t.id == id {
			if t.cancel != nil {
				t.cancel()
			}
			break
		}
	}
	m.remove(id)
}

func (m *taskManager) remove(id int64) {
	for i, t := range m.tasks {
		if t.id == id {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			m.notify()
			return
		}
	}
}

func (m *taskManager) notify() {
	if m.onChange == nil {
		return
	}
	views := make([]ui.TaskView, len(m.tasks))
	for i, t := range m.tasks {
		views[i] = ui.TaskView{ID: t.id, Label: t.label}
	}
	m.onChange(views)
}
