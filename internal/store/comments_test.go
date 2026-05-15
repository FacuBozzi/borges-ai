package store

import "testing"

func TestInsertAndListComments(t *testing.T) {
	s := openTestStore(t)

	id1, err := s.InsertComment(Comment{
		DocPath:        "/tmp/a.md",
		AnchorText:     "hello",
		RangeStartHint: 0,
		RangeEndHint:   5,
		Body:           "first",
	})
	if err != nil || id1 == 0 {
		t.Fatalf("insert: id=%d err=%v", id1, err)
	}
	id2, err := s.InsertComment(Comment{
		DocPath:        "/tmp/a.md",
		AnchorText:     "world",
		RangeStartHint: 6,
		RangeEndHint:   11,
		Body:           "second",
	})
	if err != nil || id2 == 0 {
		t.Fatalf("insert 2: %v", err)
	}

	rows, err := s.ListComments("/tmp/a.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Body != "first" || rows[1].Body != "second" {
		t.Fatalf("order wrong: %v %v", rows[0].Body, rows[1].Body)
	}
}

func TestListComments_FiltersResolved(t *testing.T) {
	s := openTestStore(t)

	id, _ := s.InsertComment(Comment{DocPath: "/tmp/a.md", AnchorText: "x", Body: "open"})
	resolvedID, _ := s.InsertComment(Comment{DocPath: "/tmp/a.md", AnchorText: "y", Body: "done"})
	if err := s.SetCommentResolved(resolvedID, true); err != nil {
		t.Fatal(err)
	}

	open, _ := s.ListComments("/tmp/a.md", false)
	if len(open) != 1 || open[0].ID != id {
		t.Fatalf("expected only the unresolved row, got %+v", open)
	}

	all, _ := s.ListComments("/tmp/a.md", true)
	if len(all) != 2 {
		t.Fatalf("expected both rows, got %d", len(all))
	}
}

func TestListComments_ScopedPerDoc(t *testing.T) {
	s := openTestStore(t)

	_, _ = s.InsertComment(Comment{DocPath: "/tmp/a.md", Body: "a"})
	_, _ = s.InsertComment(Comment{DocPath: "/tmp/b.md", Body: "b"})

	a, _ := s.ListComments("/tmp/a.md", false)
	b, _ := s.ListComments("/tmp/b.md", false)
	if len(a) != 1 || a[0].Body != "a" {
		t.Fatalf("doc a: %+v", a)
	}
	if len(b) != 1 || b[0].Body != "b" {
		t.Fatalf("doc b: %+v", b)
	}
}

func TestUpdateCommentBodyAndAnchorHint(t *testing.T) {
	s := openTestStore(t)

	id, _ := s.InsertComment(Comment{DocPath: "/tmp/a.md", AnchorText: "hi", BlockIndex: 0, RangeStartHint: 0, RangeEndHint: 2, Body: "draft"})
	if err := s.UpdateCommentBody(id, "final"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCommentAnchorHint(id, 3, 10, 12); err != nil {
		t.Fatal(err)
	}

	rows, _ := s.ListComments("/tmp/a.md", false)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row")
	}
	if rows[0].Body != "final" {
		t.Fatalf("body: %q", rows[0].Body)
	}
	if rows[0].BlockIndex != 3 || rows[0].RangeStartHint != 10 || rows[0].RangeEndHint != 12 {
		t.Fatalf("hint: block=%d range=%d-%d", rows[0].BlockIndex, rows[0].RangeStartHint, rows[0].RangeEndHint)
	}
}

func TestDeleteComment(t *testing.T) {
	s := openTestStore(t)

	id, _ := s.InsertComment(Comment{DocPath: "/tmp/a.md", Body: "bye"})
	if err := s.DeleteComment(id); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListComments("/tmp/a.md", true)
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}
