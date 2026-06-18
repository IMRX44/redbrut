//go:build linux

package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorGreen   = lipgloss.Color("#00ff88")
	colorRed     = lipgloss.Color("#ff4466")
	colorYellow  = lipgloss.Color("#ffcc00")
	colorBlue    = lipgloss.Color("#44aaff")
	colorPurple  = lipgloss.Color("#cc88ff")
	colorDim     = lipgloss.Color("#555577")
	colorBg      = lipgloss.Color("#0d0d16")
	colorBorder  = lipgloss.Color("#2a2a44")
	colorMuted   = lipgloss.Color("#888899")
	colorWhite   = lipgloss.Color("#e0e0f0")

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGreen).
			PaddingBottom(1)

	styleBanner = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPurple)

	styleLabel = lipgloss.NewStyle().
			Foreground(colorMuted).
			Width(16)

	styleValue = lipgloss.NewStyle().
			Foreground(colorWhite)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	styleLocked = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	styleError = lipgloss.NewStyle().
			Foreground(colorRed)

	styleDim = lipgloss.NewStyle().
			Foreground(colorDim)

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	styleBoxActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPurple).
			Padding(0, 1)

	styleStat = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 2)

	styleFoundStat  = styleStat.Foreground(colorGreen)
	styleLockedStat = styleStat.Foreground(colorYellow)
	styleErrorStat  = styleStat.Foreground(colorRed)
	styleRetryStat  = styleStat.Foreground(colorBlue)
	styleSpeedStat  = styleStat.Foreground(colorPurple)

	styleHint = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGreen).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Width(60)
)

const banner = `
██████╗ ███████╗██████╗ ██████╗ ██████╗ ██╗   ██╗████████╗
██╔══██╗██╔════╝██╔══██╗██╔══██╗██╔══██╗██║   ██║╚══██╔══╝
██████╔╝█████╗  ██║  ██║██████╔╝██████╔╝██║   ██║   ██║
██╔══██╗██╔══╝  ██║  ██║██╔══██╗██╔══██╗██║   ██║   ██║
██║  ██║███████╗██████╔╝██████╔╝██║  ██║╚██████╔╝   ██║
╚═╝  ╚═╝╚══════╝╚═════╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝    ╚═╝  `
