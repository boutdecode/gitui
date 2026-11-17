package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/i18n"
)

var issueStates = []struct {
	label string
	state domain.State
}{
	{"state|open", domain.StateOpen},
	{"state|closed", domain.StateClosed},
	{"state|any", ""},
}

type issueItem struct{ i domain.Issue }

func (it issueItem) FilterValue() string {
	return it.i.Title + " " + it.i.Author + " " + strings.Join(it.i.Labels, " ")
}

// Row is two lines: the title, then author · age · labels.
func (it issueItem) Row(w int, sel bool) string {
	first := marker(sel) + " " + fit(fmt.Sprintf("#%d %s", it.i.Number, it.i.Title), max(w-2, 1))
	meta := "  " + dimStyle.Render("@"+it.i.Author+" · "+ilYa(it.i.UpdatedAt))
	for _, l := range it.i.Labels {
		meta += " " + chip(l)
	}
	if sel {
		return selectLine(first, w, true) + "\n" + fit(meta, w)
	}
	return fit(first, w) + "\n" + fit(meta, w)
}

type issuesLoadedMsg struct {
	gen    int
	issues []domain.Issue
}

type issueCommentsMsg struct {
	number   int
	comments []domain.Comment
	err      error
}

type issueSelTick struct{ number int }

type issuesScreen struct {
	c       *repoCtx
	list    list.Model
	spinner spinner.Model
	loading bool

	stateIdx int
	gen      int

	comments map[int][]domain.Comment
	pending  map[int]bool
	lastSel  int

	w, h int

	filter key.Binding
}

func newIssuesScreen(c *repoCtx) *issuesScreen {
	return &issuesScreen{c: c, list: newRowList(2, 1), loading: true,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		comments: map[int][]domain.Comment{}, pending: map[int]bool{},
		filter: bind("filter")}
}

func (s *issuesScreen) Title() string  { return s.c.repo.FullName() + " › Issues" }
func (s *issuesScreen) Header() string { return s.c.header() }
func (s *issuesScreen) Tabs() ([]tab, int) {
	return s.c.tabs(), tabIssues
}
func (s *issuesScreen) SwitchTab(i int) tea.Cmd { return s.c.switchTo(i) }

func (s *issuesScreen) Init() tea.Cmd {
	gen, state := s.gen, issueStates[s.stateIdx].state
	load := func() tea.Msg {
		is, err := s.c.p.ListIssues(context.Background(), s.c.repo, state)
		if err != nil {
			return errMsg{err}
		}
		return issuesLoadedMsg{gen, is}
	}
	return tea.Batch(load, s.spinner.Tick, s.c.loadCounts())
}

func (s *issuesScreen) reload() tea.Cmd {
	s.gen++
	s.loading = true
	s.list.SetItems(nil)
	return s.Init()
}

func (s *issuesScreen) selected() (domain.Issue, bool) {
	it, ok := s.list.SelectedItem().(issueItem)
	return it.i, ok
}

func (s *issuesScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if applyCounts(msg) {
		return s, nil
	}
	cmd := s.update(msg)
	if is, ok := s.selected(); ok && is.Number != s.lastSel {
		s.lastSel = is.Number
		n := is.Number
		cmd = tea.Batch(cmd, tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg { return issueSelTick{n} }))
	}
	return s, cmd
}

