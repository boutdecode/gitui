// Package provider defines the contract every git hosting backend implements.
package provider

import (
	"context"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/i18n"
)

type Provider interface {
	Name() string
	CurrentUser(ctx context.Context) (domain.User, error)
	ListRepos(ctx context.Context) ([]domain.Repo, error)
	ListPullRequests(ctx context.Context, repo domain.Repo, q domain.PRQuery) ([]domain.PullRequest, error)
	GetPullRequestDiff(ctx context.Context, repo domain.Repo, number int) (string, error)
	ListIssues(ctx context.Context, repo domain.Repo, state domain.State) ([]domain.Issue, error)
	ListComments(ctx context.Context, repo domain.Repo, number int) ([]domain.Comment, error)
	AddComment(ctx context.Context, repo domain.Repo, number int, body string) error
	ApprovePullRequest(ctx context.Context, repo domain.Repo, number int, body string) error
	MergePullRequest(ctx context.Context, repo domain.Repo, number int) error

	// Browsing code.
	ListBranches(ctx context.Context, repo domain.Repo) ([]string, error)
	// ListTree lists one directory; path "" is the root and ref "" the default branch.
	ListTree(ctx context.Context, repo domain.Repo, ref, path string) ([]domain.TreeEntry, error)
	GetFile(ctx context.Context, repo domain.Repo, ref, path string) (domain.FileContent, error)

	// Repository collections.
	ListStarred(ctx context.Context) ([]domain.Repo, error)
	SetStarred(ctx context.Context, repo domain.Repo, starred bool) error
	ListOrgs(ctx context.Context) ([]domain.Org, error)
	ListOrgRepos(ctx context.Context, org string) ([]domain.Repo, error)

	// Details loaded on demand for the current selection (each costs API calls).
	CountOpen(ctx context.Context, repo domain.Repo) (prs, issues int, err error)
	GetPullRequest(ctx context.Context, repo domain.Repo, number int) (domain.PullRequest, error)
	ListPullRequestFiles(ctx context.Context, repo domain.Repo, number int) ([]domain.PRFile, error)
	ListChecks(ctx context.Context, repo domain.Repo, ref string) ([]domain.Check, error)
}

// ErrUnauthorized means the token is missing, expired or revoked.
var ErrUnauthorized = i18n.Err("session expired or token revoked")

// RateLimiter is implemented by providers that can report their API quota.
type RateLimiter interface {
	// RateLimit returns the quota from the most recent response; ok is false
	// until a request has completed.
	RateLimit() (rl domain.RateLimit, ok bool)
}
