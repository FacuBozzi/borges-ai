package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// openTestStore opens an isolated SQLite store backed by a temp file and runs
// migrations. We can't reuse Open() because it picks the user's data dir.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	s := &Store{DB: db, Path: path}
	if err := s.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return s
}

func TestInsertVersion_DedupsConsecutiveIdenticalHash(t *testing.T) {
	s := openTestStore(t)

	id1, ok1, err := s.InsertVersion("/tmp/a.md", `{"x":1}`, "hash-a")
	if err != nil || !ok1 || id1 == 0 {
		t.Fatalf("first insert: id=%d ok=%v err=%v", id1, ok1, err)
	}

	id2, ok2, err := s.InsertVersion("/tmp/a.md", `{"x":1}`, "hash-a")
	if err != nil {
		t.Fatalf("dup insert err: %v", err)
	}
	if ok2 || id2 != 0 {
		t.Fatalf("expected dedup; got id=%d ok=%v", id2, ok2)
	}

	id3, ok3, err := s.InsertVersion("/tmp/a.md", `{"x":2}`, "hash-b")
	if err != nil || !ok3 || id3 == 0 {
		t.Fatalf("third insert: id=%d ok=%v err=%v", id3, ok3, err)
	}

	rows, err := s.ListVersions("/tmp/a.md")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows after dedup, got %d", len(rows))
	}
}

func TestInsertVersion_AllowsSameHashAfterDifferentSave(t *testing.T) {
	s := openTestStore(t)

	if _, _, err := s.InsertVersion("/tmp/a.md", `{"v":1}`, "h1"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.InsertVersion("/tmp/a.md", `{"v":2}`, "h2"); err != nil {
		t.Fatal(err)
	}
	_, ok, err := s.InsertVersion("/tmp/a.md", `{"v":1}`, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("h1 should be allowed after h2 broke the streak")
	}
}

func TestInsertVersion_ScopedPerDocPath(t *testing.T) {
	s := openTestStore(t)

	if _, _, err := s.InsertVersion("/tmp/a.md", `{}`, "h"); err != nil {
		t.Fatal(err)
	}
	_, ok, err := s.InsertVersion("/tmp/b.md", `{}`, "h")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("identical hash on different doc should not dedup")
	}
}

func TestListVersions_OrdersNewestFirst(t *testing.T) {
	s := openTestStore(t)

	for i, h := range []string{"h1", "h2", "h3"} {
		_, _, err := s.InsertVersion("/tmp/a.md", `{}`, h)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	rows, err := s.ListVersions("/tmp/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].ContentHash != "h3" || rows[2].ContentHash != "h1" {
		t.Fatalf("unexpected order: %v", []string{rows[0].ContentHash, rows[1].ContentHash, rows[2].ContentHash})
	}
}

func TestLoadVersion_RoundTrip(t *testing.T) {
	s := openTestStore(t)

	id, _, err := s.InsertVersion("/tmp/a.md", `{"hello":"world"}`, "h")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadVersion(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotJSON != `{"hello":"world"}` || got.DocPath != "/tmp/a.md" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestGCVersions_KeepsNewestN(t *testing.T) {
	s := openTestStore(t)

	for i := 0; i < 10; i++ {
		hash := string(rune('a' + i))
		if _, _, err := s.InsertVersion("/tmp/a.md", `{}`, hash); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.GCVersions("/tmp/a.md", 3); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListVersions("/tmp/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 surviving rows, got %d", len(rows))
	}
	if rows[0].ContentHash != "j" || rows[2].ContentHash != "h" {
		t.Fatalf("wrong rows kept: %v %v %v", rows[0].ContentHash, rows[1].ContentHash, rows[2].ContentHash)
	}
}

func TestGCVersions_ScopedPerDoc(t *testing.T) {
	s := openTestStore(t)

	for i := 0; i < 5; i++ {
		hash := string(rune('a' + i))
		if _, _, err := s.InsertVersion("/tmp/a.md", `{}`, hash); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.InsertVersion("/tmp/b.md", `{}`, hash); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.GCVersions("/tmp/a.md", 2); err != nil {
		t.Fatal(err)
	}
	a, _ := s.ListVersions("/tmp/a.md")
	b, _ := s.ListVersions("/tmp/b.md")
	if len(a) != 2 {
		t.Fatalf("a should have 2 rows, got %d", len(a))
	}
	if len(b) != 5 {
		t.Fatalf("b should still have 5 rows, got %d", len(b))
	}
}
