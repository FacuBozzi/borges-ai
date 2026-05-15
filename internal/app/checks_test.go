package app

import (
	"testing"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

func TestFindUniqueAnchor(t *testing.T) {
	cases := []struct {
		name      string
		blocks    []string
		anchor    string
		wantBlock int
		wantOff   int
		wantOK    bool
	}{
		{
			name:      "single occurrence in one block",
			blocks:    []string{"hello world", "another line"},
			anchor:    "world",
			wantBlock: 0,
			wantOff:   6,
			wantOK:    true,
		},
		{
			name:      "found in second block",
			blocks:    []string{"foo", "bar baz qux"},
			anchor:    "baz",
			wantBlock: 1,
			wantOff:   4,
			wantOK:    true,
		},
		{
			name:   "duplicate in same block rejected",
			blocks: []string{"hello hello"},
			anchor: "hello",
			wantOK: false,
		},
		{
			name:   "duplicate across blocks rejected",
			blocks: []string{"alpha", "alpha"},
			anchor: "alpha",
			wantOK: false,
		},
		{
			name:   "not found",
			blocks: []string{"alpha"},
			anchor: "beta",
			wantOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := doc.New()
			d.Blocks = d.Blocks[:0]
			for _, text := range c.blocks {
				d.Blocks = append(d.Blocks, doc.Block{
					Type:    doc.BlockParagraph,
					Inlines: []doc.Inline{{Text: text}},
				})
			}
			gotBlock, gotOff, gotOK := findUniqueAnchor(d, c.anchor)
			if gotOK != c.wantOK {
				t.Fatalf("ok got %v want %v", gotOK, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if gotBlock != c.wantBlock {
				t.Errorf("block got %d want %d", gotBlock, c.wantBlock)
			}
			if gotOff != c.wantOff {
				t.Errorf("offset got %d want %d", gotOff, c.wantOff)
			}
		})
	}
}
