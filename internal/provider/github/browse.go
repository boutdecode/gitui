package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	gh "github.com/google/go-github/v90/github"

	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/i18n"
)

// maxFileSize is the largest file the code viewer will download and show.
const maxFileSize = 1 << 20

func repoFrom(r *gh.Repository) domain.Repo {
	if r == nil {
		return domain.Repo{}
	}
	return domain.Repo{
		Owner:         r.GetOwner().GetLogin(),
		Name:          r.GetName(),
		Description:   r.GetDescription(),
		Private:       r.GetPrivate(),
		Stars:         r.GetStargazersCount(),
		URL:           r.GetHTMLURL(),
		UpdatedAt:     r.GetPushedAt().Time,
		Language:      r.GetLanguage(),
		Forks:         r.GetForksCount(),
		Fork:          r.GetFork(),
		Archived:      r.GetArchived(),
		DefaultBranch: r.GetDefaultBranch(),
		License:       r.GetLicense().GetSPDXID(),
		Topics:        r.Topics,
	}
}

func (p *Provider) ListBranches(ctx context.Context, repo domain.Repo) ([]string, error) {
	opts := &gh.BranchListOptions{ListOptions: gh.ListOptions{PerPage: perPage}}
	var out []string
	for page := 0; page < maxPages; page++ {
		bs, resp, err := p.client.Repositories.ListBranches(ctx, repo.Owner, repo.Name, opts)
		if err := p.track(resp, err); err != nil {
			return nil, err
		}
		for _, b := range bs {
			out = append(out, b.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func (p *Provider) ListTree(ctx context.Context, repo domain.Repo, ref, path string) ([]domain.TreeEntry, error) {
	_, dir, resp, err := p.client.Repositories.GetContents(ctx, repo.Owner, repo.Name, path,
		&gh.RepositoryContentGetOptions{Ref: ref})
	if err := p.track(resp, err); err != nil {
		return nil, err
	}
	out := make([]domain.TreeEntry, 0, len(dir))
	for _, e := range dir {
		t := domain.EntryFile
		if e.GetType() == "dir" {
			t = domain.EntryDir
		}
		out = append(out, domain.TreeEntry{Path: e.GetPath(), Name: e.GetName(), Type: t, Size: e.GetSize()})
	}
	return out, nil
}

func (p *Provider) GetFile(ctx context.Context, repo domain.Repo, ref, path string) (domain.FileContent, error) {
	f, _, resp, err := p.client.Repositories.GetContents(ctx, repo.Owner, repo.Name, path,
		&gh.RepositoryContentGetOptions{Ref: ref})
	if err := p.track(resp, err); err != nil {
		return domain.FileContent{}, err
	}
	if f == nil {
		return domain.FileContent{}, fmt.Errorf(i18n.T("%s is not a file"), path)
	}
	out := domain.FileContent{Path: f.GetPath(), Size: f.GetSize()}
	if out.Size > maxFileSize {
		out.TooLarge = true
	} else if text, err := f.GetContent(); err != nil {
		return domain.FileContent{}, err
	} else if !utf8.ValidString(text) || strings.IndexByte(text, 0) >= 0 {
		out.Binary = true
	} else {
		out.Content = text
	}
	// Last commit touching the file; best effort, the viewer works without it.
	cs, cresp, cerr := p.client.Repositories.ListCommits(ctx, repo.Owner, repo.Name, &gh.CommitsListOptions{
		SHA: ref, Path: path, ListOptions: gh.ListOptions{PerPage: 1},
	})
	if p.track(cresp, cerr) == nil && len(cs) > 0 {
		out.CommitSHA = cs[0].GetSHA()
		out.CommitTime = cs[0].GetCommit().GetCommitter().GetDate().Time
	}
	return out, nil
}

func (p *Provider) ListStarred(ctx context.Context) ([]domain.Repo, error) {
	opts := &gh.ActivityListStarredOptions{
		Sort: "updated", ListOptions: gh.ListOptions{PerPage: perPage},
	}
	var out []domain.Repo
	for page := 0; page < maxPages; page++ {
		rs, resp, err := p.client.Activity.ListStarred(ctx, "", opts)
		if err := p.track(resp, err); err != nil {
			return nil, err
		}
		for _, s := range rs {
			r := repoFrom(s.GetRepository())
			r.Starred = true
			out = append(out, r)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func (p *Provider) SetStarred(ctx context.Context, repo domain.Repo, starred bool) error {
	var (
		resp *gh.Response
		err  error
	)
	if starred {
		resp, err = p.client.Activity.Star(ctx, repo.Owner, repo.Name)
	} else {
		resp, err = p.client.Activity.Unstar(ctx, repo.Owner, repo.Name)
	}
	return p.track(resp, err)
}

func (p *Provider) ListOrgs(ctx context.Context) ([]domain.Org, error) {
	opts := &gh.ListOptions{PerPage: perPage}
	var out []domain.Org
	for page := 0; page < maxPages; page++ {
		os, resp, err := p.client.Organizations.List(ctx, "", opts)
		if err := p.track(resp, err); err != nil {
			return nil, err
		}
		for _, o := range os {
			out = append(out, domain.Org{Login: o.GetLogin(), Description: o.GetDescription()})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func (p *Provider) ListOrgRepos(ctx context.Context, org string) ([]domain.Repo, error) {
	opts := &gh.RepositoryListByOrgOptions{Sort: "pushed", ListOptions: gh.ListOptions{PerPage: perPage}}
	var out []domain.Repo
	for page := 0; page < maxPages; page++ {
		rs, resp, err := p.client.Repositories.ListByOrg(ctx, org, opts)
		if err := p.track(resp, err); err != nil {
			return nil, err
		}
		for _, r := range rs {
			out = append(out, repoFrom(r))
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// CountOpen returns the number of open pull requests and issues using the
// search API (one request each, per_page=1: only the total is read).
func (p *Provider) CountOpen(ctx context.Context, repo domain.Repo) (prs, issues int, err error) {
	count := func(kind string) (int, error) {
		res, resp, err := p.client.Search.Issues(ctx,
			fmt.Sprintf("repo:%s/%s is:%s is:open", repo.Owner, repo.Name, kind),
			&gh.SearchOptions{ListOptions: gh.ListOptions{PerPage: 1}})
		if err := p.track(resp, err); err != nil {
			return 0, err
		}
		return res.GetTotal(), nil
	}
	if prs, err = count("pr"); err != nil {
		return 0, 0, err
	}
	if issues, err = count("issue"); err != nil {
		return 0, 0, err
	}
	return prs, issues, nil
}

func (p *Provider) GetPullRequest(ctx context.Context, repo domain.Repo, number int) (domain.PullRequest, error) {
	pr, resp, err := p.client.PullRequests.Get(ctx, repo.Owner, repo.Name, number)
	if err := p.track(resp, err); err != nil {
		return domain.PullRequest{}, err
	}
	return domain.PullRequest{
		Number:       pr.GetNumber(),
		Title:        pr.GetTitle(),
		Body:         pr.GetBody(),
		Author:       pr.GetUser().GetLogin(),
		State:        prState(pr),
		Draft:        pr.GetDraft(),
		Head:         pr.GetHead().GetRef(),
		Base:         pr.GetBase().GetRef(),
		URL:          pr.GetHTMLURL(),
		UpdatedAt:    pr.GetUpdatedAt().Time,
		HeadSHA:      pr.GetHead().GetSHA(),
		Additions:    pr.GetAdditions(),
		Deletions:    pr.GetDeletions(),
		ChangedFiles: pr.GetChangedFiles(),
		Comments:     pr.GetComments(),
	}, nil
}

func (p *Provider) ListPullRequestFiles(ctx context.Context, repo domain.Repo, number int) ([]domain.PRFile, error) {
	opts := &gh.ListOptions{PerPage: perPage}
	var out []domain.PRFile
	for page := 0; page < maxPages; page++ {
		fs, resp, err := p.client.PullRequests.ListFiles(ctx, repo.Owner, repo.Name, number, opts)
		if err := p.track(resp, err); err != nil {
			return nil, err
		}
		for _, f := range fs {
			out = append(out, domain.PRFile{
				Name: f.GetFilename(), Status: f.GetStatus(),
				Additions: f.GetAdditions(), Deletions: f.GetDeletions(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func (p *Provider) ListChecks(ctx context.Context, repo domain.Repo, ref string) ([]domain.Check, error) {
	if ref == "" {
		return nil, errors.New(i18n.T("missing commit reference"))
	}
	res, resp, err := p.client.Checks.ListCheckRunsForRef(ctx, repo.Owner, repo.Name, ref,
		&gh.ListCheckRunsOptions{ListOptions: gh.ListOptions{PerPage: perPage}})
	if err := p.track(resp, err); err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	out := make([]domain.Check, 0, len(res.CheckRuns))
	for _, c := range res.CheckRuns {
		ch := domain.Check{Name: c.GetName(), Status: checkStatus(c)}
		if c.StartedAt != nil && c.CompletedAt != nil {
			ch.Duration = c.CompletedAt.Sub(c.StartedAt.Time).Round(time.Second)
		}
		out = append(out, ch)
	}
	return out, nil
}

func checkStatus(c *gh.CheckRun) domain.CheckStatus {
	if c.GetStatus() != "completed" {
		return domain.CheckPending
	}
	switch c.GetConclusion() {
	case "success", "neutral", "skipped":
		return domain.CheckSuccess
	}
	return domain.CheckFailure
}
