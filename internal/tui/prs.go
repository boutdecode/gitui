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
)

// prStates are the state filters, in display order. A zero state means all.
var prStates = []struct {
	label string
	state domain.State
}{
	{"ouvertes", domain.StateOpen},
	{"fermées", domain.StateClosed},
	{"fusionnées", domain.StateMerged},
	{"toutes", ""},
}

var scopes = []domain.PRScope{
	domain.ScopeAll, domain.ScopeReviewRequested, domain.ScopeAssigned, domain.ScopeAuthored,
}

var scopeLabels = map[domain.PRScope]string{
	domain.ScopeAll:             "tous",
	domain.ScopeReviewRequested: "à relire",
	domain.ScopeAssigned:        "assignées",
	domain.ScopeAuthored:        "les miennes",
}

// prExtra is what the preview loads on demand for one pull request.
type prExtra struct {
	pr     domain.PullRequest
	files  []domain.PRFile
	checks []domain.Check
	status domain.CheckStatus
}

type prItem struct {
	pr     domain.PullRequest
	status domain.CheckStatus // CheckNone until the details were loaded
	files  int
}

func (i prItem) FilterValue() string { return i.pr.Title + " " + i.pr.Author }

func ciIcon(s domain.CheckStatus) string {
	switch s {
	case domain.CheckSuccess:
		return greenStyle.Render("✓")
	case domain.CheckPending:
		return yellowStyle.Render("●")
	case domain.CheckFailure:
		return redStyle.Render("✗")
	}
	return dimStyle.Render("·")
}

func ciBadge(s domain.CheckStatus) string { return ciIcon(s) + dimStyle.Render(" CI") }

func stateDot(st domain.State) string {
	switch st {
	case domain.StateMerged:
		return purpleStyle.Render("●")
	case domain.StateClosed:
		return redStyle.Render("●")
	}
	return greenStyle.Render("●")
}

// Row is two lines: the title, then author · age · CI · files.
func (i prItem) Row(w int, sel bool) string {
	title := fmt.Sprintf("#%d %s", i.pr.Number, i.pr.Title)
	if i.pr.Draft {
		title = "◌ brouillon  " + title
	}
	first := marker(sel) + " " + fit(title, max(w-2, 1))
	meta := "  " + stateDot(i.pr.State) + dimStyle.Render(" @"+i.pr.Author+" · "+ilYa(i.pr.UpdatedAt)+" · ") + ciBadge(i.status)
	if i.files > 0 {
		meta += dimStyle.Render(fmt.Sprintf(" · ✎ %d", i.files))
	}
	if sel {
		return selectLine(first, w, true) + "\n" + fit(meta, w)
	}
	return fit(first, w) + "\n" + fit(meta, w)
}

type prsLoadedMsg struct {
	gen int // load generation, to drop results of a superseded filter
	prs []domain.PullRequest
}

type prExtraMsg struct {
	number int
	extra  prExtra
	err    error
}

type prSelTick struct{ number int }

type prsScreen struct {
	c       *repoCtx
	list    list.Model
	spinner spinner.Model
	loading bool

	stateIdx int
	scope    domain.PRScope
	gen      int

	extras  map[int]prExtra
	pending map[int]bool
	lastSel int

	w, h int

	filter, scopeKey key.Binding
}

func newPRsScreen(c *repoCtx) *prsScreen {
	return &prsScreen{c: c, list: newRowList(2, 1), loading: true, scope: domain.ScopeAll,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		extras:  map[int]prExtra{}, pending: map[int]bool{},
		filter: bind("filter"), scopeKey: bind("scope")}
}

func (s *prsScreen) Title() string {
	return fmt.Sprintf("%s › Pull requests (%s)", s.c.repo.FullName(), scopeLabels[s.scope])
}
func (s *prsScreen) Header() string { return s.c.header() }
func (s *prsScreen) Tabs() ([]tab, int) {
	return s.c.tabs(), tabPRs
}
func (s *prsScreen) SwitchTab(i int) tea.Cmd { return s.c.switchTo(i) }

func (s *prsScreen) Init() tea.Cmd {
	gen, scope, state := s.gen, s.scope, prStates[s.stateIdx].state
	load := func() tea.Msg {
		prs, err := s.c.p.ListPullRequests(context.Background(), s.c.repo,
			domain.PRQuery{State: state, Scope: scope})
		if err != nil {
			return errMsg{err}
		}
		return prsLoadedMsg{gen, prs}
	}
	return tea.Batch(load, s.spinner.Tick, s.c.loadCounts())
}

