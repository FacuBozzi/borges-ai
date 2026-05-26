package editor

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// TestLayoutCacheMatchesPure asserts the cached layout is byte-identical to the
// pure layout() function, and that hit / width-change / invalidate behave.
func TestLayoutCacheMatchesPure(t *testing.T) {
	test.NewApp() // fyne.MeasureText needs a driver

	d := doc.ParseMarkdown("# Title\n\n" +
		"a fairly long paragraph of body text that should wrap across several " +
		"visual lines once the content width is narrow enough to force breaks\n\n" +
		"- item one\n- item two\n")
	e := &RichEditor{}
	const width = 200

	want := layout(d, width)
	got := e.layoutCached(d, width)
	if len(got) != len(want) {
		t.Fatalf("line count: cached %d, pure %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d differs:\n cached %+v\n pure   %+v", i, got[i], want[i])
		}
	}
	if e.wrapCache == nil || len(e.wrapCache.entries) == 0 {
		t.Fatal("cache not populated after first layout")
	}

	// Second call hits the cache: identical result, no new entries.
	n := len(e.wrapCache.entries)
	got2 := e.layoutCached(d, width)
	if len(got2) != len(want) {
		t.Fatalf("second call line count %d, want %d", len(got2), len(want))
	}
	if len(e.wrapCache.entries) != n {
		t.Errorf("cache grew on hit: %d entries, want %d", len(e.wrapCache.entries), n)
	}

	// Width change rebuilds at the new width.
	e.layoutCached(d, width*2)
	if e.wrapCache.width != width*2 {
		t.Errorf("cache width = %v, want %v", e.wrapCache.width, float32(width*2))
	}

	// Invalidate drops the cache entirely.
	e.InvalidateLayoutCache()
	if e.wrapCache != nil {
		t.Error("cache not nil after InvalidateLayoutCache")
	}
}
