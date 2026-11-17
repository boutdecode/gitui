package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

func sgr(kind int, c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", kind, r>>8, g>>8, b>>8)
}

// paintBackground fills every cell of a w×h screen with the theme background
// and default text colour. Styled spans end with a reset, which would drop
// the background, so each reset is followed by the base colours again.
func paintBackground(frame string, w, h int) string {
	if w <= 0 || h <= 0 {
		return frame
	}
	base := sgr(48, colBg) + sgr(38, colFg)
	lines := strings.Split(frame, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i, l := range lines[:h] {
		l = strings.ReplaceAll(l, "\x1b[0m", "\x1b[0m"+base)
		l = strings.ReplaceAll(l, "\x1b[m", "\x1b[m"+base)
		pad := max(w-lipgloss.Width(l), 0)
		lines[i] = base + l + strings.Repeat(" ", pad) + "\x1b[0m"
	}
	return strings.Join(lines[:h], "\n")
}
