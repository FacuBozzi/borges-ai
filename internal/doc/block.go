package doc

// Helpers for working with the inline runs inside a block. Position offsets
// in the editor are byte offsets within the block's concatenated PlainText,
// not within a specific inline — these helpers translate when needed.

// InlineAt returns the index of the inline that contains the given byte
// offset within b.PlainText(), along with the offset within that inline.
// Byte offsets at an inline boundary belong to the inline on the left, so
// the caret can sit at the *end* of inline N rather than the *start* of
// inline N+1, matching most editors' behavior.
func InlineAt(b Block, byteOff int) (inline, offset int) {
	if len(b.Inlines) == 0 {
		return 0, 0
	}
	consumed := 0
	for i, in := range b.Inlines {
		ln := len(in.Text)
		if byteOff <= consumed+ln {
			return i, byteOff - consumed
		}
		consumed += ln
	}
	last := len(b.Inlines) - 1
	return last, len(b.Inlines[last].Text)
}

// MarksAt returns the marks active at the given byte offset. At an inline
// boundary, prefers the marks of the inline on the left (matches InlineAt).
func MarksAt(b Block, byteOff int) Mark {
	if len(b.Inlines) == 0 {
		return 0
	}
	i, _ := InlineAt(b, byteOff)
	return b.Inlines[i].Marks
}

// InsertText inserts s into b at the given byte offset, attaching the given
// marks to the new text. If the marks match an adjacent inline, the new
// text extends that inline; otherwise a new inline is spliced in (splitting
// the host inline if the insertion is in the middle of one).
//
// Returns the modified block by value. Caller assigns it back.
func InsertText(b Block, byteOff int, s string, marks Mark) Block {
	if s == "" {
		return b
	}
	if len(b.Inlines) == 0 {
		b.Inlines = []Inline{{Text: s, Marks: marks}}
		return b
	}
	inlineIdx, local := InlineAt(b, byteOff)
	host := b.Inlines[inlineIdx]
	if host.Marks == marks {
		host.Text = host.Text[:local] + s + host.Text[local:]
		b.Inlines[inlineIdx] = host
		return normalizeInlines(b)
	}
	// Caret at end of host inline and the next inline has matching marks?
	if local == len(host.Text) && inlineIdx+1 < len(b.Inlines) && b.Inlines[inlineIdx+1].Marks == marks {
		nxt := b.Inlines[inlineIdx+1]
		nxt.Text = s + nxt.Text
		b.Inlines[inlineIdx+1] = nxt
		return normalizeInlines(b)
	}
	// Caret at start of host inline and prev inline matches? Already handled
	// by InlineAt's left-preference. So we only get here when we must split.
	left := Inline{Text: host.Text[:local], Marks: host.Marks}
	mid := Inline{Text: s, Marks: marks}
	right := Inline{Text: host.Text[local:], Marks: host.Marks}
	out := make([]Inline, 0, len(b.Inlines)+2)
	out = append(out, b.Inlines[:inlineIdx]...)
	if left.Text != "" {
		out = append(out, left)
	}
	out = append(out, mid)
	if right.Text != "" {
		out = append(out, right)
	}
	out = append(out, b.Inlines[inlineIdx+1:]...)
	b.Inlines = out
	return normalizeInlines(b)
}

// DeleteRange removes bytes [from, to) from b.PlainText(), preserving inline
// boundaries outside the deleted span. Returns the modified block.
func DeleteRange(b Block, from, to int) Block {
	if from >= to || len(b.Inlines) == 0 {
		return b
	}
	plain := b.PlainText()
	if to > len(plain) {
		to = len(plain)
	}
	if from < 0 {
		from = 0
	}
	fromI, fromOff := InlineAt(b, from)
	toI, toOff := InlineAt(b, to)
	if fromI == toI {
		host := b.Inlines[fromI]
		host.Text = host.Text[:fromOff] + host.Text[toOff:]
		if host.Text == "" {
			b.Inlines = append(b.Inlines[:fromI], b.Inlines[fromI+1:]...)
		} else {
			b.Inlines[fromI] = host
		}
		return normalizeInlines(b)
	}
	// Truncate fromI's tail and toI's head, then drop everything between.
	leftInline := b.Inlines[fromI]
	leftInline.Text = leftInline.Text[:fromOff]
	rightInline := b.Inlines[toI]
	rightInline.Text = rightInline.Text[toOff:]
	out := make([]Inline, 0, len(b.Inlines))
	out = append(out, b.Inlines[:fromI]...)
	if leftInline.Text != "" {
		out = append(out, leftInline)
	}
	if rightInline.Text != "" {
		out = append(out, rightInline)
	}
	out = append(out, b.Inlines[toI+1:]...)
	b.Inlines = out
	return normalizeInlines(b)
}

