package editor

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

const benchWidth = 700

// bigDoc builds a document of `paras` paragraphs, each `wordsPerPara` words,
// to approximate a long document (200×50 ≈ 10k words).
func bigDoc(paras, wordsPerPara int) *doc.Document {
	para := strings.TrimSpace(strings.Repeat("lorem ", wordsPerPara))
	var sb strings.Builder
	for i := 0; i < paras; i++ {
		sb.WriteString(para)
		sb.WriteString("\n\n")
	}
	return doc.ParseMarkdown(sb.String())
}

// BenchmarkLayoutPure: full re-wrap of every block on every call (pre-cache).
func BenchmarkLayoutPure(b *testing.B) {
	test.NewApp()
	d := bigDoc(200, 50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = layout(d, benchWidth)
	}
}

// BenchmarkLayoutCachedWarm: steady-state Refresh with no edits (all hits) —
// the cost of every non-editing refresh (scroll, caret blink, selection).
func BenchmarkLayoutCachedWarm(b *testing.B) {
	test.NewApp()
	d := bigDoc(200, 50)
	e := &RichEditor{}
	e.layoutCached(d, benchWidth) // warm
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.layoutCached(d, benchWidth)
	}
}

// BenchmarkLayoutCachedTyping: the realistic per-keystroke case — one block's
// text changes each iteration, so only it re-wraps; the rest hit the cache.
// The edited block is reset to its original size periodically so it stays a
// realistic paragraph length rather than ballooning over b.N iterations.
func BenchmarkLayoutCachedTyping(b *testing.B) {
	test.NewApp()
	d := bigDoc(200, 50)
	orig := d.Blocks[0]
	e := &RichEditor{}
	e.layoutCached(d, benchWidth) // warm
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%64 == 0 {
			d.Blocks[0] = orig // keep the block paragraph-sized
		}
		d.Blocks[0] = doc.InsertText(d.Blocks[0], 0, "x", 0)
		_ = e.layoutCached(d, benchWidth)
	}
}
