// Package fake is an in-memory Provider for developing the UI offline.
package fake

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kevin/gitui/internal/domain"
)

type Provider struct {
	mu      sync.Mutex
	starred map[string]bool
}

func New() *Provider { return &Provider{starred: map[string]bool{"charmbracelet/bubbles": true}} }

func (*Provider) Name() string { return "fake" }

func (*Provider) CurrentUser(context.Context) (domain.User, error) {
	return domain.User{Login: "octocat", Name: "The Octocat"}, nil
}

func ago(d time.Duration) time.Time { return time.Now().Add(-d) }

const (
	hour = time.Hour
	day  = 24 * time.Hour
)

func repos() []domain.Repo {
	return []domain.Repo{
		{Owner: "octocat", Name: "hello-world", Description: "My first repo", Stars: 42, UpdatedAt: ago(2 * hour),
			Language: "Go", Forks: 3, DefaultBranch: "main", License: "MIT", Topics: []string{"demo", "go"}},
		{Owner: "octocat", Name: "secret", Description: "Private stuff", Private: true, UpdatedAt: ago(day),
			Language: "Shell", DefaultBranch: "main"},
		{Owner: "charmbracelet", Name: "bubbles", Description: "TUI components for Bubble Tea", Stars: 6100,
			UpdatedAt: ago(day), Language: "Go", Forks: 340, DefaultBranch: "main", License: "MIT"},
		{Owner: "octocat", Name: "fork-neovim", Description: "A fork", Fork: true, UpdatedAt: ago(90 * day),
			Language: "Lua", DefaultBranch: "master"},
		{Owner: "octocat", Name: "old-blog", Archived: true, Stars: 1, UpdatedAt: ago(700 * day),
			Language: "JS", DefaultBranch: "main"},
	}
}

func (p *Provider) mark(rs []domain.Repo) []domain.Repo {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range rs {
		rs[i].Starred = p.starred[rs[i].FullName()]
	}
	return rs
}

func (p *Provider) ListRepos(context.Context) ([]domain.Repo, error) {
	var out []domain.Repo
	for _, r := range repos() {
		if r.Owner == "octocat" {
			out = append(out, r)
		}
	}
	return p.mark(out), nil
}

func (p *Provider) ListStarred(context.Context) ([]domain.Repo, error) {
	var out []domain.Repo
	for _, r := range p.mark(repos()) {
		if r.Starred {
			out = append(out, r)
		}
	}
	return out, nil
}

func (p *Provider) SetStarred(_ context.Context, r domain.Repo, on bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.starred[r.FullName()] = on
	return nil
}

func (*Provider) ListOrgs(context.Context) ([]domain.Org, error) {
	return []domain.Org{{Login: "charmbracelet", Description: "Make the command line glamorous"}}, nil
}

func (p *Provider) ListOrgRepos(_ context.Context, org string) ([]domain.Repo, error) {
	var out []domain.Repo
	for _, r := range repos() {
		if r.Owner == org {
			out = append(out, r)
		}
	}
	return p.mark(out), nil
}

func (*Provider) CountOpen(context.Context, domain.Repo) (int, int, error) { return 12, 48, nil }

func (*Provider) ListPullRequests(_ context.Context, _ domain.Repo, _ domain.PRQuery) ([]domain.PullRequest, error) {
	return []domain.PullRequest{
		{Number: 1, Title: "Add feature", Body: "Adds the feature.\n\n- first\n- second\n\nCloses #2", Author: "octocat",
			State: domain.StateOpen, Head: "feat", Base: "main", UpdatedAt: ago(2 * hour)},
	}, nil
}

func (*Provider) GetPullRequest(_ context.Context, _ domain.Repo, n int) (domain.PullRequest, error) {
	return domain.PullRequest{Number: n, Title: "Add feature", Body: "Adds the feature.\n\n- first\n- second\n\nCloses #2",
		Author: "octocat", State: domain.StateOpen, Head: "feat", Base: "main", UpdatedAt: ago(2 * hour),
		HeadSHA: "a3f9c21", Additions: 84, Deletions: 31, ChangedFiles: 3, Comments: 1}, nil
}

