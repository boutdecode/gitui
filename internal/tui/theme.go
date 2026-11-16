package tui

import (
	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
)

// Styles are package-level so every screen shares one look; applyTheme
// rebuilds them for a light or dark terminal background. Bubble Tea runs
// Update and View on a single goroutine, so reassigning them is safe.
var (
	isDark = true

	accent lipgloss.Style // only used for its colors, via .GetBackground()

	headerStyle, crumbStyle, statusStyle, infoStyle, errorStyle lipgloss.Style
	codeStyle, urlStyle, hintStyle                              lipgloss.Style
	titleStyle, authorStyle, dimStyle, promptStyle              lipgloss.Style
	diffAdd, diffDel, diffHunk, diffFile, diffMeta              lipgloss.Style
)

func init() { applyTheme(true) }

// applyTheme rebuilds all styles for a dark or light terminal background.
func applyTheme(dark bool) {
	isDark = dark
	ld := lipgloss.LightDark(dark)
	c := func(light, dark string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(ld(lipgloss.Color(light), lipgloss.Color(dark)))
	}

	bg := ld(lipgloss.Color("62"), lipgloss.Color("63"))
	onBg := ld(lipgloss.Color("255"), lipgloss.Color("230"))
	accent = lipgloss.NewStyle().Background(bg)

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(onBg).Background(bg).Padding(0, 1)
	codeStyle = lipgloss.NewStyle().Bold(true).Foreground(onBg).Background(bg).Padding(0, 2)

	crumbStyle = c("240", "245")
	statusStyle = c("240", "245")
	hintStyle = c("240", "245")
	dimStyle = c("240", "245")
	diffMeta = c("240", "245")

	infoStyle = c("28", "42")
	diffAdd = c("28", "42")

	errorStyle = c("160", "196").Bold(true)
	diffDel = c("160", "203")

	urlStyle = c("27", "39")
	diffHunk = c("27", "39")
	authorStyle = c("27", "39").Bold(true)

	promptStyle = c("130", "221").Bold(true)
	diffFile = c("130", "221").Bold(true)

	titleStyle = lipgloss.NewStyle().Bold(true)
}

// restyler is implemented by screens holding bubbles components whose own
// styles depend on the background.
type restyler interface{ Restyle(dark bool) }

func newListModel() list.Model {
	l := list.New(nil, newDelegate(isDark), 0, 0)
	l.Styles = list.DefaultStyles(isDark)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.KeyMap.Filter.SetKeys(keySearch.Keys()...)
	return l
}

func newDelegate(dark bool) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles = list.NewDefaultItemStyles(dark)
	return d
}

func restyleList(l *list.Model, dark bool) {
	l.Styles = list.DefaultStyles(dark)
	l.SetDelegate(newDelegate(dark))
}
