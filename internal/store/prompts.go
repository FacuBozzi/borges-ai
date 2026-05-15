package store

import "time"

// Prompt is a user-defined AI command stored in the prompts table.
// Template is rendered with text/template against ai.PromptInputs at run time.
// Hotkey is a parsed shortcut spec like "Cmd+Shift+P" (empty means no hotkey).
// RequiresSelection guards palette enablement and right-click visibility.
type Prompt struct {
	ID                int64
	Name              string
	Description       string
	Template          string
	Hotkey            string
	RequiresSelection bool
	CreatedAt         time.Time
}

// ListPrompts returns all prompts in creation order.
func (s *Store) ListPrompts() ([]Prompt, error) {
	rows, err := s.DB.Query(`SELECT id, name, description, template, hotkey, requires_selection, created_at
		FROM prompts ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Prompt
	for rows.Next() {
		var p Prompt
		var rs int
		var created int64
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Template, &p.Hotkey, &rs, &created); err != nil {
			return nil, err
		}
		p.RequiresSelection = rs != 0
		p.CreatedAt = time.Unix(created, 0)
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreatePrompt inserts a new row and returns its assigned ID.
func (s *Store) CreatePrompt(p Prompt) (int64, error) {
	rs := 0
	if p.RequiresSelection {
		rs = 1
	}
	res, err := s.DB.Exec(`INSERT INTO prompts(name, description, template, hotkey, requires_selection, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		p.Name, p.Description, p.Template, p.Hotkey, rs, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdatePrompt overwrites all editable fields of an existing prompt.
func (s *Store) UpdatePrompt(p Prompt) error {
	rs := 0
	if p.RequiresSelection {
		rs = 1
	}
	_, err := s.DB.Exec(`UPDATE prompts SET name = ?, description = ?, template = ?, hotkey = ?, requires_selection = ?
		WHERE id = ?`,
		p.Name, p.Description, p.Template, p.Hotkey, rs, p.ID)
	return err
}

// DeletePrompt removes a prompt by ID.
func (s *Store) DeletePrompt(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM prompts WHERE id = ?`, id)
	return err
}
