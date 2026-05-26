package store

import "database/sql"

// Setting keys persisted in the `settings` table. Centralized here so the
// app + dialog reference the same constants.
const (
	KeyActiveProvider = "active_provider"
	KeyAnthropicModel = "anthropic_model"
	KeyOpenAIModel    = "openai_model"
	KeyThemeVariant   = "theme_variant" // "system" | "light" | "dark"
	KeyLastOpenDir    = "last_open_dir"
	KeyOnboardingDone = "onboarding_done"
)

// GetSetting returns the stored value for key, or "" if no row exists.
func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.DB.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// SetSetting upserts a key/value pair.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.DB.Exec(
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
