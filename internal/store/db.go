package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB   *sql.DB
	Path string
}

// Open opens (creating if needed) the app's SQLite database in the user's
// app-data directory and runs all migrations.
func Open() (*Store, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "fyne-writer.db")
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{DB: db, Path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func dataDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "fyne-writer"), nil
}

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY
	)`,
	`CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS versions (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		doc_path      TEXT NOT NULL,
		snapshot_json TEXT NOT NULL,
		content_hash  TEXT NOT NULL,
		created_at    INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_versions_doc ON versions(doc_path, created_at DESC)`,
	`CREATE TABLE IF NOT EXISTS comments (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		doc_path          TEXT NOT NULL,
		anchor_text       TEXT NOT NULL,
		block_index       INTEGER NOT NULL DEFAULT 0,
		range_start_hint  INTEGER NOT NULL,
		range_end_hint    INTEGER NOT NULL,
		body              TEXT NOT NULL,
		resolved          INTEGER NOT NULL DEFAULT 0,
		created_at        INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_comments_doc ON comments(doc_path, created_at)`,
	`CREATE TABLE IF NOT EXISTS prompts (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		name                TEXT NOT NULL,
		description         TEXT NOT NULL DEFAULT '',
		template            TEXT NOT NULL,
		hotkey              TEXT NOT NULL DEFAULT '',
		requires_selection  INTEGER NOT NULL DEFAULT 1,
		created_at          INTEGER NOT NULL
	)`,
}

func (s *Store) migrate() error {
	for i, stmt := range migrations {
		if _, err := s.DB.Exec(stmt); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	// Idempotent column adds for dev DBs that predate later milestones.
	// SQLite has no "ADD COLUMN IF NOT EXISTS", so we check first.
	if err := s.ensureColumn("comments", "block_index", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("ensure comments.block_index: %w", err)
	}
	return nil
}

// ensureColumn adds the column if missing. SQLite's PRAGMA table_info lists
// existing columns; we skip ALTER TABLE when the column is already there.
func (s *Store) ensureColumn(table, column, typeAndConstraints string) error {
	rows, err := s.DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, typeAndConstraints))
	return err
}
