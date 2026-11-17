package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider"
	"github.com/kevin/gitui/internal/recents"
)

// Tabs of the repository list.
const (
	repoTabMine = iota
	repoTabStarred
	repoTabOrgs
	repoTabRecent
	repoTabCount
)

var repoTabLabels = [repoTabCount]string{"Mes dépôts", "Favoris", "Organisations", "Récents"}

var repoFilters = []string{"tous", "publics", "privés", "forks", "archivés"}

var repoSorts = []string{"activité", "nom", "étoiles"}

func repoMatches(r domain.Repo, filter int) bool {
	switch filter {
	case 1:
		return !r.Private
	case 2:
		return r.Private
	case 3:
		return r.Fork
	case 4:
		return r.Archived
	}
	return true
}

type repoItem struct{ r domain.Repo }

func (i repoItem) FilterValue() string { return i.r.FullName() + " " + i.r.Description }

// Column widths of the repository table (name takes the rest).
const (
	colLang  = 7
	colStars = 6
	colWhen  = 7
)

func repoNameWidth(w int) int { return max(w-2-(colLang+colStars+colWhen+3), 10) }

func (i repoItem) Row(w int, sel bool) string {
	r := i.r
	name := r.FullName()
	if r.Private {
		name += " ⚿"
	}
	if r.Fork {
		name += " ⑂"
	}
	if r.Archived {
		name += " [archivé]"
	}
	if r.Starred {
		name += " ♥"
	}
	line := marker(sel) + " " + fit(name, repoNameWidth(w)) + " " + fit(r.Language, colLang) + " " +
		fitRight(humanCount(r.Stars), colStars) + " " + fitRight(since(r.UpdatedAt), colWhen)
	switch {
	case sel:
		return selectLine(line, w, true)
	case r.Fork || r.Archived:
		return dimStyle.Render(line)
	}
	return line
}

func repoHeader(w int) string {
	return dimStyle.Render("  " + fit("NOM", repoNameWidth(w)) + " " + fit("LANG", colLang) + " " +
		fitRight("★", colStars) + " " + fitRight("MAJ", colWhen))
}

type (
	reposLoadedMsg struct {
		tab   int
		repos []domain.Repo
	}
	repoCountsMsg struct {
		name        string
		prs, issues int
	}
	selTickMsg  struct{ name string }
	starDoneMsg struct {
		repo domain.Repo
		on   bool
	}
)

type reposScreen struct {
	p       provider.Provider
	recents *recents.Store
	list    list.Model
	spinner spinner.Model

	tab     int
	data    [repoTabCount][]domain.Repo
	loaded  [repoTabCount]bool
	loading [repoTabCount]bool

	filter, sortBy int
	w, h           int

	counts  map[string][2]int // repo -> open PRs, open issues
	pending map[string]bool
	lastSel string

	open, filterKey, star, sortKey key.Binding
}

func newReposScreen(p provider.Provider, rec *recents.Store) *reposScreen {
	return &reposScreen{
		p: p, recents: rec, list: newRowList(1, 0),
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		counts:  map[string][2]int{}, pending: map[string]bool{},
		open: keyOpenItem, filterKey: bind("filter"), star: bind("star"), sortKey: bind("sort"),
	}
}

func (s *reposScreen) Title() string { return "Dépôts" }

func (s *reposScreen) Header() string {
	name := s.p.Name()
	if name == "github" {
		name = "GitHub"
	}
	return name + dimStyle.Render(" · Dépôts")
}

func (s *reposScreen) Tabs() ([]tab, int) {
	tabs := make([]tab, repoTabCount)
	for i := range tabs {
		tabs[i] = tab{Label: repoTabLabels[i], Count: -1}
		if s.loaded[i] && i != repoTabRecent {
			tabs[i].Count = len(s.data[i])
		}
	}
	return tabs, s.tab
}