func (*Provider) ListPullRequestFiles(context.Context, domain.Repo, int) ([]domain.PRFile, error) {
	return []domain.PRFile{
		{Name: "tea.go", Status: "modified", Additions: 31, Deletions: 12},
		{Name: "renderer.go", Status: "modified", Additions: 40, Deletions: 15},
		{Name: "resize_test.go", Status: "added", Additions: 13},
	}, nil
}

func (*Provider) ListChecks(context.Context, domain.Repo, string) ([]domain.Check, error) {
	return []domain.Check{
		{Name: "build (ubuntu)", Status: domain.CheckSuccess, Duration: 42 * time.Second},
		{Name: "build (windows)", Status: domain.CheckSuccess, Duration: 68 * time.Second},
		{Name: "lint", Status: domain.CheckSuccess, Duration: 19 * time.Second},
	}, nil
}

func (*Provider) GetPullRequestDiff(context.Context, domain.Repo, int) (string, error) {
	return "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n", nil
}

func (*Provider) ListIssues(_ context.Context, _ domain.Repo, _ domain.State) ([]domain.Issue, error) {
	return []domain.Issue{{Number: 2, Title: "Bug", Body: "It **crashes**.", Author: "octocat", State: domain.StateOpen,
		Labels: []string{"bug"}, UpdatedAt: ago(day)}}, nil
}

func (*Provider) ListComments(context.Context, domain.Repo, int) ([]domain.Comment, error) {
	return []domain.Comment{{Author: "octocat", Body: "LGTM", CreatedAt: time.Now()}}, nil
}

func (*Provider) AddComment(context.Context, domain.Repo, int, string) error         { return nil }
func (*Provider) ApprovePullRequest(context.Context, domain.Repo, int, string) error { return nil }
func (*Provider) MergePullRequest(context.Context, domain.Repo, int) error           { return nil }

func (*Provider) ListBranches(context.Context, domain.Repo) ([]string, error) {
	return []string{"main", "dev"}, nil
}

// files is the demo repository: path -> content.
var files = map[string]string{
	"README.md":                    "# hello-world\n\nA demo repository.\n",
	".gitignore":                   "gitui\n*.test\n",
	"go.mod":                       "module example.com/hello\n\ngo 1.26\n",
	"examples/list-simple/main.go": sample,
	"examples/list-simple/go.mod":  "module list-simple\n\ngo 1.26\n",
	"examples/spinner/main.go":     "package main\n\nfunc main() {}\n",
	"examples/chat/main.go":        "package main\n\nfunc main() {}\n",
	"tea/tea.go":                   "package tea\n",
	"assets/logo.png":              "\x00PNG\x00binary",
}

const sample = `package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	choices  []string
	cursor   int
	selected map[int]struct{}
}

func (m model) Init() tea.Cmd { return nil }

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
`

func (*Provider) ListTree(_ context.Context, _ domain.Repo, _, dir string) ([]domain.TreeEntry, error) {
	prefix := ""
	if dir != "" {
		prefix = strings.TrimSuffix(dir, "/") + "/"
	}
	seen := map[string]domain.TreeEntry{}
	for p, c := range files {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok {
			continue
		}
		name, _, nested := strings.Cut(rest, "/")
		if nested {
			seen[name] = domain.TreeEntry{Path: prefix + name, Name: name, Type: domain.EntryDir}
		} else {
			seen[name] = domain.TreeEntry{Path: p, Name: name, Type: domain.EntryFile, Size: len(c)}
		}
	}
	if len(seen) == 0 && dir != "" {
		return nil, fmt.Errorf("%s : dossier introuvable", dir)
	}
	out := make([]domain.TreeEntry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (*Provider) GetFile(_ context.Context, _ domain.Repo, _, p string) (domain.FileContent, error) {
	c, ok := files[path.Clean(p)]
	if !ok {
		return domain.FileContent{}, fmt.Errorf("%s : fichier introuvable", p)
	}
	f := domain.FileContent{Path: p, Size: len(c), CommitSHA: "a3f9c21", CommitTime: ago(3 * day)}
	if strings.ContainsRune(c, 0) {
		f.Binary = true
	} else {
		f.Content = c
	}
	return f, nil
}
