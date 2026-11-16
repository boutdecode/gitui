package tui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider"
)

type prItem struct{ pr domain.PullRequest }

func (i prItem) FilterValue() string { return i.pr.Title }
func (i prItem) Title() string       { return fmt.Sprintf("#%d %s", i.pr.Number, i.pr.Title) }
func (i prItem) Description() string {
	d := fmt.Sprintf("%s · %s", i.pr.Author, i.pr.State)
	if i.pr.Draft {
		d += " (brouillon)"
	}
	if i.pr.Head != "" && i.pr.Base != "" {
		d = fmt.Sprintf("%s → %s · %s", i.pr.Head, i.pr.Base, d)
	}
	return d
}

type prsLoadedMsg struct {
	gen int // load generation, to drop results of a superseded filter
	prs []domain.PullRequest
}

var scopes = []domain.PRScope{
	domain.ScopeAll, domain.ScopeReviewRequested, domain.ScopeAssigned, domain.ScopeAuthored,
}

var scopeLabels = map[domain.PRScope]string{
	domain.ScopeAll:             "toutes",
	domain.ScopeReviewRequested: "à relire",
	domain.ScopeAssigned:        "assignées",
	domain.ScopeAuthored:        "les miennes",
}

type prsScreen struct {
	p       provider.Provider
	repo    domain.Repo
	list    list.Model
	spinner spinner.Model
	loading bool
	scope   domain.PRScope
	gen     int
	filter  key.Binding
}

func newPRsScreen(p provider.Provider, repo domain.Repo) *prsScreen {
	l := newListModel()
	return &prsScreen{p: p, repo: repo, list: l, loading: true, scope: domain.ScopeAll,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		filter:  bind("filter")}
}

func (s *prsScreen) Title() string {
	return fmt.Sprintf("%s › Pull requests (%s)", s.repo.FullName(), scopeLabels[s.scope])
}

func (s *prsScreen) Init() tea.Cmd {
	gen, scope := s.gen, s.scope
	load := func() tea.Msg {
		prs, err := s.p.ListPullRequests(context.Background(), s.repo,
			domain.PRQuery{State: domain.StateOpen, Scope: scope})
		if err != nil {
			return errMsg{err}
		}
		return prsLoadedMsg{gen, prs}
	}
	return tea.Batch(load, s.spinner.Tick)
}

// nextScope cycles the filter and reloads.
func (s *prsScreen) nextScope() tea.Cmd {
	for i, sc := range scopes {
		if sc == s.scope {
			s.scope = scopes[(i+1)%len(scopes)]
			break
		}
	}
	s.gen++
	s.loading = true
	s.list.SetItems(nil)
	return s.Init()
}

func (s *prsScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case prsLoadedMsg:
		if msg.gen != s.gen {
			return s, nil
		}
		s.loading = false
		items := make([]list.Item, len(msg.prs))
		for i, pr := range msg.prs {
			items[i] = prItem{pr}
		}
		return s, s.list.SetItems(items)
	case errMsg:
		s.loading = false
	case spinner.TickMsg:
		if !s.loading {
			return s, nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd
	case tea.KeyPressMsg:
		if s.list.FilterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, s.filter):
			return s, s.nextScope()
		case key.Matches(msg, keySwitch):
			return s, replace(newIssuesScreen(s.p, s.repo))
		case key.Matches(msg, keyOpenItem):
			if it, ok := s.list.SelectedItem().(prItem); ok {
				return s, push(newDetailScreen(s.p, s.repo, prTarget(it.pr)))
			}
		case key.Matches(msg, keyBrowser):
			if it, ok := s.list.SelectedItem().(prItem); ok {
				return s, openInBrowser(it.pr.URL)
			}
		}
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s *prsScreen) View() string {
	if s.loading {
		return s.spinner.View() + " chargement des pull requests…"
	}
	return s.list.View()
}

func (s *prsScreen) SetSize(w, h int) { s.list.SetSize(w, h) }
func (s *prsScreen) Capturing() bool  { return s.list.FilterState() == list.Filtering }
func (s *prsScreen) Keys() []key.Binding {
	return []key.Binding{keyOpenItem, s.filter, keySwitch, keyBrowser, keySearch}
}

func (s *prsScreen) Restyle(dark bool) { restyleList(&s.list, dark) }
