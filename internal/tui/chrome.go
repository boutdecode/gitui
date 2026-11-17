package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// tab is one entry of the tab bar. Count < 0 means "no counter".
type tab struct {
	Label string
	Count int
}

// headerView is the top line: logo, context on the left, the signed-in user
// on the right. The context is truncated first when space runs out.
func headerView(w int, context, user string) string {
	logo := headerStyle.Render("gitui")
	right := ""
	if user != "" {
		right = greenStyle.Render("● connecté : @" + user)
	}
	avail := w - lipgloss.Width(logo) - lipgloss.Width(right) - 2
	ctx := lipgloss.NewStyle().MaxWidth(max(avail, 0)).Render(context)
	gap := w - lipgloss.Width(logo) - 1 - lipgloss.Width(ctx) - lipgloss.Width(right)
	return logo + " " + ctx + strings.Repeat(" ", max(gap, 1)) + right
}

// tabsView renders "1 Code  2 Pull requests 12 …", the active tab highlighted.
func tabsView(tabs []tab, active int) string {
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		label := fmt.Sprintf("%d %s", i+1, t.Label)
		switch {
		case i == active && t.Count >= 0:
			parts[i] = tabActiveStyle.Render(fmt.Sprintf("%s %d", label, t.Count))
		case i == active:
			parts[i] = tabActiveStyle.Render(label)
		case t.Count >= 0:
			parts[i] = tabInactiveStyle.Render(label + " " + pinkStyle.Background(colKeyBg).Render(fmt.Sprint(t.Count)))
		default:
			parts[i] = tabInactiveStyle.Render(label)
		}
	}
	return strings.Join(parts, " ")
}

// helpBar renders bindings as "[key] description". In short mode items are
// dropped (the last `keep` ones never) until the bar fits one line; in full
// mode it wraps onto as many lines as needed.
func helpBar(bs []key.Binding, w int, full bool, keep int) string {
	items := make([]string, 0, len(bs))
	for _, b := range bs {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		items = append(items, keyCapStyle.Render(h.Key)+" "+dimStyle.Render(h.Desc))
	}
	const sep = "  "
	if full {
		var lines []string
		cur := ""
		for _, it := range items {
			if cur != "" && lipgloss.Width(cur)+len(sep)+lipgloss.Width(it) > w {
				lines = append(lines, cur)
				cur = ""
			}
			if cur != "" {
				cur += sep
			}
			cur += it
		}
		return strings.Join(append(lines, cur), "\n")
	}
	for w > 0 && len(items) > keep && lipgloss.Width(strings.Join(items, sep)) > w {
		items = append(items[:len(items)-keep-1], items[len(items)-keep:]...)
	}
	return strings.Join(items, sep)
}
