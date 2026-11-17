package github

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/kevin/gitui/internal/domain"
)

func TestListTreeAndGetFile(t *testing.T) {
	p, _ := newStub(t, func(_ *stub, m *http.ServeMux) {
		m.HandleFunc("/repos/o/r/contents/", func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/o/r/contents/":
				writeJSON(w, []map[string]any{
					{"type": "dir", "name": "cmd", "path": "cmd"},
					{"type": "file", "name": "go.mod", "path": "go.mod", "size": 12},
				})
			case "/repos/o/r/contents/main.go":
				writeJSON(w, map[string]any{"type": "file", "name": "main.go", "path": "main.go", "size": 5,
					"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("hello"))})
			case "/repos/o/r/contents/img.bin":
				writeJSON(w, map[string]any{"type": "file", "name": "img.bin", "path": "img.bin", "size": 3,
					"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte{0, 1, 2})})
			case "/repos/o/r/contents/big.txt":
				writeJSON(w, map[string]any{"type": "file", "name": "big.txt", "path": "big.txt", "size": maxFileSize + 1})
			}
		})
		m.HandleFunc("/repos/o/r/commits", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{{"sha": "abc1234", "commit": map[string]any{
				"committer": map[string]any{"date": "2026-01-02T03:04:05Z"}}}})
		})
	})
	ctx := context.Background()

	tree, err := p.ListTree(ctx, repo, "", "")
	if err != nil || len(tree) != 2 || tree[0].Type != domain.EntryDir || tree[1].Size != 12 {
		t.Fatalf("tree = %+v, %v", tree, err)
	}
	f, err := p.GetFile(ctx, repo, "", "main.go")
	if err != nil || f.Content != "hello" || f.CommitSHA != "abc1234" || f.CommitTime.IsZero() {
		t.Fatalf("file = %+v, %v", f, err)
	}
	if f, err = p.GetFile(ctx, repo, "", "img.bin"); err != nil || !f.Binary || f.Content != "" {
		t.Fatalf("binary file = %+v, %v", f, err)
	}
	if f, err = p.GetFile(ctx, repo, "", "big.txt"); err != nil || !f.TooLarge || f.Content != "" {
		t.Fatalf("large file = %+v, %v", f, err)
	}
}

func TestStarredAndOrgs(t *testing.T) {
	var starred, unstarred bool
	p, _ := newStub(t, func(_ *stub, m *http.ServeMux) {
		m.HandleFunc("/user/starred", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{{"starred_at": "2026-01-01T00:00:00Z",
				"repo": map[string]any{"name": "a", "owner": map[string]any{"login": "o"}, "language": "Go"}}})
		})
		m.HandleFunc("/user/starred/o/r", func(w http.ResponseWriter, r *http.Request) {
			starred = starred || r.Method == http.MethodPut
			unstarred = unstarred || r.Method == http.MethodDelete
			w.WriteHeader(http.StatusNoContent)
		})
		m.HandleFunc("/user/orgs", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{{"login": "acme"}})
		})
		m.HandleFunc("/orgs/acme/repos", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{{"name": "x", "owner": map[string]any{"login": "acme"}}})
		})
	})
	ctx := context.Background()
	rs, err := p.ListStarred(ctx)
	if err != nil || len(rs) != 1 || !rs[0].Starred || rs[0].Language != "Go" {
		t.Fatalf("starred = %+v, %v", rs, err)
	}
	if err := p.SetStarred(ctx, repo, true); err != nil || !starred {
		t.Fatalf("star: %v %v", err, starred)
	}
	if err := p.SetStarred(ctx, repo, false); err != nil || !unstarred {
		t.Fatalf("unstar: %v %v", err, unstarred)
	}
	orgs, err := p.ListOrgs(ctx)
	if err != nil || len(orgs) != 1 || orgs[0].Login != "acme" {
		t.Fatalf("orgs = %+v, %v", orgs, err)
	}
	or, err := p.ListOrgRepos(ctx, "acme")
	if err != nil || len(or) != 1 || or[0].FullName() != "acme/x" {
		t.Fatalf("org repos = %+v, %v", or, err)
	}
}

func TestPullRequestDetails(t *testing.T) {
	p, _ := newStub(t, func(_ *stub, m *http.ServeMux) {
		m.HandleFunc("/repos/o/r/pulls/7", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{"number": 7, "title": "T", "additions": 84, "deletions": 31,
				"changed_files": 6, "head": map[string]any{"ref": "f", "sha": "deadbeef"}, "base": map[string]any{"ref": "main"}})
		})
		m.HandleFunc("/repos/o/r/pulls/7/files", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{{"filename": "a.go", "status": "added", "additions": 3, "deletions": 0}})
		})
		m.HandleFunc("/repos/o/r/commits/deadbeef/check-runs", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{"total_count": 2, "check_runs": []map[string]any{
				{"name": "build", "status": "completed", "conclusion": "success",
					"started_at": "2026-01-01T00:00:00Z", "completed_at": "2026-01-01T00:00:42Z"},
				{"name": "lint", "status": "in_progress"},
			}})
		})
		m.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
			n := 48
			if q := r.URL.Query().Get("q"); q == "repo:o/r is:pr is:open" {
				n = 12
			}
			writeJSON(w, map[string]any{"total_count": n, "items": []any{}})
		})
	})
	ctx := context.Background()
	pr, err := p.GetPullRequest(ctx, repo, 7)
	if err != nil || pr.HeadSHA != "deadbeef" || pr.Additions != 84 || pr.ChangedFiles != 6 {
		t.Fatalf("pr = %+v, %v", pr, err)
	}
	fs, err := p.ListPullRequestFiles(ctx, repo, 7)
	if err != nil || len(fs) != 1 || fs[0].Name != "a.go" {
		t.Fatalf("files = %+v, %v", fs, err)
	}
	cs, err := p.ListChecks(ctx, repo, pr.HeadSHA)
	if err != nil || len(cs) != 2 || cs[0].Status != domain.CheckSuccess || cs[0].Duration.Seconds() != 42 ||
		cs[1].Status != domain.CheckPending || domain.CombineChecks(cs) != domain.CheckPending {
		t.Fatalf("checks = %+v, %v", cs, err)
	}
	prs, issues, err := p.CountOpen(ctx, repo)
	if err != nil || prs != 12 || issues != 48 {
		t.Fatalf("counts = %d %d %v", prs, issues, err)
	}
}
