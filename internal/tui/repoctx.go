package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider"
)

// repoCtx is shared by the three screens of one repository (Code, Pull
// requests, Issues): the tab bar, the header and the open-item counters.
type repoCtx struct {
	p       provider.Provider
	repo    domain.Repo
	branch  string
	prs     int // open pull requests; -1 until loaded
	issues  int // open issues; -1 until loaded
	loading bool
}

func newRepoCtx(p provider.Provider, repo domain.Repo) *repoCtx {
	b := repo.DefaultBranch
	if b == "" {
		b = "main"
	}
	return &repoCtx{p: p, repo: repo, branch: b, prs: -1, issues: -1}
}

const (
	tabCode = iota
	tabPRs
	tabIssues
)

func (c *repoCtx) tabs() []tab {
	return []tab{{"Code", -1}, {"Pull requests", c.prs}, {"Issues", c.issues}}
}

// header is "github.com/owner/repo ⎇ branch".
func (c *repoCtx) header() string {
	host := c.p.Name()
	if host == "github" {
		host = "github.com"
	}
	return host + "/" + c.repo.FullName() + dimStyle.Render("  ⎇ "+c.branch)
}

// switchTo opens tab i in place of the current screen.
func (c *repoCtx) switchTo(i int) tea.Cmd {
	switch i {
	case tabCode:
		return replace(newCodeScreen(c))
	case tabPRs:
		return replace(newPRsScreen(c))
	}
	return replace(newIssuesScreen(c))
}

type countsMsg struct {
	c          *repoCtx
	prs, issue int
}

// loadCounts fetches the open PR and issue counts once per repository.
func (c *repoCtx) loadCounts() tea.Cmd {
	if c.loading || (c.prs >= 0 && c.issues >= 0) {
		return nil
	}
	c.loading = true
	return func() tea.Msg {
		prs, is, err := c.p.CountOpen(context.Background(), c.repo)
		if err != nil {
			return countsMsg{c, -1, -1}
		}
		return countsMsg{c, prs, is}
	}
}

// apply stores a counts result; returns true if the message was ours.
func applyCounts(msg tea.Msg) bool {
	m, ok := msg.(countsMsg)
	if !ok {
		return false
	}
	m.c.loading = false
	if m.prs >= 0 {
		m.c.prs, m.c.issues = m.prs, m.issue
	}
	return true
}
