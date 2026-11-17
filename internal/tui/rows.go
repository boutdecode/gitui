package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// rowItem is a list item that draws itself. Row returns exactly the lines the
// delegate was built for (see rowDelegate.height), each w cells wide.
type rowItem interface {
	list.Item
	Row(w int, selected bool) string
}

// rowDelegate renders rowItems: it keeps the bubbles list for navigation and
// filtering but draws every row itself.
type rowDelegate struct{ height, spacing int }

func (d rowDelegate) Height() int  { return d.height }
func (d rowDelegate) Spacing() int { return d.spacing }

func (rowDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (rowDelegate) Render(w io.Writer, m list.Model, index int, it list.Item) {
	if r, ok := it.(rowItem); ok {
		fmt.Fprint(w, r.Row(m.Width(), index == m.Index()))
	}
}

// newRowList builds a list that shows no chrome of its own (no title, filter
// bar, status or pagination): screens draw those parts themselves.
func newRowList(height, spacing int) list.Model {
	l := list.New(nil, rowDelegate{height, spacing}, 0, 0)
	l.Styles = list.DefaultStyles(isDark)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowFilter(false)
	l.SetShowPagination(false)
	l.KeyMap.Filter.SetKeys(keySearch.Keys()...)
	// The list's paging letters (h b u f d g G) would shadow our own actions.
	l.KeyMap.NextPage.SetKeys("pgdown")
	l.KeyMap.PrevPage.SetKeys("pgup")
	l.KeyMap.GoToStart.SetKeys("home")
	l.KeyMap.GoToEnd.SetKeys("end")
	return l
}

func restyleList(l *list.Model, dark bool) { l.Styles = list.DefaultStyles(dark) }

// marker is the selection gutter: "▌" on the selected row, blank otherwise.
func marker(selected bool) string {
	if selected {
		return "▌"
	}
	return " "
}

// selectLine highlights a whole row over w cells.
func selectLine(s string, w int, selected bool) string {
	if !selected {
		return s
	}
	return selStyle.Render(fit(strings.TrimRight(stripStyle(s), " "), w))
}

func stripStyle(s string) string { return plain(s) }

// toolbarHeight is the height of toolbar(): a bordered search box and bordered
// filter pills side by side.
const toolbarHeight = 3

// box draws content inside a rounded border; width 0 fits the content.
func box(content string, width int, active bool) string {
	bc := colBorder
	if active {
		bc = colAccent
	}
	st := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(bc).Padding(0, 1)
	if width > 0 {
		st = st.Width(width)
	}
	return st.Render(content)
}

// pill is a bordered "label : value" filter indicator.
func pill(label, value string) string {
	return box(dimStyle.Render(label+" : ")+value, 0, false)
}

// toolbar renders the search box (taking the remaining width) followed by the
// filter pills, e.g. "/ Filtrer…   [statut : ouvertes]".
func toolbar(w int, l list.Model, placeholder string, pills ...string) string {
	pillsW := 0
	for _, p := range pills {
		pillsW += lipgloss.Width(p) + 1
	}
	slash := pinkStyle.Render("/") + " "
	var text string
	active := false
	switch {
	case l.FilterState() == list.Filtering:
		text, active = slash+l.FilterValue()+"█", true
	case l.FilterState() == list.FilterApplied:
		text = slash + l.FilterValue()
	default:
		text = slash + dimStyle.Render(placeholder)
	}
	search := box(fit(text, max(w-pillsW-4, 1)), max(w-pillsW, 6), active)
	parts := []string{search}
	for _, p := range pills {
		parts = append(parts, " ", p)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
