package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// FyneWriterTheme implements fyne.Theme with a modern, restrained palette.
// Font embedding (Inter / JetBrains Mono) is deferred to a later milestone;
// we fall back to the default theme's fonts so the app boots without assets.
type FyneWriterTheme struct{}

func NewTheme() fyne.Theme { return FyneWriterTheme{} }

var (
	// Dark palette
	darkBackground = color.NRGBA{0x0F, 0x11, 0x15, 0xFF}
	darkSurface    = color.NRGBA{0x17, 0x1A, 0x21, 0xFF}
	darkText       = color.NRGBA{0xE6, 0xE9, 0xEF, 0xFF}
	darkMuted      = color.NRGBA{0x8A, 0x93, 0xA6, 0xFF}
	darkAccent     = color.NRGBA{0x7C, 0x9C, 0xFF, 0xFF}
	darkDanger     = color.NRGBA{0xF0, 0x71, 0x78, 0xFF}
	darkBorder     = color.NRGBA{0x26, 0x2A, 0x33, 0xFF}
	darkHover      = color.NRGBA{0xFF, 0xFF, 0xFF, 0x10}

	// Light palette
	lightBackground = color.NRGBA{0xFB, 0xFB, 0xFC, 0xFF}
	lightSurface    = color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}
	lightText       = color.NRGBA{0x1A, 0x1D, 0x24, 0xFF}
	lightMuted      = color.NRGBA{0x6B, 0x72, 0x80, 0xFF}
	lightAccent     = color.NRGBA{0x4F, 0x6B, 0xED, 0xFF}
	lightDanger     = color.NRGBA{0xD0, 0x46, 0x4C, 0xFF}
	lightBorder     = color.NRGBA{0xE5, 0xE7, 0xEB, 0xFF}
	lightHover      = color.NRGBA{0x00, 0x00, 0x00, 0x0C}
)

func (FyneWriterTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	dark := v == theme.VariantDark
	switch name {
	case theme.ColorNameBackground:
		return pick(dark, darkBackground, lightBackground)
	case theme.ColorNameForeground:
		return pick(dark, darkText, lightText)
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		return pick(dark, darkAccent, lightAccent)
	case theme.ColorNameError:
		return pick(dark, darkDanger, lightDanger)
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return pick(dark, darkMuted, lightMuted)
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return pick(dark, darkSurface, lightSurface)
	case theme.ColorNameButton:
		return pick(dark, darkSurface, lightSurface)
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return pick(dark, darkBorder, lightBorder)
	case theme.ColorNameHover, theme.ColorNamePressed:
		return pick(dark, darkHover, lightHover)
	case theme.ColorNameForegroundOnPrimary:
		return color.White
	case theme.ColorNameSelection:
		if dark {
			return color.NRGBA{0x7C, 0x9C, 0xFF, 0x55}
		}
		return color.NRGBA{0x4F, 0x6B, 0xED, 0x33}
	}
	return theme.DefaultTheme().Color(name, v)
}

func (FyneWriterTheme) Font(s fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(s)
}

func (FyneWriterTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (FyneWriterTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameText:
		return 15
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 18
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	}
	return theme.DefaultTheme().Size(n)
}

func pick(dark bool, d, l color.Color) color.Color {
	if dark {
		return d
	}
	return l
}
