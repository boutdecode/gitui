package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kevin/gitui/internal/provider/fake"
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

func TestBackgroundPaintedUnlessTransparent(t *testing.T) {
	t.Cleanup(func() { applyTheme(true) })
	p := fake.New()
	bg := sgr(48, colBg)

	for _, theme := range []string{"dark", "auto"} {
		m := New(Options{Provider: p, Theme: theme})
		m.width, m.height = 60, 10
		v := m.View().Content
		if !strings.Contains(v, bg) {
			t.Fatalf("theme %q: background not painted", theme)
		}
		lines := strings.Split(v, "\n")
		if len(lines) != 10 {
			t.Fatalf("theme %q: %d lines, want 10", theme, len(lines))
		}
		for i, l := range lines {
			if !strings.HasPrefix(l, sgr(48, colBg)) || lipgloss.Width(l) != 60 {
				t.Fatalf("theme %q line %d: not a full painted row (width %d)", theme, i, lipgloss.Width(l))
			}
		}
	}

	m := New(Options{Provider: p, Theme: "dark", Transparent: true})
	m.width, m.height = 60, 10
	if strings.Contains(m.View().Content, bg) {
		t.Fatal("--transparent must not paint the background")
	}
}

func TestPaintKeepsBackgroundAfterStyleResets(t *testing.T) {
	t.Cleanup(func() { applyTheme(true) })
	out := paintBackground("a\x1b[31mb\x1b[0mc", 10, 1)
	if strings.Count(out, sgr(48, colBg)) < 2 {
		t.Fatalf("background must be re-applied after a reset: %q", out)
	}
}

func TestMarginKeepsAppAwayFromEdges(t *testing.T) {
	t.Cleanup(func() { applyTheme(true) })
	m := New(Options{Provider: fake.New(), Theme: "dark", Transparent: true})
	m.width, m.height = 100, 30
	m.layout()
	lines := strings.Split(plain(m.View().Content), "\n")
	if len(lines) != 30 {
		t.Fatalf("frame has %d lines, want 30", len(lines))
	}
	if strings.TrimSpace(lines[0]) != "" || strings.TrimSpace(lines[29]) != "" {
		t.Fatalf("first and last rows must be blank margins: %q / %q", lines[0], lines[29])
	}
	for i, l := range lines {
		if lipgloss.Width(l) > 100-marginX {
			t.Errorf("line %d reaches the right edge (%d cells)", i, lipgloss.Width(l))
		}
		if l != "" && !strings.HasPrefix(l, strings.Repeat(" ", marginX)) {
			t.Errorf("line %d touches the left edge: %q", i, l)
		}
	}
	// tiny terminals keep every cell for the app
	m.width, m.height = 50, 12
	if m.hasMargin() {
		t.Fatal("no margin on a tiny terminal")
	}
}

func TestTabAndShiftTabCycleTabs(t *testing.T) {
	m := New(Options{Provider: fake.New()})
	m = run(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = run(t, m, reposLoadedMsg{tab: repoTabMine, repos: mustRepos(t)})
	m = run(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}) // Code tab
	steps := []struct {
		k    tea.KeyPressMsg
		want any
	}{
		{tea.KeyPressMsg{Code: tea.KeyTab}, &prsScreen{}},
		{tea.KeyPressMsg{Code: tea.KeyTab}, &issuesScreen{}},
		{tea.KeyPressMsg{Code: tea.KeyTab}, &codeScreen{}},
		{tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, &issuesScreen{}},
		{tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, &prsScreen{}},
	}
	for i, st := range steps {
		m = run(t, m, st.k)
		if fmt.Sprintf("%T", m.top()) != fmt.Sprintf("%T", st.want) {
			t.Fatalf("step %d: on %T, want %T", i, m.top(), st.want)
		}
	}
}
