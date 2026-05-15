package ai

import (
	"context"
	"encoding/json"
	"strings"
)

// Suggestion is one raw item returned by the document-check prompt. Anchor
// resolution (mapping AnchorText back to a byte range in the editor's
// document) happens outside this package; we only do the AI call + parse.
type Suggestion struct {
	AnchorText  string `json:"anchor_text"`
	Type        string `json:"type"`     // "grammar" | "clarity" | "style" | "tone"
	Severity    string `json:"severity"` // "low" | "med" | "high"
	Message     string `json:"message"`
	Replacement string `json:"replacement"`
}

// CheckDocument runs the document-wide grammar/style/clarity prompt against
// the active provider and returns parsed suggestions. The document is
// truncated to ~6k chars to bound token cost; chunking will be added if
// long-document UX warrants it.
func CheckDocument(ctx context.Context, p Provider, model, document, context_ string) ([]Suggestion, error) {
	req := buildCheckRequest(model, document, context_)
	resp, err := p.Generate(ctx, req)
	if err != nil {
		return nil, err
	}
	return parseSuggestions(resp.Text), nil
}

func buildCheckRequest(model, document, context_ string) Request {
	body := trim(document, 6000)
	system := `You are an expert copy-editor. Review the user's document and return a JSON object listing concrete problems.

Output schema:
{"issues":[{"anchor_text":"<EXACT substring from the document, 6-80 chars, must be unique>","type":"grammar|clarity|style|tone","severity":"low|med|high","message":"<one sentence>","replacement":"<suggested fix, or empty if no concrete fix>"}]}

Rules:
- anchor_text must appear EXACTLY ONCE in the document and be at least 6 characters long. Include surrounding words if the phrase alone is not unique.
- Limit to at most 12 issues. Prefer high-severity items.
- If there are no problems, return {"issues":[]}.
- Respond with JSON only — no preface, no fences.`
	if context_ = strings.TrimSpace(context_); context_ != "" {
		system += "\n\nAdditional document-level guidance from the user:\n" + context_
	}
	return Request{
		System:      system,
		Messages:    []Message{{Role: RoleUser, Content: "Document:\n---\n" + body + "\n---"}},
		Model:       model,
		MaxTokens:   2048,
		Temperature: 0.2,
		JSON:        true,
	}
}

// parseSuggestions accepts either {"issues":[...]} or a bare JSON array.
// Suggestions with empty anchors are dropped silently.
func parseSuggestions(text string) []Suggestion {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	type wrap struct {
		Issues []Suggestion `json:"issues"`
	}
	var w wrap
	if err := json.Unmarshal([]byte(text), &w); err == nil && len(w.Issues) > 0 {
		return filterSuggestions(w.Issues)
	}
	var arr []Suggestion
	if err := json.Unmarshal([]byte(text), &arr); err == nil && len(arr) > 0 {
		return filterSuggestions(arr)
	}
	return nil
}

func filterSuggestions(in []Suggestion) []Suggestion {
	out := in[:0]
	for _, s := range in {
		s.AnchorText = strings.TrimSpace(s.AnchorText)
		if s.AnchorText == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
