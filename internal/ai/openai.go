package ai

import (
	"context"
	"errors"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
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
	}
	if req.MaxTokens > 0 {
		params.MaxCompletionTokens = param.NewOpt(int64(req.MaxTokens))
	}
	if req.Temperature > 0 {
		params.Temperature = param.NewOpt(req.Temperature)
	}
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