// reload restarts the listing after a filter change.
func (s *prsScreen) reload() tea.Cmd {
	s.gen++
	s.loading = true
	s.list.SetItems(nil)
	return s.Init()
}

func (s *prsScreen) nextState() tea.Cmd {
	s.stateIdx = (s.stateIdx + 1) % len(prStates)
	return s.reload()
}

// nextScope cycles the involvement scope and reloads.
func (s *prsScreen) nextScope() tea.Cmd {
	for i, sc := range scopes {
		if sc == s.scope {
			s.scope = scopes[(i+1)%len(scopes)]
			break
		}
	}
	return s.reload()
}

func (s *prsScreen) selected() (domain.PullRequest, bool) {
	it, ok := s.list.SelectedItem().(prItem)
	return it.pr, ok
}

func (s *prsScreen) item(pr domain.PullRequest) prItem {
	it := prItem{pr: pr}
	if x, ok := s.extras[pr.Number]; ok {
		it.status, it.files = x.status, x.pr.ChangedFiles
	}
	return it
}

// refreshItems rebuilds the rows so cached CI statuses show, keeping the cursor.
func (s *prsScreen) refreshItems() {
	idx := s.list.Index()
	items := s.list.Items()
	out := make([]list.Item, len(items))
	for i, it := range items {
		out[i] = s.item(it.(prItem).pr)
	}
	s.list.SetItems(out)
	s.list.Select(idx)
}

func (s *prsScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if applyCounts(msg) {
		return s, nil
	}
	cmd := s.update(msg)
	return s, tea.Batch(cmd, s.watchSelection())
}

func (s *prsScreen) watchSelection() tea.Cmd {
	pr, ok := s.selected()
	if !ok || pr.Number == s.lastSel {
		return nil
	}
	s.lastSel = pr.Number
	n := pr.Number
	return tea.Tick(300*time.Millisecond, func(time.Time) tea.Msg { return prSelTick{n} })
}

func (s *prsScreen) loadExtra(pr domain.PullRequest) tea.Cmd {
	if _, ok := s.extras[pr.Number]; ok || s.pending[pr.Number] {
		return nil
	}
	s.pending[pr.Number] = true
	p, repo := s.c.p, s.c.repo
	return func() tea.Msg {
		ctx := context.Background()
		full, err := p.GetPullRequest(ctx, repo, pr.Number)
		if err != nil {
			return prExtraMsg{number: pr.Number, err: err}
		}
		x := prExtra{pr: full}
		x.files, _ = p.ListPullRequestFiles(ctx, repo, pr.Number)
		x.checks, _ = p.ListChecks(ctx, repo, full.HeadSHA)
		x.status = domain.CombineChecks(x.checks)
		return prExtraMsg{number: pr.Number, extra: x}
	}
}

func (s *prsScreen) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case prsLoadedMsg:
		if msg.gen != s.gen {
			return nil
		}
		s.loading = false
		items := make([]list.Item, len(msg.prs))
		for i, pr := range msg.prs {
			items[i] = s.item(pr)
		}
		return s.list.SetItems(items)
	case prSelTick:
		if pr, ok := s.selected(); ok && pr.Number == msg.number {
			return s.loadExtra(pr)
		}
		return nil
	case prExtraMsg:
		delete(s.pending, msg.number)
		if msg.err == nil {
			s.extras[msg.number] = msg.extra
			s.refreshItems()
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
			return s.nextState()
		case key.Matches(msg, s.scopeKey):
			return s.nextScope()
		case key.Matches(msg, keyRefresh):
			s.extras = map[int]prExtra{}
			s.c.prs, s.c.issues = -1, -1
			return tea.Batch(s.reload())
		case key.Matches(msg, keyOpenItem):
			if pr, ok := s.selected(); ok {
				return push(newDetailScreen(s.c.p, s.c.repo, prTarget(pr)))
			}
		case key.Matches(msg, keyBrowser):
			if pr, ok := s.selected(); ok {
				return openInBrowser(pr.URL)
			}
		}
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return cmd
}

