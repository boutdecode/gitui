package tui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider/fake"
)

func resetKeys(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if err := SetKeyOverrides(nil); err != nil {
			t.Fatal(err)
		}
	})
}

func TestKeyOverrideRebindsAction(t *testing.T) {
	resetKeys(t)
	if err := SetKeyOverrides(map[string][]string{"merge": {"M"}}); err != nil {
		t.Fatal(err)
	}
	s, p := newPRDetail(t)
	// the help bar shows the new key
	found := false
	for _, b := range s.Keys() {
		if b.Help().Key == "M" && b.Help().Desc == "merger" {
			found = true
		}
	}
	if !found {
		t.Fatal("help should advertise the overridden key")
	}
	press(t, s, keyOf('m', "m"))
	if s.Capturing() {
		t.Fatal("old key m must no longer merge")
	}
	press(t, s, keyOf('M', "M"))
	if !s.Capturing() {
		t.Fatal("new key M should open the merge prompt")
	}
	_, cmd := press(t, s, keyOf('y', "y"))
	exec(t, s, cmd)
	if len(p.calls) != 1 || p.calls[0] != "merge" {
		t.Fatalf("calls = %v", p.calls)
	}
}

func TestKeyOverrideValidation(t *testing.T) {
	resetKeys(t)
	cases := map[string]map[string][]string{
		"inconnue":    {"explode": {"x"}},
		"vide":        {"merge": {}},
		"touche vide": {"merge": {" "}},
		"conflit":     {"merge": {"a"}},   // approve is "a" on the same screen
		"global":      {"comment": {"q"}}, // shadowed by quit
	}
	for name, o := range cases {
		err := SetKeyOverrides(o)
		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		if !strings.Contains(err.Error(), "raccourcis invalides") {
			t.Errorf("%s: unexpected error %v", name, err)
		}
	}
	// failed validation must not leave partial state behind
	if len(overrides) != 0 {
		t.Fatalf("overrides leaked: %v", overrides)
	}
}

func TestSwappingKeysIsNotAConflict(t *testing.T) {
	resetKeys(t)
	if err := SetKeyOverrides(map[string][]string{"approve": {"m"}, "merge": {"a"}}); err != nil {
		t.Fatalf("swap should be valid: %v", err)
	}
}

func TestCtrlCAlwaysQuits(t *testing.T) {
	resetKeys(t)
	if err := SetKeyOverrides(map[string][]string{"quit": {"x"}}); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Provider: fake.New()})
	m = run(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c must quit even when quit is rebound")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("got %T", cmd())
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("q was rebound and must not quit anymore")
		}
	}
}

func TestSearchOverrideReachesList(t *testing.T) {
	resetKeys(t)
	if err := SetKeyOverrides(map[string][]string{"search": {"s"}}); err != nil {
		t.Fatal(err)
	}
	l := newListModel()
	if got := l.KeyMap.Filter.Keys(); len(got) != 1 || got[0] != "s" {
		t.Fatalf("list filter keys = %v", got)
	}
}

func TestThemeLightDark(t *testing.T) {
	t.Cleanup(func() { applyTheme(true) })

	applyTheme(true)
	dark := diffAdd.Render("+x")
	applyTheme(false)
	light := diffAdd.Render("+x")
	if dark == light {
		t.Fatal("light and dark themes should differ")
	}
	if isDark {
		t.Fatal("isDark should be false after light theme")
	}
}

func TestExplicitThemeIgnoresTerminalBackground(t *testing.T) {
	t.Cleanup(func() { applyTheme(true) })
	m := New(Options{Provider: fake.New(), Theme: "dark"})
	m2, _ := m.Update(tea.BackgroundColorMsg{Color: color.White})
	_ = m2
	if !isDark {
		t.Fatal("explicit dark must not follow a light terminal")
	}
}

func TestAutoThemeFollowsTerminalBackground(t *testing.T) {
	t.Cleanup(func() { applyTheme(true) })
	m := New(Options{Provider: fake.New(), Theme: "auto"})
	if cmd := m.Init(); cmd == nil {
		t.Fatal("auto theme must request the background color")
	}
	m.Update(tea.BackgroundColorMsg{Color: color.White})
	if isDark {
		t.Fatal("light terminal should switch to the light theme")
	}
	m.Update(tea.BackgroundColorMsg{Color: color.Black})
	if !isDark {
		t.Fatal("dark terminal should switch back to the dark theme")
	}
}

func TestRestyleReachesScreens(t *testing.T) {
	t.Cleanup(func() { applyTheme(true) })
	m := New(Options{Provider: fake.New(), Theme: "dark"})
	m = run(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m.stack = append(m.stack, newDetailScreen(fake.New(), domain.Repo{}, target{number: 1}))
	m.setTheme(false) // must not panic on list/detail screens
	if isDark {
		t.Fatal("theme not applied")
	}
	m.setTheme(true)
}
