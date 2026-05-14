package doc

import "testing"

func TestRoundTripParagraphs(t *testing.T) {
	cases := []string{
		"hello\n",
		"hello world\n",
		"first paragraph\n\nsecond paragraph\n",
		"one\n\ntwo\n\nthree\n",
	}
	for _, in := range cases {
		d := ParseMarkdown(in)
		out := WriteMarkdown(d)
		if out != in {
			t.Errorf("round-trip mismatch:\nin:  %q\nout: %q", in, out)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	d := ParseMarkdown("")
	if len(d.Blocks) != 1 {
		t.Fatalf("empty doc should have 1 placeholder paragraph, got %d", len(d.Blocks))
	}
	if d.Blocks[0].Type != BlockParagraph {
		t.Errorf("placeholder should be paragraph, got %v", d.Blocks[0].Type)
	}
	if d.Blocks[0].PlainText() != "" {
		t.Errorf("placeholder should be empty, got %q", d.Blocks[0].PlainText())
	}
}

func TestCloneIsDeep(t *testing.T) {
	d := ParseMarkdown("hello\n\nworld\n")
	c := d.Clone()
	c.Blocks[0].Inlines[0].Text = "changed"
	if d.Blocks[0].Inlines[0].Text != "hello" {
		t.Errorf("clone shared inline storage: original mutated to %q", d.Blocks[0].Inlines[0].Text)
	}
}

func TestPositionBlockAt(t *testing.T) {
	d := ParseMarkdown("one\n\ntwo\n")
	b := d.BlockAt(Position{Path: []int{1}})
	if b == nil {
		t.Fatal("BlockAt returned nil")
	}
	if b.PlainText() != "two" {
		t.Errorf("expected 'two', got %q", b.PlainText())
	}
	if d.BlockAt(Position{Path: []int{5}}) != nil {
		t.Error("out-of-range path should return nil")
	}
}
