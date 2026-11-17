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

// searchBar shows the list's active search ("recherche : bubble█"), or just
// the hint when idle.
func searchBar(l list.Model) string {
	switch {
	case l.FilterState() == list.Filtering:
		return dimStyle.Render("recherche : ") + blueStyle.Render(l.FilterValue()+"█")
	case l.FilterState() == list.FilterApplied:
		return dimStyle.Render("recherche : ") + blueStyle.Render(l.FilterValue())
	}
	return dimStyle.Render("recherche : /")
}

// filterBar draws "filtre : ● a  b  c" with the current option marked, and
// the right-hand text (e.g. the search bar) pushed to the far edge.
func filterBar(w int, options []string, current int, right string) string {
	parts := make([]string, len(options))
	for i, o := range options {
		if i == current {
			parts[i] = pinkStyle.Render("● " + o)
		} else {
			parts[i] = dimStyle.Render(o)
		}
	}
	left := dimStyle.Render("filtre : ") + strings.Join(parts, "  ")
	gap := max(w-lipgloss.Width(left)-lipgloss.Width(right), 2)
	return left + strings.Repeat(" ", gap) + right
}
