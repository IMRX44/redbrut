//go:build windows

package gui

import (
	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Run starts the Windows GUI application.
func Run() {
	a := fyneapp.New()
	a.Settings().SetTheme(&darkTheme{})

	w := a.NewWindow("redbrut  —  RDP Credential Testing")
	w.Resize(fyne.NewSize(960, 640))
	w.SetFixedSize(false)

	state := newUIState()

	// Live panel
	livePanel, liveContent := newLivePanel()
	state.LivePanel = livePanel

	// Config panel
	configContent := newConfigPanel(state, w)

	// Header
	header := widget.NewLabelWithStyle(
		"  ██████╗ ███████╗██████╗ ██████╗ ██████╗ ██╗   ██╗████████╗",
		fyne.TextAlignLeading,
		fyne.TextStyle{Monospace: true, Bold: true},
	)

	// Tabs
	tabs := container.NewAppTabs(
		container.NewTabItem("  Config  ", configContent),
		container.NewTabItem("  Live    ", liveContent),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	w.SetContent(container.NewBorder(header, nil, nil, nil, tabs))
	w.ShowAndRun()
}
