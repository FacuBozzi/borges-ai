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

func TestRoundTripMarks(t *testing.T) {
	cases := []string{
		"This has **bold** in it\n",
		"This has *italic* in it\n",
		"This has `code` in it\n",
		"This has ~~strike~~ in it\n",
		"Mix **bold** and *italic* in one line\n",
	}
	for _, in := range cases {
		d := ParseMarkdown(in)
		out := WriteMarkdown(d)
		if out != in {
			t.Errorf("round-trip mismatch:\nin:  %q\nout: %q", in, out)
		}
	}
}

func TestParseExtractsMarks(t *testing.T) {
	d := ParseMarkdown("a **bold** word\n")
	b := d.Blocks[0]
	if len(b.Inlines) != 3 {
		t.Fatalf("expected 3 inlines (a /bold/ word), got %d: %+v", len(b.Inlines), b.Inlines)
	}
	if b.Inlines[1].Text != "bold" || !b.Inlines[1].Marks.Has(MarkBold) {
		t.Errorf("middle inline should be bold 'bold', got %+v", b.Inlines[1])
	}
}

func TestParseUnderlineEmitsViaWriter(t *testing.T) {
	// Verify we can emit underline correctly even though we don't currently
	// parse <u>...</u> (the reader leaves it as literal text). The writer
	// path is what matters for shortcuts → save.
	doc := &Document{Blocks: []Block{{
		Type: BlockParagraph,
		Inlines: []Inline{
			{Text: "before "},
			{Text: "under", Marks: MarkUnderline},
			{Text: " after"},
		},
	}}}
	got := WriteMarkdown(doc)
	want := "before <u>under</u> after\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
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
