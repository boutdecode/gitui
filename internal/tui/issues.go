package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider"
)

type issueItem struct{ i domain.Issue }

func (it issueItem) FilterValue() string { return it.i.Title }
func (it issueItem) Title() string       { return fmt.Sprintf("#%d %s", it.i.Number, it.i.Title) }
func (it issueItem) Description() string {
	d := fmt.Sprintf("%s · %s", it.i.Author, it.i.State)
	if len(it.i.Labels) > 0 {
		d += " · " + strings.Join(it.i.Labels, ", ")
	}
	return d
}

type issuesLoadedMsg []domain.Issue

type issuesScreen struct {
	p       provider.Provider
	repo    domain.Repo
	list    list.Model
	spinner spinner.Model
	loading bool
}

func newIssuesScreen(p provider.Provider, repo domain.Repo) *issuesScreen {
	l := newListModel()
	return &issuesScreen{p: p, repo: repo, list: l, loading: true,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot))}
}

func (s *issuesScreen) Title() string { return s.repo.FullName() + " › Issues" }

func (s *issuesScreen) Init() tea.Cmd {
	load := func() tea.Msg {
		is, err := s.p.ListIssues(context.Background(), s.repo, domain.StateOpen)
		if err != nil {
			return errMsg{err}
		}
		return issuesLoadedMsg(is)
	}
	return tea.Batch(load, s.spinner.Tick)
}

func (s *issuesScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case issuesLoadedMsg:
		s.loading = false
		items := make([]list.Item, len(msg))
		for i, is := range msg {
			items[i] = issueItem{is}
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
		case key.Matches(msg, keySwitch):
			return s, replace(newPRsScreen(s.p, s.repo))
		case key.Matches(msg, keyOpenItem):
			if it, ok := s.list.SelectedItem().(issueItem); ok {
				return s, push(newDetailScreen(s.p, s.repo, issueTarget(it.i)))
			}
		case key.Matches(msg, keyBrowser):
			if it, ok := s.list.SelectedItem().(issueItem); ok {
				return s, openInBrowser(it.i.URL)
			}
		}
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s *issuesScreen) View() string {
	if s.loading {
		return s.spinner.View() + " chargement des issues…"
	}
	return s.list.View()
}

func (s *issuesScreen) SetSize(w, h int) { s.list.SetSize(w, h) }
func (s *issuesScreen) Capturing() bool  { return s.list.FilterState() == list.Filtering }
func (s *issuesScreen) Keys() []key.Binding {
	return []key.Binding{keyOpenItem, keySwitch, keyBrowser, keySearch}
}

func (s *issuesScreen) Restyle(dark bool) { restyleList(&s.list, dark) }
