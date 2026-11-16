// Package fake is an in-memory Provider for developing the UI offline.
package fake

import (
	"context"
	"time"

	"github.com/kevin/gitui/internal/domain"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (*Provider) Name() string { return "fake" }

func (*Provider) CurrentUser(context.Context) (domain.User, error) {
	return domain.User{Login: "octocat", Name: "The Octocat"}, nil
}

func (*Provider) ListRepos(context.Context) ([]domain.Repo, error) {
	now := time.Now()
	return []domain.Repo{
		{Owner: "octocat", Name: "hello-world", Description: "My first repo", Stars: 42, UpdatedAt: now},
		{Owner: "octocat", Name: "secret", Description: "Private stuff", Private: true, UpdatedAt: now},
	}, nil
}

func (*Provider) ListPullRequests(_ context.Context, _ domain.Repo, _ domain.PRQuery) ([]domain.PullRequest, error) {
	return []domain.PullRequest{
		{Number: 1, Title: "Add feature", Author: "octocat", State: domain.StateOpen, Head: "feat", Base: "main", UpdatedAt: time.Now()},
	}, nil
}

func (*Provider) GetPullRequestDiff(context.Context, domain.Repo, int) (string, error) {
	return "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n", nil
}

func (*Provider) ListIssues(_ context.Context, _ domain.Repo, _ domain.State) ([]domain.Issue, error) {
	return []domain.Issue{{Number: 2, Title: "Bug", Author: "octocat", State: domain.StateOpen, UpdatedAt: time.Now()}}, nil
}

func (*Provider) ListComments(context.Context, domain.Repo, int) ([]domain.Comment, error) {
	return []domain.Comment{{Author: "octocat", Body: "LGTM", CreatedAt: time.Now()}}, nil
}

func (*Provider) AddComment(context.Context, domain.Repo, int, string) error         { return nil }
func (*Provider) ApprovePullRequest(context.Context, domain.Repo, int, string) error { return nil }
func (*Provider) MergePullRequest(context.Context, domain.Repo, int) error           { return nil }