func (s *prsScreen) SetSize(w, h int) {
	s.w, s.h = w, h
	lw, _ := s.widths()
	s.list.SetSize(max(lw-4, 1), max(h-toolbarHeight-2, 1))
}

func (s *prsScreen) widths() (int, int) {
	if !canSplit(s.w) {
		return s.w, 0
	}
	return splitWidths(s.w, 48)
}

func (s *prsScreen) View() string {
	bar := toolbar(s.w, s.list, "Filtrer les pull requests…",
		pill("état", prStates[s.stateIdx].label), pill("portée", scopeLabels[s.scope]))
	lw, rw := s.widths()
	ph := max(s.h-toolbarHeight, 3)

	var left string
	switch {
	case s.loading:
		left = s.spinner.View() + " chargement des pull requests…"
	case len(s.list.Items()) == 0:
		left = dimStyle.Render("Aucune pull request.")
	default:
		left = s.list.View()
	}
	if rw == 0 {
		return bar + "\n" + pane(left, lw, ph, true)
	}
	return bar + "\n" + lipgloss.JoinHorizontal(lipgloss.Top,
		pane(left, lw, ph, true), pane(s.preview(rw-4), rw, ph, false))
}

func stateBadge(st domain.State) string {
	switch st {
	case domain.StateMerged:
		return mergedBadge.Render("◉ Fusionnée")
	case domain.StateClosed:
		return closedBadge.Render("◉ Fermée")
	}
	return openBadge.Render("◉ Ouverte")
}

func (s *prsScreen) preview(w int) string {
	pr, ok := s.selected()
	if !ok {
		return ""
	}
	x, loaded := s.extras[pr.Number]
	if loaded {
		pr = x.pr
	}
	var b strings.Builder
	b.WriteString(pinkStyle.Bold(true).Render(fmt.Sprintf("#%d", pr.Number)) + " " + titleStyle.Render(pr.Title) + "\n")
	meta := stateBadge(pr.State)
	if pr.Head != "" {
		meta += dimStyle.Render("  " + pr.Head + " → " + pr.Base)
	}
	if loaded {
		meta += dimStyle.Render("  ·  ") + greenStyle.Render(fmt.Sprintf("+%d", pr.Additions)) + " " +
			redStyle.Render(fmt.Sprintf("−%d", pr.Deletions)) + dimStyle.Render(fmt.Sprintf("  ·  %d fichiers", pr.ChangedFiles))
	}
	b.WriteString(meta + "\n" + rule(w, "") + "\n")
	b.WriteString(purpleStyle.Bold(true).Render("Description") + "\n")
	if strings.TrimSpace(pr.Body) == "" {
		b.WriteString(dimStyle.Render("(aucune description)") + "\n")
	} else {
		b.WriteString(styleBody(pr.Body, w) + "\n")
	}
	if !loaded {
		b.WriteString("\n" + dimStyle.Render("chargement des checks et fichiers…"))
		return b.String()
	}
	if len(x.checks) > 0 {
		b.WriteString(rule(w, "Checks") + "\n")
		for _, c := range x.checks {
			dur := ""
			if c.Duration > 0 {
				dur = dimStyle.Render("  " + c.Duration.String())
			}
			b.WriteString(ciIcon(c.Status) + " " + c.Name + dur + "\n")
		}
	}
	if len(x.files) > 0 {
		b.WriteString(rule(w, "Fichiers modifiés") + "\n")
		for _, f := range x.files {
			letter := map[string]string{"added": greenStyle.Render("A"), "removed": redStyle.Render("D"),
				"renamed": blueStyle.Render("R")}[f.Status]
			if letter == "" {
				letter = yellowStyle.Render("M")
			}
			b.WriteString(letter + " " + fit(f.Name, max(w-14, 8)) + " " + greenStyle.Render(fmt.Sprintf("+%d", f.Additions)) +
				" " + redStyle.Render(fmt.Sprintf("−%d", f.Deletions)) + "\n")
		}
	}
	return b.String()
}

func (s *prsScreen) Capturing() bool { return s.list.FilterState() == list.Filtering }
func (s *prsScreen) Keys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("j"), key.WithHelp("j/k", "naviguer")),
		keyOpenItem, s.filter, s.scopeKey, keySearch, keySwitch, keyBrowser, keyRefresh,
	}
}

func (s *prsScreen) Restyle(dark bool) { restyleList(&s.list, dark) }
