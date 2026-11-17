// Package github implements provider.Provider on top of the GitHub REST API.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	gh "github.com/google/go-github/v90/github"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider"
)

const (
	perPage = 100
	// maxPages bounds auto-pagination (maxPages*perPage items per listing) so
	// a huge account can't trigger hundreds of requests from one keypress.
	maxPages = 10
)

type Provider struct {
	client *gh.Client

	mu   sync.Mutex
	me   string // cached login of the authenticated user
	rate *domain.RateLimit
}

var (
	_ provider.Provider    = (*Provider)(nil)
	_ provider.RateLimiter = (*Provider)(nil)
)

// New builds a provider authenticated with token. Extra options let callers
// (and tests) point the client at another base URL.
func New(token string, opts ...gh.ClientOptionsFunc) (*Provider, error) {
	opts = append([]gh.ClientOptionsFunc{
		gh.WithAuthToken(token),
		gh.WithUserAgent("gitui"),
	}, opts...)
	c, err := gh.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &Provider{client: c}, nil
}

func (*Provider) Name() string { return "github" }

// RateLimit implements provider.RateLimiter.
func (p *Provider) RateLimit() (domain.RateLimit, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rate == nil {
		return domain.RateLimit{}, false
	}
	return *p.rate, true
}

// track records the quota from a response and translates API errors.
func (p *Provider) track(resp *gh.Response, err error) error {
	if resp != nil && resp.Rate.Limit > 0 {
		p.mu.Lock()
		p.rate = &domain.RateLimit{
			Limit:     resp.Rate.Limit,
			Remaining: resp.Rate.Remaining,
			Reset:     resp.Rate.Reset.Time,
		}
		p.mu.Unlock()
	}
	return mapErr(resp, err)
}

func mapErr(resp *gh.Response, err error) error {
	if err == nil {
		return nil
	}
	var rl *gh.RateLimitError
	var abuse *gh.AbuseRateLimitError
	switch {
	case errors.As(err, &rl):
		return fmt.Errorf("limite d'API atteinte, réessayez après %s", rl.Rate.Reset.Format("15:04"))
	case errors.As(err, &abuse):
		return errors.New("GitHub demande de ralentir (limite secondaire), réessayez dans un instant")
	case resp != nil && resp.StatusCode == http.StatusUnauthorized:
		return provider.ErrUnauthorized
	}
	return err
}

func (p *Provider) CurrentUser(ctx context.Context) (domain.User, error) {
	u, resp, err := p.client.Users.Get(ctx, "") // "" = authenticated user
	if err := p.track(resp, err); err != nil {
		return domain.User{}, err
	}
	p.mu.Lock()
	p.me = u.GetLogin()
	p.mu.Unlock()
	return domain.User{Login: u.GetLogin(), Name: u.GetName()}, nil
}

func (p *Provider) login(ctx context.Context) (string, error) {
	p.mu.Lock()
	me := p.me
	p.mu.Unlock()
	if me != "" {
		return me, nil
	}
	u, err := p.CurrentUser(ctx)
	return u.Login, err
}