// ApplyMark sets or clears mark m on bytes [from, to) of b. Splits inlines
// at the boundaries as needed.
func ApplyMark(b Block, from, to int, m Mark, set bool) Block {
	if from >= to || len(b.Inlines) == 0 {
		return b
	}
	plain := b.PlainText()
	if to > len(plain) {
		to = len(plain)
	}
	if from < 0 {
		from = 0
	}
	b = splitInlineAt(b, from)
	b = splitInlineAt(b, to)
	consumed := 0
	for i := range b.Inlines {
		start := consumed
		end := consumed + len(b.Inlines[i].Text)
		if start >= from && end <= to {
			if set {
				b.Inlines[i].Marks = b.Inlines[i].Marks.With(m)
			} else {
				b.Inlines[i].Marks = b.Inlines[i].Marks.Without(m)
			}
		}
		consumed = end
	}
	return normalizeInlines(b)
}

// splitInlineAt ensures there is an inline boundary at byteOff. If byteOff
// already lies on a boundary, returns b unchanged.
func splitInlineAt(b Block, byteOff int) Block {
	if len(b.Inlines) == 0 {
		return b
	}
	idx, local := InlineAt(b, byteOff)
	if local == 0 || local == len(b.Inlines[idx].Text) {
		return b
	}
	host := b.Inlines[idx]
	left := Inline{Text: host.Text[:local], Marks: host.Marks}
	right := Inline{Text: host.Text[local:], Marks: host.Marks}
	out := make([]Inline, 0, len(b.Inlines)+1)
	out = append(out, b.Inlines[:idx]...)
	out = append(out, left, right)
	out = append(out, b.Inlines[idx+1:]...)
	b.Inlines = out
	return b
}

// normalizeInlines merges adjacent inlines that share the same marks and
// drops any empty inlines (unless that would leave the block with zero
// inlines, in which case we keep a single empty one as a caret anchor).
func normalizeInlines(b Block) Block {
	if len(b.Inlines) == 0 {
		b.Inlines = []Inline{{}}
		return b
	}
	out := b.Inlines[:0]
	for _, in := range b.Inlines {
		if in.Text == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Marks == in.Marks {
			out[n-1].Text += in.Text
			continue
		}
		out = append(out, in)
	}
	if len(out) == 0 {
		out = append(out, Inline{Marks: b.Inlines[0].Marks})
	}
	b.Inlines = out
	return b
}

// SplitBlock splits b into two halves at byte offset byteOff. Returns the
// left and right halves. Each half is normalized.
func SplitBlock(b Block, byteOff int) (Block, Block) {
	if len(b.Inlines) == 0 {
		return b, Block{Type: b.Type, Inlines: []Inline{{}}}
	}
	idx, local := InlineAt(b, byteOff)
	left := Block{Type: b.Type, Meta: cloneMeta(b.Meta)}
	right := Block{Type: b.Type, Meta: cloneMeta(b.Meta)}
	left.Inlines = append(left.Inlines, b.Inlines[:idx]...)
	right.Inlines = append(right.Inlines, b.Inlines[idx+1:]...)
	host := b.Inlines[idx]
	if local > 0 {
		left.Inlines = append(left.Inlines, Inline{Text: host.Text[:local], Marks: host.Marks})
	}
	if local < len(host.Text) {
		right.Inlines = append([]Inline{{Text: host.Text[local:], Marks: host.Marks}}, right.Inlines...)
	}
	return normalizeInlines(left), normalizeInlines(right)
}

func cloneMeta(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
