package editor

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/facubozzi/fyne-writer/internal/doc"
)

// TestWrapNoLeadingSpaceOnContinuation guards WRI-9: a paragraph that wraps
// across several visual lines must keep every continuation line flush with the
// first (no leading space), and the wrapped lines must still cover the block's
// plain text contiguously with no bytes dropped.
func TestWrapNoLeadingSpaceOnContinuation(t *testing.T) {
	test.NewApp() // fyne.MeasureText needs a driver

	d := doc.ParseMarkdown(
		"a fairly long paragraph of body text that should wrap across several " +
			"visual lines once the content width is narrow enough to force breaks\n")
	const width = 200

	lines := layout(d, width)

	// Collect the visual lines of the single paragraph block (index 0).
	var para []visualLine
	for _, ln := range lines {
		if ln.blockIdx == 0 {
			para = append(para, ln)
		}
	}
	if len(para) < 2 {
		t.Fatalf("expected the paragraph to wrap into >1 line, got %d", len(para))
	}

	// No continuation line may begin with a leading space/tab.
	for i := 1; i < len(para); i++ {
		if strings.HasPrefix(para[i].text, " ") || strings.HasPrefix(para[i].text, "\t") {
			t.Errorf("continuation line %d starts with whitespace: %q", i, para[i].text)
		}
	}

	// Byte ranges stay contiguous and cover the full plain text — no bytes
	// dropped or double-counted by the wrap-whitespace absorption.
	full := len(d.Blocks[0].PlainText())
	if para[0].startByte != 0 {
		t.Errorf("first line startByte = %d, want 0", para[0].startByte)
	}
	if para[len(para)-1].endByte != full {
		t.Errorf("last line endByte = %d, want %d", para[len(para)-1].endByte, full)
	}
	for i := 1; i < len(para); i++ {
		if para[i].startByte != para[i-1].endByte {
			t.Errorf("gap between line %d (endByte %d) and line %d (startByte %d)",
				i-1, para[i-1].endByte, i, para[i].startByte)
		}
	}
}
