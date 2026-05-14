package ai

import "context"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

type Request struct {
	System      string
	Messages    []Message
	Model       string
	MaxTokens   int
	Temperature float64
	JSON        bool
}

type Response struct {
	Text  string
	Model string
}

type Chunk struct {
	Delta string
	Err   error
	Done  bool
}

type Provider interface {
	Name() string
	Models() []string
	Generate(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (<-chan Chunk, error)
}
