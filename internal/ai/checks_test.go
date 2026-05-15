package ai

import "testing"

func TestParseSuggestions(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"object form", `{"issues":[{"anchor_text":"foo bar","type":"grammar","severity":"high","message":"x","replacement":"y"}]}`, 1},
		{"array form", `[{"anchor_text":"foo","type":"clarity","severity":"low","message":"x","replacement":""}]`, 1},
		{"empty anchor dropped", `{"issues":[{"anchor_text":"","message":"x"}]}`, 0},
		{"fenced", "```json\n{\"issues\":[{\"anchor_text\":\"a b\",\"message\":\"m\"}]}\n```", 1},
		{"empty array", `{"issues":[]}`, 0},
		{"malformed", `not json`, 0},
		{"whitespace only", `   `, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseSuggestions(c.in)
			if len(got) != c.want {
				t.Fatalf("parseSuggestions(%q) returned %d items, want %d", c.in, len(got), c.want)
			}
		})
	}
}
