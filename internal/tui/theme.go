package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette (from the design mockup). applyTheme rebuilds it for a light or dark
// terminal background; screens use the styles below, never raw colors.
var (
	isDark = true

	colAccent, colBorder, colOnAccent         color.Color
	colPink, colPurple, colGreen, colYellow   color.Color
	colBlue, colRed, colDim, colKeyBg, colKey color.Color
)

// Styles are package-level so every screen shares one look. Bubble Tea runs
// Update and View on a single goroutine, so reassigning them is safe.
var (
	headerStyle, crumbStyle, statusStyle, infoStyle, errorStyle lipgloss.Style
	codeStyle, urlStyle, hintStyle                              lipgloss.Style
	titleStyle, authorStyle, dimStyle, promptStyle              lipgloss.Style
	diffAdd, diffDel, diffHunk, diffFile, diffMeta              lipgloss.Style

	pinkStyle, purpleStyle, greenStyle, yellowStyle, blueStyle, redStyle lipgloss.Style
	tabActiveStyle, tabInactiveStyle, keyCapStyle, selStyle, openBadge   lipgloss.Style
	closedBadge, mergedBadge                                             lipgloss.Style
)

func init() { applyTheme(true) }

// applyTheme rebuilds all styles for a dark or light terminal background.
func applyTheme(dark bool) {
	isDark = dark
	ld := lipgloss.LightDark(dark)
	hex := func(light, dark string) color.Color { return ld(lipgloss.Color(light), lipgloss.Color(dark)) }
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

	colAccent = hex("#6a3fe0", "#7d56f4")
	colOnAccent = lipgloss.Color("#ffffff")
	colBorder = hex("#cfc8e6", "#3a3358")
	colPink = hex("#bf2c86", "#ff6ac1")
	colPurple = hex("#6e4fd1", "#b9a2ff")
	colGreen = hex("#1a7f37", "#7ee787")
	colYellow = hex("#8a5a00", "#e3b341")
	colBlue = hex("#0a5bc4", "#79c0ff")
	colRed = hex("#cf222e", "#ff7b72")
	colDim = hex("#5f587a", "#9a93b8")
	colKeyBg = hex("#e8e4f5", "#2a2540")
	colKey = hex("#2a2540", "#e6e1f5")

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(hex("#ffffff", "#14121f")).Background(colPink).Padding(0, 1)
	codeStyle = lipgloss.NewStyle().Bold(true).Foreground(colOnAccent).Background(colAccent).Padding(0, 2)

	dimStyle = fg(colDim)
	crumbStyle, statusStyle, hintStyle, diffMeta = dimStyle, dimStyle, dimStyle, dimStyle

	greenStyle = fg(colGreen)
	infoStyle, diffAdd = greenStyle, greenStyle

	redStyle = fg(colRed)
	errorStyle = fg(colRed).Bold(true)
	diffDel = redStyle

	blueStyle = fg(colBlue)
	urlStyle, diffHunk = blueStyle, blueStyle
	authorStyle = fg(colBlue).Bold(true)

	yellowStyle = fg(colYellow)
	promptStyle = fg(colYellow).Bold(true)
	diffFile = promptStyle

	pinkStyle = fg(colPink)
	purpleStyle = fg(colPurple)
	titleStyle = lipgloss.NewStyle().Bold(true)

	tabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(colOnAccent).Background(colAccent).Padding(0, 1)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(colDim).Background(colKeyBg).Padding(0, 1)
	keyCapStyle = lipgloss.NewStyle().Foreground(colKey).Background(colKeyBg).Padding(0, 1)
	selStyle = lipgloss.NewStyle().Foreground(colOnAccent).Background(colAccent)

	badge := func(bg string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color(bg)).Padding(0, 1)
	}
	openBadge, closedBadge, mergedBadge = badge("#238636"), badge("#cf222e"), badge("#8250df")
}

// restyler is implemented by screens holding bubbles components whose own
// styles depend on the background.
type restyler interface{ Restyle(dark bool) }
