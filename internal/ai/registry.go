package ai

import "github.com/facubozzi/fyne-writer/internal/config"

// Registry holds the configured providers. At least one is always available
// (the mock provider falls back when no API keys are present).
type Registry struct {
	providers map[string]Provider
	active    string
}

// NewRegistry builds the registry from environment-derived config.
// Preferred provider order when both keys are set: Anthropic, then OpenAI.
func NewRegistry(cfg config.Config) *Registry {
	r := &Registry{providers: map[string]Provider{}}
	if cfg.HasAnthropic() {
		r.providers[ProviderAnthropic] = NewAnthropic(cfg.AnthropicAPIKey, cfg.AnthropicModel)
		r.active = ProviderAnthropic
	}
	if cfg.HasOpenAI() {
		r.providers[ProviderOpenAI] = NewOpenAI(cfg.OpenAIAPIKey, cfg.OpenAIModel)
		if r.active == "" {
			r.active = ProviderOpenAI
		}
	}
	if r.active == "" {
		r.providers[ProviderMock] = NewMock()
		r.active = ProviderMock
	}
	return r
}

func (r *Registry) Active() Provider           { return r.providers[r.active] }
func (r *Registry) ActiveName() string         { return r.active }
func (r *Registry) Available() []string {
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	return names
}

// SetActive selects a provider by name. Returns false if unknown.
func (r *Registry) SetActive(name string) bool {
	if _, ok := r.providers[name]; !ok {
		return false
	}
	r.active = name
	return true
}
