package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh "github.com/google/go-github/v90/github"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider"
)

type stub struct {
	srv      *httptest.Server
	gotAuth  string
	comments []string
}

func newStub(t *testing.T, mux func(s *stub, m *http.ServeMux)) (*Provider, *stub) {
	t.Helper()
	s := &stub{}
	m := http.NewServeMux()
	mux(s, m)
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.gotAuth = r.Header.Get("Authorization")
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", "4321")
		w.Header().Set("X-RateLimit-Reset", "1900000000")
		m.ServeHTTP(w, r)
	}))
	t.Cleanup(s.srv.Close)
	base := s.srv.URL + "/"
	p, err := New("tok123", gh.WithURLs(&base, &base))
	if err != nil {
		t.Fatal(err)
	}
	return p, s
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

var repo = domain.Repo{Owner: "o", Name: "r"}

func TestListReposPaginatesAndAuthenticates(t *testing.T) {
	p, s := newStub(t, func(s *stub, m *http.ServeMux) {
		m.HandleFunc("/user/repos", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "2" {
				writeJSON(w, []map[string]any{{"name": "b", "owner": map[string]any{"login": "o"}}})
				return
			}
			w.Header().Set("Link", fmt.Sprintf(`<%s/user/repos?page=2>; rel="next"`, s.srv.URL))
			writeJSON(w, []map[string]any{{
				"name": "a", "description": "d", "private": true, "stargazers_count": 7,
				"owner": map[string]any{"login": "o"}, "html_url": "https://github.com/o/a",
			}})
		})
	})
	repos, err := p.ListRepos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 || repos[0].URL != "https://github.com/o/a" || repos[0].FullName() != "o/a" || !repos[0].Private || repos[0].Stars != 7 || repos[1].Name != "b" {
		t.Fatalf("unexpected repos: %+v", repos)
	}
	if s.gotAuth != "Bearer tok123" {
		t.Fatalf("auth header = %q", s.gotAuth)
	}
	rl, ok := p.RateLimit()
	if !ok || rl.Limit != 5000 || rl.Remaining != 4321 {
		t.Fatalf("rate limit = %+v ok=%v", rl, ok)
	}
}

func prFixture() []map[string]any {
	u := func(l string) map[string]any { return map[string]any{"login": l} }
	return []map[string]any{
		{"number": 1, "title": "mine", "state": "open", "user": u("me"),
			"head": map[string]any{"ref": "f"}, "base": map[string]any{"ref": "main"}},
		{"number": 2, "title": "assigned", "state": "open", "user": u("bob"),
			"assignees": []any{u("me")}},
		{"number": 3, "title": "review", "state": "open", "user": u("bob"), "draft": true,
			"requested_reviewers": []any{u("me")}},
		{"number": 4, "title": "merged", "state": "closed", "user": u("bob"), "merged_at": "2026-01-01T00:00:00Z"},
		{"number": 5, "title": "closed", "state": "closed", "user": u("bob")},
	}
}

func TestListPullRequestsFilters(t *testing.T) {
	var gotState string
	p, _ := newStub(t, func(s *stub, m *http.ServeMux) {
		m.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]any{"login": "me"}) })
		m.HandleFunc("/repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
			gotState = r.URL.Query().Get("state")
			writeJSON(w, prFixture())
		})
	})
	nums := func(q domain.PRQuery) string {
		prs, err := p.ListPullRequests(context.Background(), repo, q)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, pr := range prs {
			ids = append(ids, fmt.Sprint(pr.Number))
		}
		return strings.Join(ids, ",")
	}
	cases := []struct {
		name  string
		q     domain.PRQuery
		want  string
		state string
	}{
		{"all", domain.PRQuery{}, "1,2,3,4,5", "all"},
		{"open", domain.PRQuery{State: domain.StateOpen}, "1,2,3", "open"},
		{"merged", domain.PRQuery{State: domain.StateMerged}, "4", "closed"},
		{"closed-not-merged", domain.PRQuery{State: domain.StateClosed}, "5", "closed"},
		{"authored", domain.PRQuery{Scope: domain.ScopeAuthored}, "1", "all"},
		{"assigned", domain.PRQuery{Scope: domain.ScopeAssigned}, "2", "all"},
		{"review", domain.PRQuery{Scope: domain.ScopeReviewRequested}, "3", "all"},
	}
	for _, c := range cases {
		if got := nums(c.q); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
		if gotState != c.state {
			t.Errorf("%s: API state = %q want %q", c.name, gotState, c.state)
		}
	}
	prs, _ := p.ListPullRequests(context.Background(), repo, domain.PRQuery{Scope: domain.ScopeReviewRequested})
	if !prs[0].Draft {
		t.Error("draft flag lost")
	}
	prs, _ = p.ListPullRequests(context.Background(), repo, domain.PRQuery{Scope: domain.ScopeAuthored})
	if prs[0].Head != "f" || prs[0].Base != "main" {
		t.Errorf("head/base lost: %+v", prs[0])
	}
}

