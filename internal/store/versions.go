package store

import (
	"database/sql"
	"time"
)

// Version is a single document snapshot row. SnapshotJSON is the marshalled
// doc.Document; ContentHash is a sha256 hex digest used for dedup.
type Version struct {
	ID           int64
	DocPath      string
	SnapshotJSON string
	ContentHash  string
	CreatedAt    time.Time
}

// InsertVersion appends a snapshot for docPath. If the most recent row for
// docPath already has the same contentHash, it returns inserted=false and a
// zero ID — the snapshot is treated as a duplicate of the last save.
func (s *Store) InsertVersion(docPath, snapshotJSON, contentHash string) (id int64, inserted bool, err error) {
	var lastHash string
	err = s.DB.QueryRow(
		`SELECT content_hash FROM versions WHERE doc_path = ? ORDER BY created_at DESC, id DESC LIMIT 1`,
		docPath,
	).Scan(&lastHash)
	if err != nil && err != sql.ErrNoRows {
		return 0, false, err
	}
	if err == nil && lastHash == contentHash {
		return 0, false, nil
	}
	res, err := s.DB.Exec(
		`INSERT INTO versions(doc_path, snapshot_json, content_hash, created_at) VALUES (?, ?, ?, ?)`,
		docPath, snapshotJSON, contentHash, time.Now().Unix(),
	)
	if err != nil {
		return 0, false, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// ListVersions returns snapshots for docPath ordered newest-first.
func (s *Store) ListVersions(docPath string) ([]Version, error) {
	rows, err := s.DB.Query(
		`SELECT id, doc_path, snapshot_json, content_hash, created_at
		   FROM versions WHERE doc_path = ?
		   ORDER BY created_at DESC, id DESC`,
		docPath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		var created int64
		if err := rows.Scan(&v.ID, &v.DocPath, &v.SnapshotJSON, &v.ContentHash, &created); err != nil {
			return nil, err
		}
		v.CreatedAt = time.Unix(created, 0)
		out = append(out, v)
	}
	return out, rows.Err()
}

// LoadVersion fetches one snapshot by ID.
func (s *Store) LoadVersion(id int64) (Version, error) {
	var v Version
	var created int64
	err := s.DB.QueryRow(
		`SELECT id, doc_path, snapshot_json, content_hash, created_at FROM versions WHERE id = ?`,
		id,
	).Scan(&v.ID, &v.DocPath, &v.SnapshotJSON, &v.ContentHash, &created)
	if err != nil {
		return Version{}, err
	}
	v.CreatedAt = time.Unix(created, 0)
	return v, nil
}

// GCVersions keeps the most recent `keep` rows for docPath and deletes older
// ones. Called from the snapshot path after each successful insert.
func (s *Store) GCVersions(docPath string, keep int) error {
	if keep < 0 {
		keep = 0
	}
	_, err := s.DB.Exec(
		`DELETE FROM versions
		   WHERE doc_path = ?
		     AND id NOT IN (
		       SELECT id FROM versions WHERE doc_path = ?
		         ORDER BY created_at DESC, id DESC
		         LIMIT ?
		     )`,
		docPath, docPath, keep,
	)
	return err
}
