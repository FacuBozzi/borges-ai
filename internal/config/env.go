package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	AnthropicAPIKey string
	OpenAIAPIKey    string
	AnthropicModel  string
	OpenAIModel     string
}

func (c Config) HasAnthropic() bool { return c.AnthropicAPIKey != "" }
func (c Config) HasOpenAI() bool    { return c.OpenAIAPIKey != "" }
func (c Config) HasAny() bool       { return c.HasAnthropic() || c.HasOpenAI() }

// Load reads .env from the current working directory (if present) and the
// user's project root, then returns a Config populated from the environment.
// Missing .env is not an error; the user may also set keys via real env vars.
func Load() Config {
	for _, p := range candidateEnvPaths() {
		_ = godotenv.Load(p)
	}
	return Config{
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		AnthropicModel:  envOr("ANTHROPIC_MODEL", "claude-opus-4-7"),
		OpenAIModel:     envOr("OPENAI_MODEL", "gpt-5"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func candidateEnvPaths() []string {
	paths := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "fyne-writer", ".env"))
	}
	return paths
}