func (s *reposScreen) SwitchTab(i int) tea.Cmd {
	s.tab = i
	s.list.ResetFilter()
	s.rebuild()
	return s.load(i)
}

func (s *reposScreen) Init() tea.Cmd { return s.load(s.tab) }

// load fetches a tab's repositories (Récents is local and always re-read).
func (s *reposScreen) load(tab int) tea.Cmd {
	if tab == repoTabRecent {
		if s.recents != nil {
			s.data[tab] = s.recents.List()
		}
		s.loaded[tab] = true
		s.rebuild()
		return nil
	}
	if s.loaded[tab] || s.loading[tab] {
		return nil
	}
	s.loading[tab] = true
	p := s.p
	fetch := func() tea.Msg {
		ctx := context.Background()
		var (
			repos []domain.Repo
			err   error
		)
		switch tab {
		case repoTabMine:
			repos, err = p.ListRepos(ctx)
		case repoTabStarred:
			repos, err = p.ListStarred(ctx)
		case repoTabOrgs:
			repos, err = listAllOrgRepos(ctx, p)
		}
		if err != nil {
			return failedLoadMsg{tab, err}
		}
		return reposLoadedMsg{tab, repos}
	}
	return tea.Batch(fetch, s.spinner.Tick)
}

type failedLoadMsg struct {
	tab int
	err error
}

func listAllOrgRepos(ctx context.Context, p provider.Provider) ([]domain.Repo, error) {
	orgs, err := p.ListOrgs(ctx)
	if err != nil {
		return nil, err
	}
	var all []domain.Repo
	for _, o := range orgs {
		rs, err := p.ListOrgRepos(ctx, o.Login)
		if err != nil {
			return nil, err
		}
		all = append(all, rs...)
	}
	return all, nil
}

// rebuild applies the visibility filter and sort to the active tab.
func (s *reposScreen) rebuild() {
	var vis []domain.Repo
	for _, r := range s.data[s.tab] {
		if repoMatches(r, s.filter) {
			vis = append(vis, r)
		}
	}
	if s.tab != repoTabRecent {
		sort.SliceStable(vis, func(i, j int) bool {
			switch s.sortBy {
			case 1:
				return strings.ToLower(vis[i].FullName()) < strings.ToLower(vis[j].FullName())
			case 2:
				return vis[i].Stars > vis[j].Stars
			}
			return vis[i].UpdatedAt.After(vis[j].UpdatedAt)
		})
	}
	items := make([]list.Item, len(vis))
	for i, r := range vis {
		items[i] = repoItem{r}
	}
	s.list.SetItems(items)
}

func (s *reposScreen) selected() (domain.Repo, bool) {
	it, ok := s.list.SelectedItem().(repoItem)
	return it.r, ok
}

func (s *reposScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	cmd := s.update(msg)
	return s, tea.Batch(cmd, s.watchSelection())
}

// watchSelection debounces the per-repository counters: scrolling quickly
// must not fire a search request per row.
func (s *reposScreen) watchSelection() tea.Cmd {
	r, ok := s.selected()
	if !ok || r.FullName() == s.lastSel {
		return nil
	}
	s.lastSel = r.FullName()
	name := s.lastSel
	return tea.Tick(350*time.Millisecond, func(time.Time) tea.Msg { return selTickMsg{name} })
}

