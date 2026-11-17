package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider/fake"
)

type recordingProvider struct {
	*fake.Provider
	scopes []domain.PRScope
}

func (r *recordingProvider) ListPullRequests(ctx context.Context, repo domain.Repo, q domain.PRQuery) ([]domain.PullRequest, error) {
	r.scopes = append(r.scopes, q.Scope)
	return r.Provider.ListPullRequests(ctx, repo, q)
}

func newTestPRs(p *recordingProvider) *prsScreen {
	s := newPRsScreen(newRepoCtx(p, domain.Repo{Owner: "o", Name: "r"}))
	s.SetSize(120, 30)
	return s
}

// firstBatchCmd returns the list-loading command of a Batch.
func loadMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	b, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("expected a batch")
	}
	return b[0]()
}

func TestPRScopeCycleReloads(t *testing.T) {
	p := &recordingProvider{Provider: fake.New()}
	s := newTestPRs(p)

	_, _ = s.Update(loadMsg(t, s.Init()))
	if got := p.scopes; len(got) != 1 || got[0] != domain.ScopeAll {
		t.Fatalf("initial scopes = %v", got)
	}

	_, cmd := s.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if s.scope != domain.ScopeReviewRequested || !s.loading {
		t.Fatalf("scope=%v loading=%v", s.scope, s.loading)
	}
	loaded := loadMsg(t, cmd)
	if p.scopes[len(p.scopes)-1] != domain.ScopeReviewRequested {
		t.Fatalf("provider saw %v", p.scopes)
	}
	if !strings.Contains(s.Title(), "à relire") {
		t.Fatalf("title = %q", s.Title())
	}

	// a result from the previous filter arriving late must be dropped
	_, _ = s.Update(prsLoadedMsg{gen: s.gen - 1})
	if !s.loading {
		t.Fatal("stale result should be ignored")
	}
	_, _ = s.Update(loaded)
	if s.loading {
		t.Fatal("current result should be applied")
	}
}

func TestPRStateFilterCycles(t *testing.T) {
	p := &recordingProvider{Provider: fake.New()}
	s := newTestPRs(p)
	var states []domain.State
	s.c.p = &stateProvider{recordingProvider: p, states: &states}
	for i := 0; i < len(prStates); i++ {
		_, cmd := s.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
		_ = loadMsg(t, cmd)
	}
	want := []domain.State{domain.StateClosed, domain.StateMerged, "", domain.StateOpen}
	if len(states) != len(want) {
		t.Fatalf("states = %v, want %v", states, want)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Fatalf("states = %v, want %v", states, want)
		}
	}
}

type stateProvider struct {
	*recordingProvider
	states *[]domain.State
}

func (s *stateProvider) ListPullRequests(ctx context.Context, repo domain.Repo, q domain.PRQuery) ([]domain.PullRequest, error) {
	*s.states = append(*s.states, q.State)
	return s.recordingProvider.ListPullRequests(ctx, repo, q)
}

func TestPRPreviewLoadsDetailsOnSelection(t *testing.T) {
	s := newTestPRs(&recordingProvider{Provider: fake.New()})
	prs, _ := fake.New().ListPullRequests(context.Background(), testRepo, domain.PRQuery{})
	_, _ = s.Update(prsLoadedMsg{gen: 0, prs: prs})
	if v := plain(s.View()); !strings.Contains(v, "Add feature") || !strings.Contains(v, "chargement des checks") {
		t.Fatalf("preview should show the PR and a loading hint:\n%s", v)
	}
	_, cmd := s.Update(prSelTick{number: prs[0].Number})
	msg := cmd()
	_, _ = s.Update(msg)
	v := plain(s.View())
	for _, want := range []string{"build (ubuntu)", "renderer.go", "+84", "✓ CI"} {
		if !strings.Contains(v, want) {
			t.Errorf("preview lacks %q:\n%s", want, v)
		}
	}
}

func TestQuotaTextNoLimiter(t *testing.T) {
	m := New(Options{Provider: fake.New()})
	if got := strings.TrimSpace(m.quotaText()); got != "" {
		t.Fatalf("fake provider has no quota, got %q", got)
	}
	m.p = nil
	_ = m.quotaText() // must not panic before login
}
