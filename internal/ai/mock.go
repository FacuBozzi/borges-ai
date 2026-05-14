package ai

import (
	"context"
	"strings"
	"time"
)

const ProviderMock = "mock"

// MockProvider is used when no API keys are present and for tests.
// It echoes back a deterministic transformation of the last user message
// so the UI can be exercised offline.
type MockProvider struct{}

func NewMock() *MockProvider { return &MockProvider{} }

func (MockProvider) Name() string        { return ProviderMock }
func (MockProvider) Models() []string    { return []string{"mock-1"} }

func (MockProvider) reply(req Request) string {
	if len(req.Messages) == 0 {
		return "(mock) no input"
	}
	last := req.Messages[len(req.Messages)-1].Content
	return "(mock reply) " + strings.TrimSpace(last)
}

func (m MockProvider) Generate(ctx context.Context, req Request) (Response, error) {
	return Response{Text: m.reply(req), Model: "mock-1"}, nil
}

func (m MockProvider) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	out := make(chan Chunk, 4)
	go func() {
		defer close(out)
		full := m.reply(req)
		for _, w := range strings.Fields(full) {
			select {
			case out <- Chunk{Delta: w + " "}:
			case <-ctx.Done():
				return
			}
			time.Sleep(30 * time.Millisecond)
		}
		out <- Chunk{Done: true}
	}()
	return out, nil
}
