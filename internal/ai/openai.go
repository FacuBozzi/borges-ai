package ai

import (
	"context"
	"errors"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

const ProviderOpenAI = "openai"

type OpenAIProvider struct {
	client       openai.Client
	defaultModel string
}

func NewOpenAI(apiKey, defaultModel string) *OpenAIProvider {
	c := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAIProvider{client: c, defaultModel: defaultModel}
}

func (p *OpenAIProvider) Name() string { return ProviderOpenAI }

func (p *OpenAIProvider) Models() []string {
	return []string{
		"gpt-5",
		"gpt-5-mini",
		"gpt-4.1",
		"gpt-4o",
	}
}

func (p *OpenAIProvider) build(req Request) openai.ChatCompletionNewParams {
	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, openai.SystemMessage(req.System))
	}
	for _, m := range req.Messages {
		switch m.Role {
		case RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		default:
			msgs = append(msgs, openai.UserMessage(m.Content))
		}
	}
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}
	params := openai.ChatCompletionNewParams{
		Messages: msgs,
		Model:    shared.ChatModel(model),
		// "minimal" tells gpt-5 to skip its reasoning pass. Without this the
		// model consumes the entire token budget on invisible reasoning and
		// emits no content, leaving the editor's selection replaced with an
		// empty string. The exported constants are low/medium/high but the
		// type is a plain string so "minimal" is accepted by the wire API.
		ReasoningEffort: shared.ReasoningEffort("minimal"),
	}
	// We intentionally do NOT cap MaxCompletionTokens or set Temperature.
	// gpt-5 only accepts Temperature=1 (the default) and reasoning tokens
	// count against MaxCompletionTokens; a tight cap can leave the model
	// with no budget for visible output. Anthropic still honors both via
	// its own Request fields.
	return params
}

func (p *OpenAIProvider) Generate(ctx context.Context, req Request) (Response, error) {
	params := p.build(req)
	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return Response{}, err
	}
	if len(resp.Choices) == 0 {
		return Response{}, errors.New("openai: empty completion")
	}
	return Response{Text: resp.Choices[0].Message.Content, Model: resp.Model}, nil
}

func (p *OpenAIProvider) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	params := p.build(req)
	stream := p.client.Chat.Completions.NewStreaming(ctx, params)
	if stream == nil {
		return nil, errors.New("openai: stream init failed")
	}
	out := make(chan Chunk, 16)
	go func() {
		defer close(out)
		defer stream.Close()
		for stream.Next() {
			chunk := stream.Current()
			for _, ch := range chunk.Choices {
				if ch.Delta.Content != "" {
					select {
					case out <- Chunk{Delta: ch.Delta.Content}:
					case <-ctx.Done():
						return
					}
				}
			}
		}
		if err := stream.Err(); err != nil {
			out <- Chunk{Err: err}
			return
		}
		out <- Chunk{Done: true}
	}()
	return out, nil
}
