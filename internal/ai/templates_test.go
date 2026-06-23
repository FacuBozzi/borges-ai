package ai

import (
	"strings"
	"testing"
)

func TestBuildRequestAskAI(t *testing.T) {
	in := PromptInputs{
		Selection:   "The quick brown fox.",
		Document:    "Once upon a time. The quick brown fox.",
		Instruction: "finish this in the same style",
	}
	req := BuildRequest(CmdAskAI, in, "test-model")

	if strings.TrimSpace(req.System) == "" {
		t.Fatal("expected non-empty system prompt")
	}
	if req.JSON {
		t.Error("ask_ai must not request JSON output")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req.Messages))
	}
	user := req.Messages[0].Content
	if !strings.Contains(user, in.Instruction) {
		t.Errorf("user prompt missing instruction: %q", user)
	}
	if !strings.Contains(user, in.Selection) {
		t.Errorf("user prompt missing selection: %q", user)
	}
	if !strings.Contains(user, in.Document) {
		t.Errorf("user prompt missing document context: %q", user)
	}
}
