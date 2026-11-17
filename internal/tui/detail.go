package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider"
)

// target is what the detail screen shows: a pull request or an issue.
type target struct {
	isPR       bool
	number     int
	title      string
	body       string
	author     string
	state      domain.State
	draft      bool
	head, base string
	labels     []string
	url        string
}

func prTarget(pr domain.PullRequest) target {
	return target{isPR: true, number: pr.Number, title: pr.Title, body: pr.Body, author: pr.Author,
		state: pr.State, draft: pr.Draft, head: pr.Head, base: pr.Base, url: pr.URL}
}

func issueTarget(i domain.Issue) target {
	return target{number: i.Number, title: i.Title, body: i.Body, author: i.Author,
		state: i.State, labels: i.Labels, url: i.URL}
}

type detailMode int

const (
	modeView    detailMode = iota
	modeCompose            // writing a comment
	modeConfirm            // waiting for y/n on approve or merge
)

type action int

const (
	actApprove action = iota
	actMerge
)

type (
	commentsLoadedMsg []domain.Comment
	diffLoadedMsg     string
	// actionDoneMsg reports a finished write action.
	actionDoneMsg struct {
		text   string
		reload bool // refetch comments
		merged bool
	}
)

const requestTimeout = 30 * time.Second

type detailScreen struct {
	p    provider.Provider
	repo domain.Repo
	t    target

	vp       viewport.Model
	comments []domain.Comment
	diff     string
	showDiff bool

	loadingComments bool
	loadingDiff     bool
	busy            bool // a write action is in flight
	spinner         spinner.Model

	mode    detailMode
	pending action
	input   textarea.Model

	w, h int

	comment, approve, merge, diffKey, sendKey, cancelKey, yes, no key.Binding
}

func newDetailScreen(p provider.Provider, repo domain.Repo, t target) *detailScreen {
	ta := textarea.New()
	ta.Placeholder = "Votre commentaire…"
	ta.ShowLineNumbers = false
	ta.SetStyles(textarea.DefaultStyles(isDark))
	vp := viewport.New()
	vp.SoftWrap = true
	return &detailScreen{
		p: p, repo: repo, t: t, vp: vp, input: ta,
		loadingComments: true,
		spinner:         spinner.New(spinner.WithSpinner(spinner.Dot)),
		comment:         bind("comment"),
		approve:         bind("approve"),
		merge:           bind("merge"),
		diffKey:         bind("diff"),
		sendKey:         bind("send"),
		cancelKey:       bind("cancel"),
		yes:             bind("yes"),
		no:              bind("no"),
	}
}

func (s *detailScreen) Title() string {
	kind := "Issue"
	if s.t.isPR {
		kind = "PR"
	}
	return fmt.Sprintf("%s #%d", kind, s.t.number)
}

func (s *detailScreen) Init() tea.Cmd {
	s.render()
	return tea.Batch(s.loadComments(), s.spinner.Tick)
}

func (s *detailScreen) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}

func (s *detailScreen) loadComments() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := s.ctx()
		defer cancel()
		cs, err := s.p.ListComments(ctx, s.repo, s.t.number)
		if err != nil {
			return errMsg{err}
		}
		return commentsLoadedMsg(cs)
	}
}

func (s *detailScreen) loadDiff() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := s.ctx()
		defer cancel()
		d, err := s.p.GetPullRequestDiff(ctx, s.repo, s.t.number)
		if err != nil {
			return errMsg{err}
		}
		return diffLoadedMsg(d)
	}
}

// act runs a write action, then reports the outcome as actionDoneMsg/errMsg.
func (s *detailScreen) act(f func(ctx context.Context) error, done actionDoneMsg) tea.Cmd {
	s.busy = true
	return func() tea.Msg {
		ctx, cancel := s.ctx()
		defer cancel()
		if err := f(ctx); err != nil {
			return errMsg{err}
		}
		return done
	}
}

