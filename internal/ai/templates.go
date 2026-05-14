package ai

import (
	"fmt"
	"strings"
)

// CommandKind identifies one of the built-in AI commands.
type CommandKind string

const (
	CmdParaphrase CommandKind = "paraphrase"
	CmdShorten    CommandKind = "shorten"
	CmdExpand     CommandKind = "expand"
	CmdFixTone    CommandKind = "fix_tone"
	CmdSynonyms   CommandKind = "synonyms"
)

// CommandSpec is the user-facing description of an AI command.
type CommandSpec struct {
	Kind        CommandKind
	Title       string // shown in the command palette
	Description string // hint text
	NeedsRange  bool   // requires a non-empty selection
}

// BuiltinCommands returns the list of always-available AI commands.
// Custom prompts (M4) extend this set at runtime.
func BuiltinCommands() []CommandSpec {
	return []CommandSpec{
		{CmdParaphrase, "Paraphrase", "Rewrite the selection in the same voice and length.", true},
		{CmdShorten, "Shorten", "Tighten the selection while preserving meaning.", true},
		{CmdExpand, "Expand", "Add detail and examples to the selection.", true},
		{CmdFixTone, "Fix tone (formal)", "Rewrite the selection in a more formal tone.", true},
	}
}

// PromptInputs is the data passed into every built-in prompt.
type PromptInputs struct {
	Selection   string // the highlighted text (or word, for synonyms)
	Sentence    string // surrounding sentence — used by synonyms
	Document    string // full document plain-text, trimmed to ~4k chars
	Context     string // per-document background instructions
}

// BuildRequest produces an AI Request for the given command + inputs.
func BuildRequest(cmd CommandKind, in PromptInputs, model string) Request {
	system := systemPrompt(cmd, in.Context)
	user := userPrompt(cmd, in)
	return Request{
		System:      system,
		Messages:    []Message{{Role: RoleUser, Content: user}},
		Model:       model,
		MaxTokens:   maxTokensFor(cmd, in.Selection),
		Temperature: temperatureFor(cmd),
		JSON:        cmd == CmdSynonyms,
	}
}

func systemPrompt(cmd CommandKind, context string) string {
	base := "You are an expert writing assistant integrated into a Markdown editor."
	switch cmd {
	case CmdParaphrase:
		base += " Rewrite the user's selected text so it expresses the same meaning in different words, preserving voice, length, and any inline markdown formatting (bold/italic/etc.). Reply with ONLY the rewritten text — no preface, no quotes."
	case CmdShorten:
		base += " Tighten the user's selected text. Cut filler, redundancy, and qualifiers; keep every load-bearing idea. Reply with ONLY the shortened text — no preface, no quotes."
	case CmdExpand:
		base += " Expand the user's selected text with relevant detail, specifics, or one concrete example. Preserve voice. Reply with ONLY the expanded text — no preface, no quotes."
	case CmdFixTone:
		base += " Rewrite the user's selected text in a polished, professional tone. Keep the original meaning. Reply with ONLY the rewritten text — no preface, no quotes."
	case CmdSynonyms:
		base += ` Given a word and the sentence it appears in, return up to 8 strong synonyms or alternates that fit naturally in that exact sentence. Match tense, pluralization, and capitalization to the original word. Respond with a JSON object: {"synonyms": ["word1", "word2", ...]}. No explanation.`
	}
	if context = strings.TrimSpace(context); context != "" {
		base += "\n\nAdditional document-level guidance from the user:\n" + context
	}
	return base
}

func userPrompt(cmd CommandKind, in PromptInputs) string {
	if cmd == CmdSynonyms {
		return fmt.Sprintf("Word: %s\nSentence: %s", in.Selection, in.Sentence)
	}
	var sb strings.Builder
	if doc := trim(in.Document, 4000); doc != "" {
		sb.WriteString("Full document for context (do not rewrite all of it — only the selection):\n---\n")
		sb.WriteString(doc)
		sb.WriteString("\n---\n\n")
	}
	sb.WriteString("Selected text to rewrite:\n")
	sb.WriteString(in.Selection)
	return sb.String()
}

func maxTokensFor(cmd CommandKind, selection string) int {
	switch cmd {
	case CmdSynonyms:
		return 256
	case CmdShorten:
		return tokensForText(selection) + 32
	case CmdExpand:
		return tokensForText(selection)*3 + 128
	default:
		return tokensForText(selection)*2 + 128
	}
}

func temperatureFor(cmd CommandKind) float64 {
	switch cmd {
	case CmdSynonyms:
		return 0.5
	case CmdShorten:
		return 0.3
	default:
		return 0.7
	}
}

func tokensForText(s string) int {
	// Rough heuristic: 1 token ≈ 4 chars. Conservative upper bound.
	n := len(s) / 3
	if n < 64 {
		return 64
	}
	return n
}

func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]..."
}
