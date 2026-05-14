package doc

// BlockType enumerates the structural kinds of block we support. M1 only
// emits BlockParagraph; M2 adds the rest.
type BlockType int

const (
	BlockParagraph BlockType = iota
	BlockHeading
	BlockBulletList
	BlockOrderedList
	BlockListItem
	BlockQuote
	BlockCodeBlock
	BlockHR
)

// Block is a node in the document tree. Leaf blocks (Paragraph, Heading,
// CodeBlock, ListItem) carry Inlines; container blocks (Quote, lists) carry
// Children. Meta holds type-specific data (e.g., heading level, code lang).
type Block struct {
	Type     BlockType
	Inlines  []Inline
	Children []Block
	Meta     map[string]any
}

// Inline is a contiguous run of text sharing the same set of marks.
type Inline struct {
	Text  string
	Marks Mark
}

// Document is the editable model. M1 only fills it with paragraph blocks.
type Document struct {
	Blocks []Block
	Meta   DocMeta
}

// DocMeta is document-level metadata stored in YAML front-matter at the top
// of the .md file. Background instructions (the per-document system prompt
// for AI commands) live here.
type DocMeta struct {
	Instructions string
}

// New returns an empty document containing a single empty paragraph so the
// caret always has a valid position.
func New() *Document {
	return &Document{Blocks: []Block{{Type: BlockParagraph, Inlines: []Inline{{}}}}}
}

// Clone produces a deep copy. Used by the undo stack and version snapshots.
func (d *Document) Clone() *Document {
	out := &Document{
		Blocks: make([]Block, len(d.Blocks)),
		Meta:   d.Meta,
	}
	for i, b := range d.Blocks {
		out.Blocks[i] = b.clone()
	}
	return out
}

func (b Block) clone() Block {
	cp := Block{Type: b.Type}
	if len(b.Inlines) > 0 {
		cp.Inlines = make([]Inline, len(b.Inlines))
		copy(cp.Inlines, b.Inlines)
	}
	if len(b.Children) > 0 {
		cp.Children = make([]Block, len(b.Children))
		for i, c := range b.Children {
			cp.Children[i] = c.clone()
		}
	}
	if b.Meta != nil {
		cp.Meta = make(map[string]any, len(b.Meta))
		for k, v := range b.Meta {
			cp.Meta[k] = v
		}
	}
	return cp
}

// PlainText returns the concatenated rune text of all inlines in the block.
// Useful for measurement, search, and line breaking.
func (b Block) PlainText() string {
	n := 0
	for _, in := range b.Inlines {
		n += len(in.Text)
	}
	if n == 0 {
		return ""
	}
	out := make([]byte, 0, n)
	for _, in := range b.Inlines {
		out = append(out, in.Text...)
	}
	return string(out)
}