func (s *detailScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case commentsLoadedMsg:
		s.loadingComments = false
		s.comments = msg
		s.render()
		return s, nil
	case diffLoadedMsg:
		s.loadingDiff = false
		s.diff = string(msg)
		s.render()
		return s, nil
	case actionDoneMsg:
		s.busy = false
		if msg.merged {
			s.t.state = domain.StateMerged
		}
		s.render()
		cmds := []tea.Cmd{info(msg.text)}
		if msg.reload {
			s.loadingComments = true
			cmds = append(cmds, s.loadComments(), s.spinner.Tick)
		}
		return s, tea.Batch(cmds...)
	case errMsg:
		s.loadingComments, s.loadingDiff, s.busy = false, false, false
		s.render()
		return s, nil
	case spinner.TickMsg:
		if !s.loadingComments && !s.loadingDiff {
			return s, nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		s.render()
		return s, cmd
	case tea.KeyPressMsg:
		switch s.mode {
		case modeCompose:
			return s.updateCompose(msg)
		case modeConfirm:
			return s.updateConfirm(msg)
		}
		if cmd, handled := s.handleKey(msg); handled {
			return s, cmd
		}
	}
	// scrolling (keys in view mode, mouse wheel, ...)
	if s.mode == modeCompose {
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}
	var cmd tea.Cmd
	s.vp, cmd = s.vp.Update(msg)
	return s, cmd
}

func (s *detailScreen) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, keyBrowser):
		return openInBrowser(s.t.url), true
	case key.Matches(msg, s.comment):
		if s.busy {
			return nil, true
		}
		s.mode = modeCompose
		s.layoutParts()
		return s.input.Focus(), true
	case key.Matches(msg, s.diffKey) && s.t.isPR:
		s.showDiff = !s.showDiff
		s.vp.SoftWrap = !s.showDiff // diffs scroll horizontally instead
		s.vp.GotoTop()
		var cmd tea.Cmd
		if s.showDiff && s.diff == "" && !s.loadingDiff {
			s.loadingDiff = true
			cmd = tea.Batch(s.loadDiff(), s.spinner.Tick)
		}
		s.render()
		return cmd, true
	case key.Matches(msg, s.approve) && s.t.isPR && s.t.state == domain.StateOpen && !s.busy:
		s.mode, s.pending = modeConfirm, actApprove
		s.layoutParts()
		return nil, true
	case key.Matches(msg, s.merge) && s.t.isPR && s.t.state == domain.StateOpen && !s.busy:
		s.mode, s.pending = modeConfirm, actMerge
		s.layoutParts()
		return nil, true
	}
	return nil, false
}