func (s *reposScreen) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case reposLoadedMsg:
		s.loading[msg.tab], s.loaded[msg.tab] = false, true
		s.data[msg.tab] = msg.repos
		if msg.tab == s.tab {
			s.rebuild()
		}
		return nil
	case failedLoadMsg:
		s.loading[msg.tab] = false
		return fail(msg.err)
	case errMsg:
		for i := range s.loading {
			s.loading[i] = false
		}
		return nil
	case selTickMsg:
		r, ok := s.selected()
		if !ok || r.FullName() != msg.name || s.pending[msg.name] {
			return nil
		}
		if _, done := s.counts[msg.name]; done {
			return nil
		}
		s.pending[msg.name] = true
		p := s.p
		return func() tea.Msg {
			prs, is, err := p.CountOpen(context.Background(), r)
			if err != nil {
				return repoCountsMsg{msg.name, -1, -1}
			}
			return repoCountsMsg{msg.name, prs, is}
		}
	case repoCountsMsg:
		delete(s.pending, msg.name)
		if msg.prs >= 0 {
			s.counts[msg.name] = [2]int{msg.prs, msg.issues}
		}
		return nil
	case starDoneMsg:
		s.applyStar(msg.repo, msg.on)
		if msg.on {
			return info("ajouté aux favoris : " + msg.repo.FullName())
		}
		return info("retiré des favoris : " + msg.repo.FullName())
	case spinner.TickMsg:
		if !s.anyLoading() {
			return nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return cmd
	case tea.KeyPressMsg:
		if s.list.FilterState() == list.Filtering {
			break
		}
		if cmd, handled := s.handleKey(msg); handled {
			return cmd
		}
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return cmd
}

func (s *reposScreen) anyLoading() bool {
	for _, l := range s.loading {
		if l {
			return true
		}
	}
	return false
}

func (s *reposScreen) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, s.filterKey):
		s.filter = (s.filter + 1) % len(repoFilters)
		s.rebuild()
		return nil, true
	case key.Matches(msg, s.sortKey):
		s.sortBy = (s.sortBy + 1) % len(repoSorts)
		s.rebuild()
		return info("tri : " + repoSorts[s.sortBy]), true
	case key.Matches(msg, keyRefresh):
		s.loaded[s.tab] = false
		s.counts = map[string][2]int{}
		s.lastSel = ""
		return s.load(s.tab), true
	}
	r, ok := s.selected()
	if !ok {
		return nil, false
	}
	switch {
	case key.Matches(msg, s.open):
		return tea.Batch(s.remember(r), push(newCodeScreen(newRepoCtx(s.p, r)))), true
	case key.Matches(msg, keyBrowser):
		return openInBrowser(r.URL), true
	case key.Matches(msg, s.star):
		on, p := !r.Starred, s.p
		return func() tea.Msg {
			if err := p.SetStarred(context.Background(), r, on); err != nil {
				return errMsg{err}
			}
			return starDoneMsg{r, on}
		}, true
	}
	return nil, false
}

// remember records an opened repository; failures are not worth interrupting for.
func (s *reposScreen) remember(r domain.Repo) tea.Cmd {
	if s.recents == nil {
		return nil
	}
	rec := s.recents
	return func() tea.Msg { _ = rec.Add(r); return nil }
}

// applyStar updates the star flag everywhere the repository appears and keeps
// the Favoris tab in sync.
func (s *reposScreen) applyStar(r domain.Repo, on bool) {
	for t := range s.data {
		for i := range s.data[t] {
			if s.data[t][i].FullName() == r.FullName() {
				s.data[t][i].Starred = on
			}
		}
	}
	if s.loaded[repoTabStarred] {
		var kept []domain.Repo
		for _, x := range s.data[repoTabStarred] {
			if x.FullName() != r.FullName() {
				kept = append(kept, x)
			}
		}
		if on {
			r.Starred = true
			kept = append([]domain.Repo{r}, kept...)
		}
		s.data[repoTabStarred] = kept
	}
	idx := s.list.Index()
	s.rebuild()
	s.list.Select(min(idx, max(len(s.list.Items())-1, 0)))
}

func (s *reposScreen) SetSize(w, h int) {
	s.w, s.h = w, h
	lw, _ := s.leftWidth()
	s.list.SetSize(max(lw-4, 1), max(h-1-2-1, 1)) // filter bar, pane border, table header
}