func (s *issuesScreen) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case issuesLoadedMsg:
		if msg.gen != s.gen {
			return nil
		}
		s.loading = false
		items := make([]list.Item, len(msg.issues))
		for i, is := range msg.issues {
			items[i] = issueItem{is}
		}
		return s.list.SetItems(items)
	case issueSelTick:
		is, ok := s.selected()
		if !ok || is.Number != msg.number {
			return nil
		}
		if _, have := s.comments[is.Number]; have || s.pending[is.Number] {
			return nil
		}
		s.pending[is.Number] = true
		p, repo := s.c.p, s.c.repo
		return func() tea.Msg {
			cs, err := p.ListComments(context.Background(), repo, is.Number)
			return issueCommentsMsg{is.Number, cs, err}
		}
	case issueCommentsMsg:
		delete(s.pending, msg.number)
		if msg.err == nil {
			s.comments[msg.number] = msg.comments
		}
		return nil
	case errMsg:
		s.loading = false
	case spinner.TickMsg:
		if !s.loading {
			return nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return cmd
	case tea.KeyPressMsg:
		if s.list.FilterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, s.filter):
			s.stateIdx = (s.stateIdx + 1) % len(issueStates)
			return s.reload()
		case key.Matches(msg, keyRefresh):
			s.comments = map[int][]domain.Comment{}
			s.c.prs, s.c.issues = -1, -1
			return s.reload()
		case key.Matches(msg, keyOpenItem):
			if is, ok := s.selected(); ok {
				return push(newDetailScreen(s.c.p, s.c.repo, issueTarget(is)))
			}
		case key.Matches(msg, keyBrowser):
			if is, ok := s.selected(); ok {
				return openInBrowser(is.URL)
			}
		}
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return cmd
}

func (s *issuesScreen) SetSize(w, h int) {
	s.w, s.h = w, h
	lw, _ := s.widths()
	s.list.SetSize(max(lw-4, 1), max(h-toolbarHeight-2, 1))
}

func (s *issuesScreen) widths() (int, int) {
	if !canSplit(s.w) {
		return s.w, 0
	}
	return splitWidths(s.w, 48)
}

func (s *issuesScreen) View() string {
	bar := toolbar(s.w, s.list, i18n.T("Filter issues…"), pill(i18n.T("state"), i18n.T(issueStates[s.stateIdx].label)))
	lw, rw := s.widths()
	ph := max(s.h-toolbarHeight, 3)

	var left string
	switch {
	case s.loading:
		left = s.spinner.View() + " " + i18n.T("loading issues…")
	case len(s.list.Items()) == 0:
		left = dimStyle.Render(i18n.T("No issue."))
	default:
		left = s.list.View()
	}
	if rw == 0 {
		return bar + "\n" + pane(left, lw, ph, true)
	}
	return bar + "\n" + lipgloss.JoinHorizontal(lipgloss.Top,
		pane(left, lw, ph, true), pane(s.preview(rw-4), rw, ph, false))
}

func (s *issuesScreen) preview(w int) string {
	is, ok := s.selected()
	if !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString(pinkStyle.Bold(true).Render(fmt.Sprintf("#%d", is.Number)) + " " + titleStyle.Render(is.Title) + "\n")
	badge := openBadge.Render("◉ " + i18n.T("Open"))
	if is.State == domain.StateClosed {
		badge = closedBadge.Render("◉ " + i18n.T("Closed"))
	}
	b.WriteString(badge + dimStyle.Render("  @"+is.Author) + "\n")
	if len(is.Labels) > 0 {
		chips := make([]string, len(is.Labels))
		for i, l := range is.Labels {
			chips[i] = chip(l)
		}
		b.WriteString(strings.Join(chips, " ") + "\n")
	}
	b.WriteString(rule(w, "") + "\n")
	if strings.TrimSpace(is.Body) == "" {
		b.WriteString(dimStyle.Render(i18n.T("(no description)")) + "\n")
	} else {
		b.WriteString(styleBody(is.Body, w) + "\n")
	}
	if cs, ok := s.comments[is.Number]; ok && len(cs) > 0 {
		b.WriteString(rule(w, i18n.T("Comments (%d)", len(cs))) + "\n")
		for _, c := range cs {
			b.WriteString(blueStyle.Render("@"+c.Author) + dimStyle.Render(" · "+ilYa(c.CreatedAt)) + "\n")
			b.WriteString(styleBody(c.Body, w) + "\n")
		}
	}
	return b.String()
}

func (s *issuesScreen) Capturing() bool { return s.list.FilterState() == list.Filtering }
func (s *issuesScreen) Keys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("j"), key.WithHelp("j/k", i18n.T("navigate"))),
		keyOpenItem, s.filter, keySearch, keySwitch, keyBrowser, keyRefresh,
	}
}

func (s *issuesScreen) Restyle(dark bool) { restyleList(&s.list, dark) }
