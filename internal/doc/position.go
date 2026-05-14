package doc

// Position is a cursor location inside a Document. For M1 paragraph-only
// documents Path is always a single-element slice [blockIdx]; M2 will use
// deeper paths for list nesting.
type Position struct {
	Path   []int // path into Document.Blocks via Children
	Inline int   // index into Block.Inlines
	Offset int   // rune offset into Inline.Text
}

// Equal reports whether two positions point to the same spot.
func (p Position) Equal(o Position) bool {
	if p.Inline != o.Inline || p.Offset != o.Offset || len(p.Path) != len(o.Path) {
		return false
	}
	for i := range p.Path {
		if p.Path[i] != o.Path[i] {
			return false
		}
	}
	return true
}

// Selection spans a range. When Anchor == Head the selection is collapsed
// (just the caret).
type Selection struct {
	Anchor, Head Position
}

func (s Selection) IsCollapsed() bool { return s.Anchor.Equal(s.Head) }

// BlockAt returns a pointer to the block at p.Path inside d, or nil if the
// path is invalid. For M1 only flat paths are used.
func (d *Document) BlockAt(p Position) *Block {
	if len(p.Path) == 0 {
		return nil
	}
	blocks := d.Blocks
	var b *Block
	for _, idx := range p.Path {
		if idx < 0 || idx >= len(blocks) {
			return nil
		}
		b = &blocks[idx]
		blocks = b.Children
	}
	return b
}

// Start returns the position at the very beginning of the document.
func (d *Document) Start() Position {
	return Position{Path: []int{0}, Inline: 0, Offset: 0}
}
