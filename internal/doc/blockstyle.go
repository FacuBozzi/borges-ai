package doc

// BlockStyle is the visual style derived from a block's structural type.
// Inline marks combine on top of this (italic still italicizes inside a
// heading, etc.). The editor renderer consults this when laying out and
// drawing blocks.
type BlockStyle struct {
	FontSize       float32
	LineHeight     float32
	GapBelow       float32 // vertical spacing after this block
	Bold           bool    // headings + (in future) emphasis blocks
	Monospace      bool    // code blocks
	Indent         float32 // leading horizontal indent
	GutterText     string  // shown in the gutter (bullet, number)
	LeftBar        bool    // draw a left vertical bar (blockquote)
	Background     bool    // draw a background rectangle (code block)
	IsHorizontalRule bool // render the block as a single thin line
}

// HeadingLevel returns the heading level (1..6) or 0 if not a heading.
func HeadingLevel(b Block) int {
	if b.Type != BlockHeading {
		return 0
	}
	if v, ok := b.Meta["level"].(int); ok && v >= 1 && v <= 6 {
		return v
	}
	return 1
}

// StyleForBlock returns the visual style for the given block. Pure function;
// the editor passes the result around in the layout pass.
func StyleForBlock(b Block) BlockStyle {
	switch b.Type {
	case BlockHeading:
		switch HeadingLevel(b) {
		case 1:
			return BlockStyle{FontSize: 28, LineHeight: 40, GapBelow: 14, Bold: true}
		case 2:
			return BlockStyle{FontSize: 23, LineHeight: 34, GapBelow: 12, Bold: true}
		case 3:
			return BlockStyle{FontSize: 19, LineHeight: 28, GapBelow: 10, Bold: true}
		case 4:
			return BlockStyle{FontSize: 17, LineHeight: 26, GapBelow: 8, Bold: true}
		case 5:
			return BlockStyle{FontSize: 16, LineHeight: 24, GapBelow: 8, Bold: true}
		default:
			return BlockStyle{FontSize: 15, LineHeight: 24, GapBelow: 8, Bold: true}
		}
	case BlockCodeBlock:
		return BlockStyle{FontSize: 14, LineHeight: 22, GapBelow: 8, Monospace: true, Indent: 16, Background: true}
	case BlockQuote:
		return BlockStyle{FontSize: 15, LineHeight: 24, GapBelow: 8, Indent: 18, LeftBar: true}
	case BlockListItem:
		return BlockStyle{FontSize: 15, LineHeight: 24, GapBelow: 0, Indent: 24}
	case BlockHR:
		return BlockStyle{FontSize: 1, LineHeight: 16, GapBelow: 8, IsHorizontalRule: true}
	default:
		return BlockStyle{FontSize: 15, LineHeight: 24, GapBelow: 8}
	}
}
