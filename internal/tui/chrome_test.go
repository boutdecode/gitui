package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

func TestHeaderShowsContextAndUser(t *testing.T) {
	h := plain(headerView(80, "github.com/o/r ⎇ main", "kevin"))
	for _, want := range []string{"gitui", "github.com/o/r ⎇ main", "● connecté : @kevin"} {
		if !strings.Contains(h, want) {
			t.Errorf("header %q lacks %q", h, want)
		}
	}
	if w := lipgloss.Width(h); w != 80 {
		t.Errorf("header width = %d, want 80", w)
	}
}

func TestHeaderTruncatesContextBeforeUser(t *testing.T) {
	h := plain(headerView(40, strings.Repeat("x", 100), "kevin"))
	if !strings.Contains(h, "@kevin") || lipgloss.Width(h) > 40 {
		t.Errorf("header %q (width %d) must keep the user and fit 40 cells", h, lipgloss.Width(h))
	}
}

func TestTabsViewNumbersAndCounts(t *testing.T) {
	got := plain(tabsView([]tab{{"Code", -1}, {"Pull requests", 12}, {"Issues", 48}}, 1))
	for _, want := range []string{"1 Code", "2 Pull requests 12", "3 Issues 48"} {
		if !strings.Contains(got, want) {
			t.Errorf("tabs %q lack %q", got, want)
		}
	}
}

func bindings(pairs ...string) []key.Binding {
	var out []key.Binding
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, key.NewBinding(key.WithKeys(pairs[i]), key.WithHelp(pairs[i], pairs[i+1])))
	}
	return out
}

func TestHelpBarDropsMiddleItemsButKeepsTail(t *testing.T) {
	bs := bindings("j", "naviguer", "enter", "ouvrir", "o", "navigateur", "?", "aide", "q", "quitter")
	full := plain(helpBar(bs, 200, false, 2))
	if !strings.Contains(full, "navigateur") {
		t.Fatalf("wide bar lost an item: %q", full)
	}
	narrow := plain(helpBar(bs, 30, false, 2))
	if lipgloss.Width(narrow) > 30 || !strings.Contains(narrow, "aide") || !strings.Contains(narrow, "quitter") {
		t.Errorf("narrow bar %q must fit 30 cells and keep help+quit", narrow)
	}
}

func TestHelpBarFullWraps(t *testing.T) {
	bs := bindings("j", "naviguer", "enter", "ouvrir", "o", "navigateur", "?", "aide", "q", "quitter")
	out := plain(helpBar(bs, 30, true, 2))
	if !strings.Contains(out, "\n") {
		t.Errorf("full help should wrap: %q", out)
	}
	for _, l := range strings.Split(out, "\n") {
		if lipgloss.Width(l) > 30 {
			t.Errorf("line %q wider than 30", l)
		}
	}
}

func TestPaneIsExactSizeAndClips(t *testing.T) {
	long := strings.Repeat("a", 200) + "\n" + strings.Repeat("l\n", 50)
	out := pane(long, 30, 8, true)
	if lipgloss.Width(out) != 30 || lipgloss.Height(out) != 8 {
		t.Errorf("pane = %dx%d, want 30x8", lipgloss.Width(out), lipgloss.Height(out))
	}
}

func TestSplitViewWidthsAddUp(t *testing.T) {
	for _, w := range []int{90, 110, 160} {
		out := splitView("left", "right", w, 10, w*2/5, true)
		if lipgloss.Width(out) != w || lipgloss.Height(out) != 10 {
			t.Errorf("w=%d: split = %dx%d", w, lipgloss.Width(out), lipgloss.Height(out))
		}
	}
	if canSplit(89) || !canSplit(90) {
		t.Error("canSplit threshold should be 90")
	}
}