func (s *detailScreen) updateCompose(msg tea.KeyPressMsg) (screen, tea.Cmd) {
	switch {
	case key.Matches(msg, s.cancelKey):
		s.closePrompt()
		return s, nil
	case key.Matches(msg, s.sendKey):
		body := strings.TrimSpace(s.input.Value())
		if body == "" {
			return s, info("commentaire vide, rien envoyé")
		}
		number := s.t.number
		s.closePrompt()
		s.input.Reset()
		return s, s.act(func(ctx context.Context) error {
			return s.p.AddComment(ctx, s.repo, number, body)
		}, actionDoneMsg{text: "commentaire ajouté", reload: true})
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

func (s *detailScreen) updateConfirm(msg tea.KeyPressMsg) (screen, tea.Cmd) {
	switch {
	case key.Matches(msg, s.no):
		s.closePrompt()
	case key.Matches(msg, s.yes):
		which, number := s.pending, s.t.number
		s.closePrompt()
		if which == actApprove {
			return s, s.act(func(ctx context.Context) error {
				return s.p.ApprovePullRequest(ctx, s.repo, number, "")
			}, actionDoneMsg{text: "pull request approuvée", reload: true})
		}
		return s, s.act(func(ctx context.Context) error {
			return s.p.MergePullRequest(ctx, s.repo, number)
		}, actionDoneMsg{text: "pull request mergée", merged: true})
	}
	return s, nil
}

func (s *detailScreen) closePrompt() {
	s.mode = modeView
	s.input.Blur()
	s.layoutParts()
}

// render rebuilds the viewport content for the current tab.
func (s *detailScreen) render() {
	if s.showDiff {
		switch {
		case s.loadingDiff:
			s.vp.SetContent(s.spinner.View() + " chargement du diff…")
		case strings.TrimSpace(s.diff) == "":
			s.vp.SetContent(dimStyle.Render("(diff vide)"))
		default:
			s.vp.SetContent(colorizeDiff(s.diff))
		}
		return
	}
	s.vp.SetContent(s.conversation())
}

func (s *detailScreen) conversation() string {
	var b strings.Builder
	b.WriteString(pinkStyle.Bold(true).Render(fmt.Sprintf("#%d", s.t.number)) + " " + titleStyle.Render(s.t.title))
	b.WriteString("\n")
	badge := openBadge.Render("◉ " + string(s.t.state))
	switch s.t.state {
	case domain.StateMerged:
		badge = mergedBadge.Render("◉ " + string(s.t.state))
	case domain.StateClosed:
		badge = closedBadge.Render("◉ " + string(s.t.state))
	}
	meta := dimStyle.Render("  ") + authorStyle.Render("@"+s.t.author)
	if s.t.draft {
		meta += dimStyle.Render(" · brouillon")
	}
	if s.t.head != "" && s.t.base != "" {
		meta += dimStyle.Render(fmt.Sprintf(" · %s → %s", s.t.head, s.t.base))
	}
	b.WriteString(badge + meta)
	if len(s.t.labels) > 0 {
		chips := make([]string, len(s.t.labels))
		for i, l := range s.t.labels {
			chips[i] = chip(l)
		}
		b.WriteString("\n" + strings.Join(chips, " "))
	}
	b.WriteString("\n\n")
	if strings.TrimSpace(s.t.body) == "" {
		b.WriteString(dimStyle.Render("(pas de description)"))
	} else {
		b.WriteString(s.t.body)
	}
	b.WriteString("\n\n")
	if s.loadingComments {
		b.WriteString(s.spinner.View() + " chargement des commentaires…")
		return b.String()
	}
	b.WriteString(rule(max(s.w, 20), fmt.Sprintf("%d commentaire(s)", len(s.comments))))
	for _, c := range s.comments {
		b.WriteString("\n\n" + authorStyle.Render(c.Author) + " " +
			dimStyle.Render(c.CreatedAt.Format("2006-01-02 15:04")) + "\n" + c.Body)
	}
	return b.String()
}

func (s *detailScreen) footer() string {
	switch {
	case s.mode == modeConfirm:
		what := "Approuver"
		if s.pending == actMerge {
			what = fmt.Sprintf("Merger dans %s", s.t.base)
		}
		return promptStyle.Render(fmt.Sprintf("%s #%d ? (y/n)", what, s.t.number))
	case s.busy:
		return dimStyle.Render("envoi en cours…")
	}
	return dimStyle.Render(fmt.Sprintf("%3.0f%%", s.vp.ScrollPercent()*100))
}

func (s *detailScreen) View() string {
	if s.mode == modeCompose {
		return lipgloss.JoinVertical(lipgloss.Left, s.vp.View(),
			promptStyle.Render("Nouveau commentaire"), s.input.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left, s.vp.View(), s.footer())
}

func (s *detailScreen) SetSize(w, h int) {
	s.w, s.h = w, h
	s.layoutParts()
}

// layoutParts sizes the viewport and the editor for the current mode.
func (s *detailScreen) layoutParts() {
	s.input.SetWidth(max(s.w, 10))
	if s.mode == modeCompose {
		taH := max(min(s.h/2, 8), 3)
		s.input.SetHeight(taH)
		s.vp.SetWidth(s.w)
		s.vp.SetHeight(max(s.h-taH-1, 1))
	} else {
		s.vp.SetWidth(s.w)
		s.vp.SetHeight(max(s.h-1, 1)) // 1 line for the footer
	}
	s.render()
}

func (s *detailScreen) Capturing() bool { return s.mode != modeView }

func (s *detailScreen) Keys() []key.Binding {
	switch s.mode {
	case modeCompose:
		return []key.Binding{s.sendKey, s.cancelKey}
	case modeConfirm:
		return []key.Binding{s.yes, s.no}
	}
	out := []key.Binding{s.comment}
	if s.t.isPR {
		out = append(out, s.diffKey)
		if s.t.state == domain.StateOpen {
			out = append(out, s.approve, s.merge)
		}
	}
	return append(out, keyBrowser)
}

func (s *detailScreen) Restyle(dark bool) {
	s.input.SetStyles(textarea.DefaultStyles(dark))
	s.render()
}
