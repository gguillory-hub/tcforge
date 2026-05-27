//go:build gui

package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type professionalDarkTheme struct {
	base fyne.Theme
}

func newProfessionalDarkTheme() fyne.Theme {
	return &professionalDarkTheme{base: theme.DarkTheme()}
}

func (t *professionalDarkTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x1e, G: 0x1f, B: 0x22, A: 0xff}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x2a, G: 0x2d, B: 0x31, A: 0xff}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 0x24, G: 0x26, B: 0x2a, A: 0xff}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x24, G: 0x27, B: 0x2b, A: 0xff}
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x2a, G: 0x2d, B: 0x31, A: 0xff}
	case theme.ColorNameSeparator, theme.ColorNameInputBorder:
		return color.NRGBA{R: 0x36, G: 0x3a, B: 0x40, A: 0xff}
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return color.NRGBA{R: 0x6a, G: 0xa9, B: 0xff, A: 0xff}
	case theme.ColorNameFocus, theme.ColorNameSelection:
		return color.NRGBA{R: 0x6a, G: 0xa9, B: 0xff, A: 0x42}
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0x6f, G: 0xbf, B: 0x8f, A: 0xff}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xd6, G: 0xa8, B: 0x4f, A: 0xff}
	case theme.ColorNameError:
		return color.NRGBA{R: 0xd0, G: 0x6b, B: 0x6b, A: 0xff}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xe6, G: 0xe9, B: 0xed, A: 0xff}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x9a, G: 0xa1, B: 0xaa, A: 0xff}
	}
	return t.base.Color(name, theme.VariantDark)
}

func (t *professionalDarkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *professionalDarkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *professionalDarkTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}
