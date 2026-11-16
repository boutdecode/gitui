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

func TestPRFilterCycleReloads(t *testing.T) {
	p := &recordingProvider{Provider: fake.New()}
	s := newPRsScreen(p, domain.Repo{Owner: "o", Name: "r"})
	s.SetSize(80, 20)

	// initial load
	msg := s.Init()()
	for _, m := range msg.(tea.BatchMsg)[:1] {
		_, _ = s.Update(m())
	}
	if got := p.scopes; len(got) != 1 || got[0] != domain.ScopeAll {
		t.Fatalf("initial scopes = %v", got)
	}

	// pressing f switches to "à relire" and reloads with that scope
	_, cmd := s.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	if s.scope != domain.ScopeReviewRequested || !s.loading {
		t.Fatalf("scope=%v loading=%v", s.scope, s.loading)
	}
	batch := cmd().(tea.BatchMsg)
	loaded := batch[0]()
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

func TestQuotaTextNoLimiter(t *testing.T) {
	m := New(Options{Provider: fake.New()})
	if got := strings.TrimSpace(m.quotaText()); got != "" {
		t.Fatalf("fake provider has no quota, got %q", got)
	}
	m.p = nil
	_ = m.quotaText() // must not panic before login
}