func (p *Provider) ListRepos(ctx context.Context) ([]domain.Repo, error) {
	opts := &gh.RepositoryListByAuthenticatedUserOptions{
		Affiliation: "owner,collaborator,organization_member",
		Sort:        "pushed",
		ListOptions: gh.ListOptions{PerPage: perPage},
	}
	var out []domain.Repo
	for page := 0; page < maxPages; page++ {
		repos, resp, err := p.client.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err := p.track(resp, err); err != nil {
			return nil, err
		}
		for _, r := range repos {
			out = append(out, repoFrom(r))
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// apiState maps a domain state to the REST "state" parameter. GitHub has no
// "merged" state: merged PRs are closed ones, filtered afterwards.
func apiState(s domain.State) string {
	switch s {
	case domain.StateOpen:
		return "open"
	case domain.StateClosed, domain.StateMerged:
		return "closed"
	}
	return "all"
}

func prState(pr *gh.PullRequest) domain.State {
	switch {
	case pr.MergedAt != nil || pr.GetMerged():
		return domain.StateMerged
	case pr.GetState() == "closed":
		return domain.StateClosed
	}
	return domain.StateOpen
}

func (p *Provider) ListPullRequests(ctx context.Context, repo domain.Repo, q domain.PRQuery) ([]domain.PullRequest, error) {
	var me string
	if q.Scope != "" && q.Scope != domain.ScopeAll {
		var err error
		if me, err = p.login(ctx); err != nil {
			return nil, err
		}
	}
	opts := &gh.PullRequestListOptions{
		State:       apiState(q.State),
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: gh.ListOptions{PerPage: perPage},
	}
	var out []domain.PullRequest
	for page := 0; page < maxPages; page++ {
		prs, resp, err := p.client.PullRequests.List(ctx, repo.Owner, repo.Name, opts)
		if err := p.track(resp, err); err != nil {
			return nil, err
		}
		for _, pr := range prs {
			st := prState(pr)
			// "closed" and "merged" share one API state, so match exactly here.
			if q.State != "" && st != q.State {
				continue
			}
			if !inScope(pr, q.Scope, me) {
				continue
			}
			out = append(out, domain.PullRequest{
				Number:    pr.GetNumber(),
				Title:     pr.GetTitle(),
				Body:      pr.GetBody(),
				Author:    pr.GetUser().GetLogin(),
				State:     st,
				Draft:     pr.GetDraft(),
				Head:      pr.GetHead().GetRef(),
				Base:      pr.GetBase().GetRef(),
				URL:       pr.GetHTMLURL(),
				UpdatedAt: pr.GetUpdatedAt().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func inScope(pr *gh.PullRequest, scope domain.PRScope, me string) bool {
	switch scope {
	case domain.ScopeAuthored:
		return pr.GetUser().GetLogin() == me
	case domain.ScopeAssigned:
		for _, a := range pr.Assignees {
			if a.GetLogin() == me {
				return true
			}
		}
		return false
	case domain.ScopeReviewRequested:
		for _, r := range pr.RequestedReviewers {
			if r.GetLogin() == me {
				return true
			}
		}
		return false
	}
	return true
}

func (p *Provider) GetPullRequestDiff(ctx context.Context, repo domain.Repo, number int) (string, error) {
	d, resp, err := p.client.PullRequests.GetRaw(ctx, repo.Owner, repo.Name, number, gh.RawOptions{Type: gh.Diff})
	if err := p.track(resp, err); err != nil {
		return "", err
	}
	return d, nil
}

func (p *Provider) ListIssues(ctx context.Context, repo domain.Repo, state domain.State) ([]domain.Issue, error) {
	opts := &gh.IssueListByRepoOptions{
		State:       apiState(state),
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: gh.ListOptions{PerPage: perPage},
	}
	var out []domain.Issue
	for page := 0; page < maxPages; page++ {
		issues, resp, err := p.client.Issues.ListByRepo(ctx, repo.Owner, repo.Name, opts)
		if err := p.track(resp, err); err != nil {
			return nil, err
		}
		for _, i := range issues {
			if i.IsPullRequest() { // the issues API also returns PRs
				continue
			}
			labels := make([]string, 0, len(i.Labels))
			for _, l := range i.Labels {
				labels = append(labels, l.GetName())
			}
			st := domain.StateOpen
			if i.GetState() == "closed" {
				st = domain.StateClosed
			}
			out = append(out, domain.Issue{
				Number:    i.GetNumber(),
				Title:     i.GetTitle(),
				Body:      i.GetBody(),
				Author:    i.GetUser().GetLogin(),
				State:     st,
				Labels:    labels,
				URL:       i.GetHTMLURL(),
				UpdatedAt: i.GetUpdatedAt().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		// IssueListByRepoOptions embeds two pagination structs: be explicit.
		opts.ListOptions.Page = resp.NextPage
	}
	return out, nil
}

// ListComments returns the conversation comments of an issue or pull request.
// Inline review comments are not included.
func (p *Provider) ListComments(ctx context.Context, repo domain.Repo, number int) ([]domain.Comment, error) {
	opts := &gh.IssueListCommentsOptions{ListOptions: gh.ListOptions{PerPage: perPage}}
	var out []domain.Comment
	for page := 0; page < maxPages; page++ {
		cs, resp, err := p.client.Issues.ListComments(ctx, repo.Owner, repo.Name, number, opts)
		if err := p.track(resp, err); err != nil {
			return nil, err
		}
		for _, c := range cs {
			out = append(out, domain.Comment{
				Author:    c.GetUser().GetLogin(),
				Body:      c.GetBody(),
				CreatedAt: c.GetCreatedAt().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func (p *Provider) AddComment(ctx context.Context, repo domain.Repo, number int, body string) error {
	_, resp, err := p.client.Issues.CreateComment(ctx, repo.Owner, repo.Name, number, &gh.IssueComment{Body: &body})
	return p.track(resp, err)
}

// ApprovePullRequest submits an approving review (body may be empty).
func (p *Provider) ApprovePullRequest(ctx context.Context, repo domain.Repo, number int, body string) error {
	req := &gh.PullRequestReviewRequest{Event: gh.Ptr("APPROVE")}
	if body != "" {
		req.Body = &body
	}
	_, resp, err := p.client.PullRequests.CreateReview(ctx, repo.Owner, repo.Name, number, req)
	return p.track(resp, err)
}

func (p *Provider) MergePullRequest(ctx context.Context, repo domain.Repo, number int) error {
	res, resp, err := p.client.PullRequests.Merge(ctx, repo.Owner, repo.Name, number, "", nil)
	if err := p.track(resp, err); err != nil {
		return err
	}
	if !res.GetMerged() {
		return fmt.Errorf("merge refusé : %s", res.GetMessage())
	}
	return nil
}
