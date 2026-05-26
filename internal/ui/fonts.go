package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// Embedded faces: Inter (regular/italic/bold/bold-italic) for proportional
// text, JetBrains Mono (regular/bold) for monospace. Both are SIL Open Font
// License; see the *-LICENSE / *-OFL files alongside the .ttf assets.

//go:embed assets/fonts/Inter-Regular.ttf
var interRegular []byte

//go:embed assets/fonts/Inter-Italic.ttf
var interItalic []byte

//go:embed assets/fonts/Inter-Bold.ttf
var interBold []byte

//go:embed assets/fonts/Inter-BoldItalic.ttf
var interBoldItalic []byte

//go:embed assets/fonts/JetBrainsMono-Regular.ttf
var monoRegular []byte

//go:embed assets/fonts/JetBrainsMono-Bold.ttf
var monoBold []byte

var (
	fontInterRegular    = fyne.NewStaticResource("Inter-Regular.ttf", interRegular)
	fontInterItalic     = fyne.NewStaticResource("Inter-Italic.ttf", interItalic)
	fontInterBold       = fyne.NewStaticResource("Inter-Bold.ttf", interBold)
	fontInterBoldItalic = fyne.NewStaticResource("Inter-BoldItalic.ttf", interBoldItalic)
	fontMonoRegular     = fyne.NewStaticResource("JetBrainsMono-Regular.ttf", monoRegular)
	fontMonoBold        = fyne.NewStaticResource("JetBrainsMono-Bold.ttf", monoBold)
)

// embeddedFont maps a Fyne text style onto an embedded face.
func embeddedFont(s fyne.TextStyle) fyne.Resource {
	if s.Monospace {
		if s.Bold {
			return fontMonoBold
		}
		return fontMonoRegular
	}
	switch {
	case s.Bold && s.Italic:
		return fontInterBoldItalic
	case s.Bold:
		return fontInterBold
	case s.Italic:
		return fontInterItalic
	default:
		return fontInterRegular
	}
}
