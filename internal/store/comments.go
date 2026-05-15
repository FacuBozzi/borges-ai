package store

import "time"

// Comment is a stored user comment anchored to a byte range in a document.
// The schema lives in db.go (created at migration time). BlockIndex +
// RangeStartHint/RangeEndHint together pinpoint the original anchor; the
// resolver still falls back to anchor-text substring search if the hints
// drift after edits.
type Comment struct {
	ID             int64
	DocPath        string
	AnchorText     string
	BlockIndex     int
	RangeStartHint int
	RangeEndHint   int
	Body           string
	Resolved       bool
	CreatedAt      time.Time
}

// InsertComment writes a new comment and returns its assigned ID.
func (s *Store) InsertComment(c Comment) (int64, error) {
	resolved := 0
	if c.Resolved {
		resolved = 1
	}
	res, err := s.DB.Exec(
		`INSERT INTO comments(doc_path, anchor_text, block_index, range_start_hint, range_end_hint, body, resolved, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.DocPath, c.AnchorText, c.BlockIndex, c.RangeStartHint, c.RangeEndHint, c.Body, resolved, time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListComments returns comments for docPath in creation order (oldest first).
// When includeResolved is false, resolved rows are omitted.
func (s *Store) ListComments(docPath string, includeResolved bool) ([]Comment, error) {
	q := `SELECT id, doc_path, anchor_text, block_index, range_start_hint, range_end_hint, body, resolved, created_at
	      FROM comments WHERE doc_path = ?`
	if !includeResolved {
		q += ` AND resolved = 0`
	}
	q += ` ORDER BY created_at ASC, id ASC`
	rows, err := s.DB.Query(q, docPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		var c Comment
		var resolved int
		var created int64
		if err := rows.Scan(&c.ID, &c.DocPath, &c.AnchorText, &c.BlockIndex, &c.RangeStartHint, &c.RangeEndHint, &c.Body, &resolved, &created); err != nil {
			return nil, err
		}
		c.Resolved = resolved != 0
		c.CreatedAt = time.Unix(created, 0)
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateCommentBody overwrites the body of an existing comment.
func (s *Store) UpdateCommentBody(id int64, body string) error {
	_, err := s.DB.Exec(`UPDATE comments SET body = ? WHERE id = ?`, body, id)
	return err
}

// SetCommentResolved toggles the resolved flag.
func (s *Store) SetCommentResolved(id int64, resolved bool) error {
	v := 0
	if resolved {
		v = 1
	}
	_, err := s.DB.Exec(`UPDATE comments SET resolved = ? WHERE id = ?`, v, id)
	return err
}

// UpdateCommentAnchorHint persists a corrected anchor range. Called by the
// resolver after a fallback substring search relocates the anchor.
func (s *Store) UpdateCommentAnchorHint(id int64, blockIdx, start, end int) error {
	_, err := s.DB.Exec(
		`UPDATE comments SET block_index = ?, range_start_hint = ?, range_end_hint = ? WHERE id = ?`,
		blockIdx, start, end, id,
	)
	return err
}

// DeleteComment removes a comment by ID.
func (s *Store) DeleteComment(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM comments WHERE id = ?`, id)
	return err
}
