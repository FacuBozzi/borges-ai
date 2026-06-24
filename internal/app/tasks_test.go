package app

import (
	"reflect"
	"testing"

	"github.com/facubozzi/fyne-writer/internal/ui"
)

// labels extracts the labels from the last snapshot pushed to onChange.
func labels(views []ui.TaskView) []string {
	out := make([]string, len(views))
	for i, v := range views {
		out[i] = v.Label
	}
	return out
}

func TestTaskManager_StartFinish(t *testing.T) {
	var last []ui.TaskView
	m := &taskManager{onChange: func(v []ui.TaskView) { last = v }}

	id1 := m.start("Paraphrase", func() {})
	id2 := m.start("Ask AI", func() {})
	if id1 == id2 {
		t.Fatalf("ids must be unique, got %d twice", id1)
	}
	if got := labels(last); !reflect.DeepEqual(got, []string{"Paraphrase", "Ask AI"}) {
		t.Fatalf("after two starts, snapshot = %v", got)
	}

	m.finish(id1)
	if got := labels(last); !reflect.DeepEqual(got, []string{"Ask AI"}) {
		t.Fatalf("after finishing id1, snapshot = %v", got)
	}

	m.finish(id2)
	if got := labels(last); len(got) != 0 {
		t.Fatalf("after finishing all, snapshot = %v (want empty)", got)
	}
}

func TestTaskManager_FinishUnknownIsNoop(t *testing.T) {
	notified := 0
	m := &taskManager{onChange: func([]ui.TaskView) { notified++ }}

	id := m.start("Synonyms", func() {})
	startNotifs := notified

	// Simulate a user cancel that already removed the row, followed by the
	// goroutine's terminal finish() for the same id: it must be a no-op and
	// must not push a spurious snapshot.
	m.cancel(id)
	cancelNotifs := notified
	m.finish(id)
	if notified != cancelNotifs {
		t.Fatalf("finish() on an already-removed id pushed a snapshot (notified %d→%d)", cancelNotifs, notified)
	}
	if notified <= startNotifs {
		t.Fatalf("cancel() should have notified at least once")
	}
}

func TestTaskManager_CancelInvokesCancelFunc(t *testing.T) {
	m := &taskManager{onChange: func([]ui.TaskView) {}}
	cancelled := false
	id := m.start("Check", func() { cancelled = true })

	m.cancel(id)
	if !cancelled {
		t.Fatal("cancel() did not invoke the task's CancelFunc")
	}
	// Cancelling again is a harmless no-op.
	m.cancel(id)
}
