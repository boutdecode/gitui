package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider/fake"
	"github.com/kevin/gitui/internal/recents"
)

func loadedRepos(t *testing.T, rec *recents.Store) *reposScreen {
	t.Helper()
	s := newReposScreen(fake.New(), rec)
	s.SetSize(120, 24)
	s.Update(reposLoadedMsg{repoTabMine, mustRepos(t)})
	return s
}

func names(s *reposScreen) []string {
	var out []string
	for _, it := range s.list.Items() {
		if r, ok := it.(repoItem); ok {
			out = append(out, r.r.Name)
		}
	}
	return out
}

func TestReposVisibilityFilterAndSort(t *testing.T) {
	s := loadedRepos(t, nil)
	if got := names(s); len(got) != 4 || got[0] != "hello-world" {
		t.Fatalf("default = %v (most recently pushed first)", got)
	}
	tap(s, "f") // publics
	for _, n := range names(s) {
		if n == "secret" {
			t.Fatal("private repo shown in the public filter")
		}
	}
	tap(s, "f") // privés
	if got := names(s); len(got) != 1 || got[0] != "secret" {
		t.Fatalf("private filter = %v", got)
	}
	tap(s, "f")
	if got := names(s); len(got) != 1 || got[0] != "fork-neovim" {
		t.Fatalf("fork filter = %v", got)
	}
	tap(s, "f")
	if got := names(s); len(got) != 1 || got[0] != "old-blog" {
		t.Fatalf("archived filter = %v", got)
	}
	tap(s, "f") // back to all
	tap(s, "t") // sort by name
	if got := names(s); got[0] != "fork-neovim" {
		t.Fatalf("name sort = %v", got)
	}
	tap(s, "t") // by stars
	if got := names(s); got[0] != "hello-world" || got[len(got)-1] != "secret" && got[len(got)-1] != "fork-neovim" {
		t.Fatalf("stars sort = %v", got)
	}
}

func TestReposTabsLoadLazily(t *testing.T) {
	s := loadedRepos(t, nil)
	tabs, active := s.Tabs()
	if len(tabs) != 4 || active != repoTabMine || tabs[repoTabMine].Count != 4 || tabs[repoTabStarred].Count != -1 {
		t.Fatalf("tabs = %+v active=%d", tabs, active)
	}
	cmd := s.SwitchTab(repoTabStarred)
	if cmd == nil || !s.loading[repoTabStarred] {
		t.Fatal("first visit of Favoris must load it")
	}
	starred, _ := fake.New().ListStarred(context.Background())
	s.Update(reposLoadedMsg{repoTabStarred, starred})
	if got := names(s); len(got) != 1 || got[0] != "bubbles" {
		t.Fatalf("favoris = %v", got)
	}
	if cmd := s.SwitchTab(repoTabStarred); cmd != nil {
		t.Fatal("a loaded tab must not reload")
	}
}

func TestReposStarTogglesAndSyncsFavorites(t *testing.T) {
	s := loadedRepos(t, nil)
	s.loaded[repoTabStarred] = true
	cmd := tap(s, "s")
	msg := cmd()
	done, ok := msg.(starDoneMsg)
	if !ok || !done.on || done.repo.Name != "hello-world" {
		t.Fatalf("msg = %#v", msg)
	}
	s.Update(done)
	if !s.data[repoTabMine][0].Starred || len(s.data[repoTabStarred]) != 1 {
		t.Fatalf("star not applied: mine=%+v fav=%+v", s.data[repoTabMine][0], s.data[repoTabStarred])
	}
	if v := plain(s.View()); !strings.Contains(v, "♥") {
		t.Fatalf("starred repos should be marked:\n%s", v)
	}
	// un-star
	cmd = tap(s, "s")
	s.Update(cmd())
	if len(s.data[repoTabStarred]) != 0 {
		t.Fatal("un-starring must remove it from Favoris")
	}
}

func TestReposOpenRemembersAndOpensCode(t *testing.T) {
	rec := recents.New(filepath.Join(t.TempDir(), "recent.json"))
	s := loadedRepos(t, rec)
	_, cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	b, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("expected remember + push")
	}
	var pushed screen
	for _, c := range b {
		if c == nil {
			continue
		}
		if pm, ok := c().(pushMsg); ok {
			pushed = pm.s
		}
	}
	if _, ok := pushed.(*codeScreen); !ok {
		t.Fatalf("enter should push the code screen, got %T", pushed)
	}
	got := rec.List()
	if len(got) != 1 || got[0].FullName() != "octocat/hello-world" {
		t.Fatalf("recents = %+v", got)
	}
	// Récents tab lists what was opened
	s.SwitchTab(repoTabRecent)
	if n := names(s); len(n) != 1 || n[0] != "hello-world" {
		t.Fatalf("recent tab = %v", n)
	}
}

