package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider/fake"
)

type callLog struct {
	*fake.Provider
	calls []string
}

func (c *callLog) AddComment(_ context.Context, _ domain.Repo, n int, body string) error {
	c.calls = append(c.calls, "comment:"+body)
	return nil
}
func (c *callLog) ApprovePullRequest(_ context.Context, _ domain.Repo, n int, _ string) error {
	c.calls = append(c.calls, "approve")
	return nil
}
func (c *callLog) MergePullRequest(_ context.Context, _ domain.Repo, n int) error {
	c.calls = append(c.calls, "merge")
	return nil
}

var testRepo = domain.Repo{Owner: "o", Name: "r"}

func newPRDetail(t *testing.T) (*detailScreen, *callLog) {
	t.Helper()
	p := &callLog{Provider: fake.New()}
	s := newDetailScreen(p, testRepo, prTarget(domain.PullRequest{
		Number: 3, Title: "Add thing", Body: "the body", Author: "alice",
		State: domain.StateOpen, Head: "feat", Base: "main", URL: "https://x/pr/3",
	}))
	s.SetSize(80, 20)
	return s, p
}

// exec runs a command tree synchronously and feeds each message back.
func exec(t *testing.T, s screen, cmd tea.Cmd) screen {
	t.Helper()
	if cmd == nil {
		return s
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		for _, c := range b {
			s = exec(t, s, c)
		}
		return s
	}
	if msg == nil {
		return s
	}
	if _, ok := msg.(spinner.TickMsg); ok {
		return s // ticks reschedule themselves forever; skip
	}
	s, next := s.Update(msg)
	return exec(t, s, next)
}

func keyOf(code rune, text string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code, Text: text} }

func press(t *testing.T, s screen, k tea.KeyPressMsg) (screen, tea.Cmd) {
	t.Helper()
	return s.Update(k)
}

func TestDetailShowsConversation(t *testing.T) {
	s, _ := newPRDetail(t)
	s.Init()
	s.Update(commentsLoadedMsg{{Author: "bob", Body: "looks good"}})
	v := s.View()
	for _, want := range []string{"#3 Add thing", "the body", "feat → main", "bob", "looks good"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
}

func TestDetailDiffToggle(t *testing.T) {
	s, _ := newPRDetail(t)
	s.Init()
	_, cmd := press(t, s, tea.KeyPressMsg{Code: tea.KeyTab})
	if !s.showDiff || !s.loadingDiff || cmd == nil {
		t.Fatalf("diff not requested: show=%v loading=%v", s.showDiff, s.loadingDiff)
	}
	s.Update(diffLoadedMsg("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n"))
	if v := s.View(); !strings.Contains(v, "+new") || !strings.Contains(v, "-old") {
		t.Fatalf("diff not shown:\n%s", v)
	}
	press(t, s, tea.KeyPressMsg{Code: tea.KeyTab}) // back to conversation
	if s.showDiff || !strings.Contains(s.View(), "the body") {
		t.Fatal("tab should return to conversation")
	}
}

func TestColorizeDiffClassifiesLines(t *testing.T) {
	out := strings.Split(colorizeDiff("+++ b/x\n--- a/x\n+add\n-del\n@@ -1 +1 @@\n ctx"), "\n")
	want := []string{
		diffMeta.Render("+++ b/x"), diffMeta.Render("--- a/x"),
		diffAdd.Render("+add"), diffDel.Render("-del"),
		diffHunk.Render("@@ -1 +1 @@"), " ctx",
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("line %d = %q want %q", i, out[i], want[i])
		}
	}
}

