package doc

import "testing"

func plainBlock(s string) Block {
	return Block{Type: BlockParagraph, Inlines: []Inline{{Text: s}}}
}

func mixedBlock(parts ...Inline) Block {
	return Block{Type: BlockParagraph, Inlines: parts}
}

func TestInlineAt(t *testing.T) {
	b := mixedBlock(
		Inline{Text: "ab"},               // bytes 0..2
		Inline{Text: "cd", Marks: MarkBold}, // bytes 2..4
		Inline{Text: "ef"},               // bytes 4..6
	)
	cases := []struct {
		off       int
		wantI     int
		wantLocal int
	}{
		{0, 0, 0}, {1, 0, 1}, {2, 0, 2}, // end-of-first preferred over start-of-second
		{3, 1, 1}, {4, 1, 2},
		{5, 2, 1}, {6, 2, 2},
	}
	for _, c := range cases {
		i, lo := InlineAt(b, c.off)
		if i != c.wantI || lo != c.wantLocal {
			t.Errorf("InlineAt(%d) = (%d,%d) want (%d,%d)", c.off, i, lo, c.wantI, c.wantLocal)
		}
	}
}

func TestInsertText_ExtendsHostInline(t *testing.T) {
	b := plainBlock("hello")
	b = InsertText(b, 5, " world", 0)
	if len(b.Inlines) != 1 || b.Inlines[0].Text != "hello world" {
		t.Errorf("got %+v", b.Inlines)
	}
}

func TestInsertText_SplitsForDifferentMarks(t *testing.T) {
	b := plainBlock("hello world")
	b = InsertText(b, 5, " BOLD", MarkBold)
	if len(b.Inlines) != 3 {
		t.Fatalf("want 3 inlines, got %d: %+v", len(b.Inlines), b.Inlines)
	}
	if b.Inlines[0].Text != "hello" || b.Inlines[1].Text != " BOLD" || b.Inlines[2].Text != " world" {
		t.Errorf("split wrong: %+v", b.Inlines)
	}
	if b.Inlines[1].Marks != MarkBold || b.Inlines[0].Marks != 0 {
		t.Errorf("marks wrong: %+v", b.Inlines)
	}
}

func TestInsertText_ExtendsAdjacentMatchingInline(t *testing.T) {
	b := mixedBlock(
		Inline{Text: "abc"},
		Inline{Text: "DEF", Marks: MarkBold},
	)
	// Inserting bold text right at the boundary should extend the bold inline.
	b = InsertText(b, 3, "X", MarkBold)
	if len(b.Inlines) != 2 || b.Inlines[1].Text != "XDEF" {
		t.Errorf("extend-right failed: %+v", b.Inlines)
	}
}

func TestDeleteRange_WithinInline(t *testing.T) {
	b := plainBlock("hello world")
	b = DeleteRange(b, 5, 11)
	if b.PlainText() != "hello" {
		t.Errorf("got %q", b.PlainText())
	}
}

func TestDeleteRange_AcrossInlines(t *testing.T) {
	b := mixedBlock(
		Inline{Text: "alpha"},
		Inline{Text: "BETA", Marks: MarkBold},
		Inline{Text: "gamma"},
	)
	b = DeleteRange(b, 4, 10) // remove "aBETAg" leaving "alphamma" wait recompute
	// plain = "alphaBETAgamma" (14 bytes)
	// remove [4, 10) = "aBETAg" → leaves "alph" + "amma" = "alphamma"
	if b.PlainText() != "alphamma" {
		t.Errorf("got %q", b.PlainText())
	}
}

func TestApplyMark_SplitsAndApplies(t *testing.T) {
	b := plainBlock("hello world")
	b = ApplyMark(b, 0, 5, MarkBold, true)
	if len(b.Inlines) != 2 || b.Inlines[0].Text != "hello" || !b.Inlines[0].Marks.Has(MarkBold) {
		t.Errorf("apply failed: %+v", b.Inlines)
	}
	if b.Inlines[1].Marks != 0 {
		t.Errorf("second inline should be unmarked: %+v", b.Inlines[1])
	}
}

func TestApplyMark_ClearMerges(t *testing.T) {
	b := mixedBlock(
		Inline{Text: "hello", Marks: MarkBold},
		Inline{Text: " world"},
	)
	b = ApplyMark(b, 0, 5, MarkBold, false)
	if len(b.Inlines) != 1 || b.Inlines[0].Text != "hello world" || b.Inlines[0].Marks != 0 {
		t.Errorf("clear+merge failed: %+v", b.Inlines)
	}
}

func TestSplitBlock_PreservesMarks(t *testing.T) {
	b := mixedBlock(
		Inline{Text: "hello"},
		Inline{Text: " WORLD", Marks: MarkBold},
	)
	l, r := SplitBlock(b, 7) // split inside " WORLD" after "hello W"
	if l.PlainText() != "hello W" || r.PlainText() != "ORLD" {
		t.Errorf("split halves wrong: %q / %q", l.PlainText(), r.PlainText())
	}
	if !r.Inlines[0].Marks.Has(MarkBold) {
		t.Errorf("right half should keep bold mark: %+v", r.Inlines)
	}
}
