// Package provider defines the contract every git hosting backend implements.
package provider

import (
	"context"
	"errors"

	"github.com/kevin/gitui/internal/domain"
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
}

// ErrUnauthorized means the token is missing, expired or revoked.
var ErrUnauthorized = errors.New("session expirée ou token révoqué")

// RateLimiter is implemented by providers that can report their API quota.
type RateLimiter interface {
	// RateLimit returns the quota from the most recent response; ok is false
	// until a request has completed.
	RateLimit() (rl domain.RateLimit, ok bool)
}
