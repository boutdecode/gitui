package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider/fake"
)

// run feeds msg to the model and then drains the resulting commands
// (skipping ticks/blocking cmds) so async loads complete deterministically.
func run(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	queue := []tea.Msg{msg}
	for i := 0; i < 20 && len(queue) > 0; i++ {
		next := queue[0]
		queue = queue[1:]
		var cmd tea.Cmd
		var mm tea.Model
		mm, cmd = m.Update(next)
		m = mm.(Model)
		if cmd == nil {
			continue
		}
		switch out := cmd().(type) {
		case tea.BatchMsg:
			for _, c := range out {
				if c == nil {
					continue
				}
				done := make(chan tea.Msg, 1)
				go func() { done <- c() }()
				select {
				case r := <-done:
					queue = append(queue, r)
				default:
					// likely a spinner tick; ignore
				}
			}
		case nil:
		default:
			queue = append(queue, out)
		}
	}
	return m
}

func view(m Model) string { return m.View().Content }

func TestNavigation(t *testing.T) {
	m := New(Options{Provider: fake.New()})
	m = run(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	// initial load
	m = run(t, m, reposLoadedMsg{tab: repoTabMine, repos: mustRepos(t)})
	if !strings.Contains(view(m), "octocat/hello-world") {
		t.Fatalf("repos not rendered:\n%s", view(m))
	}
	m = run(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.stack) != 2 {
		t.Fatalf("expected 2 screens, got %d", len(m.stack))
	}
	if _, ok := m.top().(*codeScreen); !ok {
		t.Fatalf("enter on a repo should open the code tab, got %T", m.top())
	}
	if v := view(m); !strings.Contains(v, "Pull requests") || !strings.Contains(v, "octocat/hello-world") {
		t.Fatalf("tab bar or header missing:\n%s", v)
	}
	m = run(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(m.stack) != 1 {
		t.Fatalf("esc should pop, stack=%d", len(m.stack))
	}
}

func TestErrorShownAndCleared(t *testing.T) {
	m := New(Options{Provider: fake.New()})
	m = run(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = run(t, m, errMsg{errors.New("boom")})
	if !strings.Contains(view(m), "boom") {
		t.Fatal("error not displayed")
	}
	m = run(t, m, tea.KeyPressMsg{Code: 'x', Text: "x"})
	if strings.Contains(view(m), "boom") {
		t.Fatal("error should clear on keypress")
	}
}

func mustRepos(t *testing.T) []domain.Repo {
	t.Helper()
	r, err := fake.New().ListRepos(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return r
}
