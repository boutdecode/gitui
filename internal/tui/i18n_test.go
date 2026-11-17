package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/kevin/gitui/internal/i18n"
)

func withLang(t *testing.T, l i18n.Lang) {
	t.Helper()
	prev := i18n.Current()
	i18n.SetLang(l)
	rebuildKeys()
	t.Cleanup(func() { i18n.SetLang(prev); rebuildKeys() })
}

func TestEnglishUI(t *testing.T) {
	withLang(t, i18n.EN)
	if got := keys.Quit.Help().Desc; got != "quit" {
		t.Errorf("quit desc = %q", got)
	}
	if got := ilYa(time.Now().Add(-2 * time.Hour)); got != "2 h ago" {
		t.Errorf("ilYa = %q", got)
	}
	if got := humanBytes(2500); got != "2.5 KB" {
		t.Errorf("humanBytes = %q", got)
	}
	if got := pill("state", "open"); !strings.Contains(got, "state:") {
		t.Errorf("pill = %q", got)
	}
	if got := i18n.T(prStates[0].label); got != "open" {
		t.Errorf("state label = %q", got)
	}
}

func TestFrenchUI(t *testing.T) {
	withLang(t, i18n.FR)
	if got := keys.Quit.Help().Desc; got != "quitter" {
		t.Errorf("quit desc = %q", got)
	}
	if got := ilYa(time.Now().Add(-2 * time.Hour)); got != "il y a 2 h" {
		t.Errorf("ilYa = %q", got)
	}
	if got := i18n.T(prStates[0].label); got != "ouvertes" {
		t.Errorf("state label = %q", got)
	}
}

// Every action description must be translated, or the help bar would mix
// languages.
func TestActionDescriptionsTranslated(t *testing.T) {
	withLang(t, i18n.FR)
	same := map[string]bool{"diff/conversation": true}
	for name, d := range defs {
		if i18n.T(d.desc) == d.desc && !same[d.desc] {
			t.Errorf("action %q: description %q has no French translation", name, d.desc)
		}
	}
}
