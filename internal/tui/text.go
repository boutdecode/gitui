package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// fit truncates s to w cells (with an ellipsis) and pads it with spaces.
// plain strips ANSI styling.
func plain(s string) string { return ansi.Strip(s) }

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(w-lipgloss.Width(s), 0))
}

// fitRight is fit with right alignment.
func fitRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return strings.Repeat(" ", max(w-lipgloss.Width(s), 0)) + s
}

// humanCount abbreviates counts: 950 → "950", 6100 → "6.1k", 29412 → "29k".
func humanCount(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprint(n)
	case n < 10000:
		return strings.Replace(fmt.Sprintf("%.1fk", float64(n)/1000), ".0k", "k", 1)
	case n < 1000000:
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1e6)
}

// since formats a past time compactly in French: "2 h", "hier", "3 sem".
func since(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "à l'instant"
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d j", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d sem", int(d.Hours()/24/7))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%d mois", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%d ans", int(d.Hours()/24/365))
}

// ilYa is since() in a sentence: "il y a 2 h".
func ilYa(t time.Time) string {
	s := since(t)
	if s == "" || s == "à l'instant" {
		return s
	}
	return "il y a " + s
}

// styleBody lightly styles a markdown body for the preview panes: headings
// in purple bold, list bullets in pink, everything wrapped to w cells.
func styleBody(body string, w int) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(strings.TrimSpace(body), "\n")
	for i, l := range lines {
		t := strings.TrimLeft(l, " ")
		switch {
		case strings.HasPrefix(t, "#"):
			lines[i] = purpleStyle.Bold(true).Render(strings.TrimSpace(strings.TrimLeft(t, "#")))
		case strings.HasPrefix(t, "- "), strings.HasPrefix(t, "* "):
			lines[i] = pinkStyle.Render("-") + t[1:]
		}
	}
	return lipgloss.NewStyle().Width(max(w, 1)).Render(strings.Join(lines, "\n"))
}

// chip renders a colored label pill from a GitHub label name; the color is
// derived from the name so the same label always looks the same.
func chip(label string) string {
	palette := []string{"#ff7b72", "#79c0ff", "#7ee787", "#e3b341", "#b9a2ff", "#ff6ac1", "#9a93b8"}
	switch strings.ToLower(label) {
	case "bug":
		return chipWith(label, palette[0])
	case "feature", "enhancement":
		return chipWith(label, palette[2])
	case "docs", "documentation":
		return chipWith(label, palette[3])
	}
	h := 0
	for _, r := range label {
		h = (h*31 + int(r)) % len(palette)
	}
	return chipWith(label, palette[h])
}

func chipWith(label, bg string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#14121f")).Background(lipgloss.Color(bg)).
		Padding(0, 1).Render(label)
}
