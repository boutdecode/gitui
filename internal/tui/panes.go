package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// minSplitWidth is the narrowest terminal that still shows list and preview
// side by side; below it screens show the list only.
const minSplitWidth = 90

func canSplit(w int) bool { return w >= minSplitWidth }

// splitWidths returns the outer widths of the left and right panes.
func splitWidths(w int, leftWidth int) (int, int) {
	l := min(max(leftWidth, 30), w-30)
	return l, w - l
}

// pane draws content inside a rounded border of exactly w×h cells (borders
// included). Content that is too wide or too tall is clipped, never wrapped.
func pane(content string, w, h int, active bool) string {
	if w < 4 || h < 3 {
		return ""
	}
	bc := colBorder
	if active {
		bc = colAccent
	}
	inner := lipgloss.NewStyle().MaxWidth(w - 4).MaxHeight(h - 2).Render(content)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(bc).
		Padding(0, 1).Width(w).Height(h).Render(inner)
}

// splitView renders two panes side by side in a w×h area. leftWidth is the
// preferred outer width of the left pane. The focused pane gets the accent border.
func splitView(left, right string, w, h, leftWidth int, focusLeft bool) string {
	lw, rw := splitWidths(w, leftWidth)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		pane(left, lw, h, focusLeft),
		pane(right, rw, h, !focusLeft))
}

// rule is a horizontal separator, optionally with a centered title.
func rule(w int, title string) string {
	if w < 1 {
		return ""
	}
	if title == "" {
		return dimStyle.Render(strings.Repeat("─", w))
	}
	t := " " + title + " "
	tw := lipgloss.Width(t)
	if tw >= w {
		return dimStyle.Render(strings.Repeat("─", w))
	}
	l := (w - tw) / 2
	return dimStyle.Render(strings.Repeat("─", l) + t + strings.Repeat("─", w-tw-l))
}
