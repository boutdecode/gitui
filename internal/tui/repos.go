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

type repoItem struct{ r domain.Repo }

func (i repoItem) FilterValue() string { return i.r.FullName() }
func (i repoItem) Title() string {
	if i.r.Private {
		return i.r.FullName() + " 🔒"
	}
	return i.r.FullName()
}
func (i repoItem) Description() string {
	return fmt.Sprintf("★ %d  %s", i.r.Stars, i.r.Description)
}

type reposLoadedMsg []domain.Repo

type reposScreen struct {
	p       provider.Provider
	list    list.Model
	spinner spinner.Model
	loading bool
	open    key.Binding
}

func newReposScreen(p provider.Provider) *reposScreen {
	l := newListModel()
	return &reposScreen{
		p: p, list: l, loading: true,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		open:    keyOpenItem,
	}
}

func (s *reposScreen) Title() string { return "Dépôts" }

func (s *reposScreen) Init() tea.Cmd {
	load := func() tea.Msg {
		repos, err := s.p.ListRepos(context.Background())
		if err != nil {
			return errMsg{err}
		}
		return reposLoadedMsg(repos)
	}
	return tea.Batch(load, s.spinner.Tick)
}

func (s *reposScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case reposLoadedMsg:
		s.loading = false
		items := make([]list.Item, len(msg))
		for i, r := range msg {
			items[i] = repoItem{r}
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
		if it, ok := s.list.SelectedItem().(repoItem); ok {
			switch {
			case key.Matches(msg, s.open):
				return s, push(newPRsScreen(s.p, it.r))
			case key.Matches(msg, keyBrowser):
				return s, openInBrowser(it.r.URL)
			}
		}
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s *reposScreen) View() string {
	if s.loading {
		return s.spinner.View() + " chargement des dépôts…"
	}
	return s.list.View()
}

func (s *reposScreen) SetSize(w, h int) { s.list.SetSize(w, h) }
func (s *reposScreen) Capturing() bool  { return s.list.FilterState() == list.Filtering }
func (s *reposScreen) Keys() []key.Binding {
	return []key.Binding{s.open, keyBrowser, keySearch}
}

func (s *reposScreen) Restyle(dark bool) { restyleList(&s.list, dark) }