func TestListIssuesSkipsPullRequests(t *testing.T) {
	p, _ := newStub(t, func(s *stub, m *http.ServeMux) {
		m.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"number": 1, "title": "bug", "state": "open", "user": map[string]any{"login": "a"},
					"labels": []any{map[string]any{"name": "bug"}}},
				{"number": 2, "title": "a PR", "state": "open", "pull_request": map[string]any{"url": "x"}},
				{"number": 3, "title": "done", "state": "closed"},
			})
		})
	})
	issues, err := p.ListIssues(context.Background(), repo, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 || issues[0].Number != 1 || issues[0].Labels[0] != "bug" || issues[1].State != domain.StateClosed {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestDiffCommentAndMerge(t *testing.T) {
	var posted, merged, approved bool
	p, _ := newStub(t, func(s *stub, m *http.ServeMux) {
		m.HandleFunc("/repos/o/r/pulls/7", func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept"), "diff") {
				http.Error(w, "want diff", 406)
				return
			}
			_, _ = w.Write([]byte("diff --git a/x b/x\n"))
		})
		m.HandleFunc("/repos/o/r/issues/7/comments", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				var b map[string]string
				_ = json.NewDecoder(r.Body).Decode(&b)
				posted = b["body"] == "hello"
				w.WriteHeader(201)
				writeJSON(w, map[string]any{"id": 1})
				return
			}
			writeJSON(w, []map[string]any{{"body": "LGTM", "user": map[string]any{"login": "bob"}}})
		})
		m.HandleFunc("/repos/o/r/pulls/7/reviews", func(w http.ResponseWriter, r *http.Request) {
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			approved = r.Method == http.MethodPost && b["event"] == "APPROVE" && b["body"] == "ok"
			writeJSON(w, map[string]any{"id": 1})
		})
		m.HandleFunc("/repos/o/r/pulls/7/merge", func(w http.ResponseWriter, _ *http.Request) {
			merged = true
			writeJSON(w, map[string]any{"merged": true})
		})
		m.HandleFunc("/repos/o/r/pulls/8/merge", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{"merged": false, "message": "not mergeable"})
		})
	})
	ctx := context.Background()
	if d, err := p.GetPullRequestDiff(ctx, repo, 7); err != nil || !strings.HasPrefix(d, "diff --git") {
		t.Fatalf("diff: %q %v", d, err)
	}
	if cs, err := p.ListComments(ctx, repo, 7); err != nil || len(cs) != 1 || cs[0].Author != "bob" {
		t.Fatalf("comments: %+v %v", cs, err)
	}
	if err := p.AddComment(ctx, repo, 7, "hello"); err != nil || !posted {
		t.Fatalf("add comment: %v posted=%v", err, posted)
	}
	if err := p.ApprovePullRequest(ctx, repo, 7, "ok"); err != nil || !approved {
		t.Fatalf("approve: %v approved=%v", err, approved)
	}
	if err := p.MergePullRequest(ctx, repo, 7); err != nil || !merged {
		t.Fatalf("merge: %v", err)
	}
	if err := p.MergePullRequest(ctx, repo, 8); err == nil || !strings.Contains(err.Error(), "not mergeable") {
		t.Fatalf("expected refusal, got %v", err)
	}
}

func TestErrorMapping(t *testing.T) {
	p, _ := newStub(t, func(s *stub, m *http.ServeMux) {
		m.HandleFunc("/user/repos", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			writeJSON(w, map[string]any{"message": "Bad credentials"})
		})
		m.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "1900000000")
			w.WriteHeader(http.StatusForbidden)
			writeJSON(w, map[string]any{"message": "API rate limit exceeded"})
		})
	})
	if _, err := p.ListRepos(context.Background()); !errors.Is(err, provider.ErrUnauthorized) {
		t.Fatalf("401 should map to ErrUnauthorized, got %v", err)
	}
	if _, err := p.ListIssues(context.Background(), repo, ""); err == nil || !strings.Contains(err.Error(), "limite") {
		t.Fatalf("rate limit error not translated: %v", err)
	}
}
