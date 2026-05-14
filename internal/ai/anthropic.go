package ai

import (
	"context"
	"errors"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

const ProviderAnthropic = "anthropic"

type AnthropicProvider struct {
	client       anthropic.Client
	defaultModel string
}

func NewAnthropic(apiKey, defaultModel string) *AnthropicProvider {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{client: c, defaultModel: defaultModel}
}

func (p *AnthropicProvider) Name() string { return ProviderAnthropic }

func (p *AnthropicProvider) Models() []string {
	return []string{
		"claude-opus-4-7",
		"claude-sonnet-4-6",
		"claude-haiku-4-5",
	}
}

func (p *AnthropicProvider) build(req Request) anthropic.MessageNewParams {
	msgs := make([]anthropic.MessageParam, 0, len(req.Messages))
	for _, m := range req.Messages {
		block := anthropic.NewTextBlock(m.Content)
		switch m.Role {
		case RoleAssistant:
			msgs = append(msgs, anthropic.NewAssistantMessage(block))
		default:
			msgs = append(msgs, anthropic.NewUserMessage(block))
		}
	}
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}
	maxTok := int64(req.MaxTokens)
	if maxTok == 0 {
		maxTok = 2048
	}
	params := anthropic.MessageNewParams{
		MaxTokens: maxTok,
		Model:     anthropic.Model(model),
		Messages:  msgs,
	}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}
	if req.Temperature > 0 {
		params.Temperature = param.NewOpt(req.Temperature)
	}
	return params
}

func (p *AnthropicProvider) Generate(ctx context.Context, req Request) (Response, error) {
	params := p.build(req)
	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return Response{}, err
	}
	var text string
	for _, b := range msg.Content {
		if b.Text != "" {
			text += b.Text
		}
	}
	return Response{Text: text, Model: string(msg.Model)}, nil
}

func (p *AnthropicProvider) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	params := p.build(req)
	stream := p.client.Messages.NewStreaming(ctx, params)
	if stream == nil {
		return nil, errors.New("anthropic: stream init failed")
	}
	out := make(chan Chunk, 16)
	go func() {
		defer close(out)
		defer stream.Close()
		for stream.Next() {
			ev := stream.Current()
			if delta, ok := ev.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
				if td, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok && td.Text != "" {
					select {
					case out <- Chunk{Delta: td.Text}:
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