func TestReposNarrowTerminalHidesDetail(t *testing.T) {
	s := loadedRepos(t, nil)
	s.SetSize(70, 20)
	v := plain(s.View())
	if strings.Contains(v, "Visibilité") {
		t.Fatalf("detail pane should be hidden below %d columns:\n%s", minSplitWidth, v)
	}
	if !strings.Contains(v, "octocat/hello-world") {
		t.Fatalf("list missing:\n%s", v)
	}
}

func TestReposCountsDebouncedPerSelection(t *testing.T) {
	s := loadedRepos(t, nil)
	_, cmd := s.Update(selTickMsg{"octocat/hello-world"})
	if cmd == nil {
		t.Fatal("selected repo should fetch counts")
	}
	_, _ = s.Update(cmd())
	if v := plain(s.View()); !strings.Contains(v, "12 pull requests") || !strings.Contains(v, "48 issues") {
		t.Fatalf("counts missing:\n%s", v)
	}
	// a tick for a repo that is no longer selected does nothing
	if _, cmd := s.Update(selTickMsg{"octocat/secret"}); cmd != nil {
		t.Fatal("stale tick must be ignored")
	}
	_ = domain.Repo{}
}

func TestOrganizationsTabListsOrgsThenTheirRepos(t *testing.T) {
	s := loadedRepos(t, nil)
	cmd := s.SwitchTab(repoTabOrgs)
	if cmd == nil || !s.loading[repoTabOrgs] {
		t.Fatal("first visit of Organisations must load it")
	}
	orgs, _ := fake.New().ListOrgs(context.Background())
	s.Update(orgsLoadedMsg(orgs))

	// only organizations, no repositories
	if got := names(s); len(got) != 0 {
		t.Fatalf("org list must not contain repos: %v", got)
	}
	v := plain(s.View())
	if !strings.Contains(v, "charmbracelet") || strings.Contains(v, "bubbles") {
		t.Fatalf("expected only the organization:\n%s", v)
	}
	if tabs, _ := s.Tabs(); tabs[repoTabOrgs].Count != 1 {
		t.Fatalf("tab count = %d, want the number of orgs", tabs[repoTabOrgs].Count)
	}

	// enter drills into the organization
	_, cmd = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if s.org != "charmbracelet" || cmd == nil {
		t.Fatalf("org = %q cmd=%v", s.org, cmd)
	}
	repos, _ := fake.New().ListOrgRepos(context.Background(), "charmbracelet")
	s.Update(orgReposMsg{"charmbracelet", repos})
	if got := names(s); len(got) != 1 || got[0] != "bubbles" {
		t.Fatalf("org repos = %v", got)
	}
	if h := plain(s.Header()); !strings.Contains(h, "charmbracelet") {
		t.Fatalf("header = %q", h)
	}

	// esc goes back to the list of organizations
	s.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if s.org != "" || len(names(s)) != 0 || len(s.list.Items()) != 1 {
		t.Fatalf("esc should return to the org list (org=%q)", s.org)
	}
	// re-entering uses the cache: no new request
	if _, cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatal("cached organization must not reload")
	}
	if got := names(s); len(got) != 1 {
		t.Fatalf("cached repos not shown: %v", got)
	}
}

func TestOrgRepoOpensCodeAndLeavingTabResets(t *testing.T) {
	s := loadedRepos(t, nil)
	s.SwitchTab(repoTabOrgs)
	orgs, _ := fake.New().ListOrgs(context.Background())
	s.Update(orgsLoadedMsg(orgs))
	s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	repos, _ := fake.New().ListOrgRepos(context.Background(), "charmbracelet")
	s.Update(orgReposMsg{"charmbracelet", repos})
	_, cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	var pushed screen
	switch m := cmd().(type) {
	case pushMsg: // no recents store: tea.Batch collapses to the single command
		pushed = m.s
	case tea.BatchMsg:
		for _, c := range m {
			if pm, ok := c().(pushMsg); ok {
				pushed = pm.s
			}
		}
	}
	if _, ok := pushed.(*codeScreen); !ok {
		t.Fatalf("enter on an org repo should open Code, got %T", pushed)
	}
	s.SwitchTab(repoTabMine)
	s.SwitchTab(repoTabOrgs)
	if s.org != "" {
		t.Fatal("returning to the tab must show the organizations again")
	}
}