func TestComposeAndSendComment(t *testing.T) {
	s, p := newPRDetail(t)
	s.Init()
	press(t, s, keyOf('c', "c"))
	if !s.Capturing() {
		t.Fatal("compose mode must capture keys")
	}
	for _, r := range "hi" {
		press(t, s, keyOf(r, string(r)))
	}
	_, cmd := press(t, s, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if s.Capturing() {
		t.Fatal("prompt should close after send")
	}
	exec(t, s, cmd)
	if len(p.calls) != 1 || p.calls[0] != "comment:hi" {
		t.Fatalf("calls = %v", p.calls)
	}
}

func TestEmptyCommentIsNotSent(t *testing.T) {
	s, p := newPRDetail(t)
	press(t, s, keyOf('c', "c"))
	_, cmd := press(t, s, tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	exec(t, s, cmd)
	if len(p.calls) != 0 {
		t.Fatalf("empty comment sent: %v", p.calls)
	}
}

func TestEscCancelsCompose(t *testing.T) {
	s, p := newPRDetail(t)
	press(t, s, keyOf('c', "c"))
	press(t, s, tea.KeyPressMsg{Code: tea.KeyEscape})
	if s.Capturing() || len(p.calls) != 0 {
		t.Fatal("esc should cancel without sending")
	}
}

func TestApproveAndMergeNeedConfirmation(t *testing.T) {
	s, p := newPRDetail(t)
	press(t, s, keyOf('a', "a"))
	if !s.Capturing() || !strings.Contains(s.View(), "Approuver #3 ? (y/n)") {
		t.Fatalf("no confirmation prompt:\n%s", s.View())
	}
	press(t, s, keyOf('n', "n"))
	if len(p.calls) != 0 || s.Capturing() {
		t.Fatalf("n must cancel: %v", p.calls)
	}

	press(t, s, keyOf('a', "a"))
	_, cmd := press(t, s, keyOf('y', "y"))
	exec(t, s, cmd)
	press(t, s, keyOf('m', "m"))
	if !strings.Contains(s.View(), "Merger dans main") {
		t.Fatalf("merge prompt missing:\n%s", s.View())
	}
	_, cmd = press(t, s, keyOf('y', "y"))
	exec(t, s, cmd)
	if strings.Join(p.calls, ",") != "approve,merge" {
		t.Fatalf("calls = %v", p.calls)
	}
	if s.t.state != domain.StateMerged {
		t.Fatalf("state = %v", s.t.state)
	}
	// merged PR: no more approve/merge
	press(t, s, keyOf('m', "m"))
	if s.Capturing() {
		t.Fatal("merge offered on a merged PR")
	}
}

func TestIssueHasNoPRActions(t *testing.T) {
	p := &callLog{Provider: fake.New()}
	s := newDetailScreen(p, testRepo, issueTarget(domain.Issue{Number: 9, Title: "Bug", State: domain.StateOpen, Labels: []string{"bug"}}))
	s.SetSize(80, 20)
	for _, k := range []rune{'a', 'm'} {
		press(t, s, keyOf(k, string(k)))
		if s.Capturing() {
			t.Fatalf("%q must not act on an issue", k)
		}
	}
	press(t, s, tea.KeyPressMsg{Code: tea.KeyTab})
	if s.showDiff {
		t.Fatal("issues have no diff")
	}
	s.Update(commentsLoadedMsg(nil))
	if !strings.Contains(s.View(), "bug") {
		t.Fatalf("labels missing:\n%s", s.View())
	}
}

func TestActionErrorResetsBusy(t *testing.T) {
	s, _ := newPRDetail(t)
	s.busy = true
	s.Update(errMsg{errors.New("boom")})
	if s.busy {
		t.Fatal("busy must clear on error so the user can retry")
	}
}

func TestOpenInBrowser(t *testing.T) {
	var opened string
	old := openURL
	openURL = func(u string) error { opened = u; return nil }
	t.Cleanup(func() { openURL = old })

	s, _ := newPRDetail(t)
	_, cmd := press(t, s, keyOf('o', "o"))
	if m, ok := cmd().(infoMsg); !ok || opened != "https://x/pr/3" {
		t.Fatalf("opened=%q msg=%#v", opened, m)
	}

	openURL = func(string) error { return errors.New("no browser") }
	_, cmd = press(t, s, keyOf('o', "o"))
	if _, ok := cmd().(errMsg); !ok {
		t.Fatal("failure must surface as errMsg")
	}
}

func TestNavigationReposToDetailAndSiblings(t *testing.T) {
	m := New(Options{Provider: fake.New()})
	m = run(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = run(t, m, reposLoadedMsg(mustRepos(t)))
	m = run(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}) // -> PRs
	// the PRs are loaded by Init's cmd; deliver them explicitly
	prs, _ := fake.New().ListPullRequests(context.Background(), testRepo, domain.PRQuery{})
	m = run(t, m, prsLoadedMsg{gen: 0, prs: prs})
	if !strings.Contains(view(m), "Add feature") {
		t.Fatalf("PR list missing:\n%s", view(m))
	}

	m = run(t, m, tea.KeyPressMsg{Code: tea.KeyTab}) // sibling: issues, stack unchanged
	if len(m.stack) != 2 || !strings.Contains(view(m), "Issues") {
		t.Fatalf("tab should swap to issues in place (stack=%d):\n%s", len(m.stack), view(m))
	}
	m = run(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if _, ok := m.top().(*prsScreen); !ok {
		t.Fatalf("tab again should return to PRs, got %T", m.top())
	}
	m = run(t, m, prsLoadedMsg{gen: 0, prs: prs})
	m = run(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if _, ok := m.top().(*detailScreen); !ok || len(m.stack) != 3 {
		t.Fatalf("enter should open detail, got %T stack=%d", m.top(), len(m.stack))
	}
	m = run(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(m.stack) != 2 {
		t.Fatalf("esc should go back, stack=%d", len(m.stack))
	}
}

func TestInfoMessageShownThenCleared(t *testing.T) {
	m := New(Options{Provider: fake.New()})
	m = run(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = run(t, m, infoMsg{"fait"})
	if !strings.Contains(view(m), "fait") {
		t.Fatal("info not shown")
	}
	m = run(t, m, tea.KeyPressMsg{Code: 'x', Text: "x"})
	if strings.Contains(view(m), "fait") {
		t.Fatal("info should clear on keypress")
	}
}
