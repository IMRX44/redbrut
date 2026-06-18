//go:build windows

package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type darkTheme struct{}

func (d *darkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x0d, G: 0x0d, B: 0x16, A: 0xff}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x1a, G: 0x1a, B: 0x2e, A: 0xff}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xe0, G: 0xe0, B: 0xf0, A: 0xff}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x00, G: 0xff, B: 0x88, A: 0xff}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0xcc, G: 0x88, B: 0xff, A: 0xff}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x15, G: 0x15, B: 0x25, A: 0xff}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x44, G: 0x44, B: 0x66, A: 0xff}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x88}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x1a, G: 0x1a, B: 0x2e, A: 0xff}
	case theme.ColorNameMenuBackground:
		return color.NRGBA{R: 0x15, G: 0x15, B: 0x25, A: 0xff}
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0x00, G: 0xff, B: 0x88, A: 0xff}
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xff, G: 0xcc, B: 0x00, A: 0xff}
	case theme.ColorNameError:
		return color.NRGBA{R: 0xff, G: 0x44, B: 0x66, A: 0xff}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0xcc, G: 0x88, B: 0xff, A: 0x55}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 0x11, G: 0x11, B: 0x20, A: 0xff}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0x2a, G: 0x2a, B: 0x44, A: 0xff}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0x44, G: 0x44, B: 0x66, A: 0xff}
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (d *darkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (d *darkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (d *darkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNameInputBorder:
		return 1.5
	}
	return theme.DefaultTheme().Size(name)
}