// leftWidth returns the outer widths of the table pane and the detail pane
// (0 when the terminal is too narrow for a preview).
func (s *reposScreen) leftWidth() (int, int) {
	if !canSplit(s.w) {
		return s.w, 0
	}
	return splitWidths(s.w, s.w*56/100)
}

func (s *reposScreen) View() string {
	right := searchBar(s.list)
	bar := filterBar(s.w, repoFilters, s.filter, dimStyle.Render("tri : "+repoSorts[s.sortBy]+"  ")+right)
	lw, rw := s.leftWidth()
	ph := max(s.h-1, 3)

	var table string
	switch {
	case s.loading[s.tab] && !s.loaded[s.tab]:
		table = s.spinner.View() + " chargement des dépôts…"
	case len(s.list.Items()) == 0:
		table = dimStyle.Render(s.emptyText())
	default:
		table = repoHeader(lw-4) + "\n" + s.list.View()
	}
	if rw == 0 {
		return bar + "\n" + pane(table, lw, ph, true)
	}
	return bar + "\n" + lipgloss.JoinHorizontal(lipgloss.Top,
		pane(table, lw, ph, true), pane(s.detail(rw-4), rw, ph, false))
}

func (s *reposScreen) emptyText() string {
	if s.tab == repoTabRecent {
		return "Aucun dépôt ouvert récemment."
	}
	if s.loaded[s.tab] && len(s.data[s.tab]) > 0 {
		return "Aucun dépôt ne correspond au filtre."
	}
	return "Aucun dépôt."
}

// detail is the right-hand summary of the selected repository.
func (s *reposScreen) detail(w int) string {
	r, ok := s.selected()
	if !ok {
		return ""
	}
	val := func(label, v string) string { return dimStyle.Render(fit(label, 11)) + v + "\n" }
	var b strings.Builder
	b.WriteString(pinkStyle.Bold(true).Render(r.FullName()) + "\n")
	b.WriteString(dimStyle.Render(fit(r.Description, w)) + "\n")
	b.WriteString(rule(w, "") + "\n")
	vis := "public"
	if r.Private {
		vis = "privé"
	}
	b.WriteString(val("Visibilité", vis))
	if r.DefaultBranch != "" {
		b.WriteString(val("Branche", purpleStyle.Render(r.DefaultBranch)))
	}
	if r.License != "" {
		b.WriteString(val("Licence", r.License))
	}
	if r.Language != "" {
		b.WriteString(val("Langage", blueStyle.Render(r.Language)))
	}
	b.WriteString(val("Étoiles", yellowStyle.Render("★")+" "+fmt.Sprint(r.Stars)))
	b.WriteString(val("Forks", fmt.Sprint(r.Forks)))
	if len(r.Topics) > 0 {
		b.WriteString(val("Topics", blueStyle.Render(strings.Join(r.Topics, "  "))))
	}
	b.WriteString(rule(w, "Activité") + "\n")
	if c, ok := s.counts[r.FullName()]; ok {
		b.WriteString(greenStyle.Render("●") + fmt.Sprintf(" %d pull requests ouvertes\n", c[0]))
		b.WriteString(greenStyle.Render("●") + fmt.Sprintf(" %d issues ouvertes\n", c[1]))
	} else {
		b.WriteString(dimStyle.Render("…\n"))
	}
	if !r.UpdatedAt.IsZero() {
		b.WriteString(dimStyle.Render("Dernier push  " + ilYa(r.UpdatedAt)))
	}
	b.WriteString("\n\n" + keyCapStyle.Render(s.open.Help().Key) + dimStyle.Render(" ouvrir le dépôt"))
	return b.String()
}

func (s *reposScreen) Capturing() bool { return s.list.FilterState() == list.Filtering }

func (s *reposScreen) Keys() []key.Binding {
	return []key.Binding{s.open, keySearch, s.star, keySwitch, s.sortKey, s.filterKey, keyRefresh, keyBrowser}
}

func (s *reposScreen) Restyle(dark bool) { restyleList(&s.list, dark) }
