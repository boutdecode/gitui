// Package domain holds provider-neutral types shared by the TUI and providers.
package domain

import "time"

type User struct {
	Login string
	Name  string
}

type Repo struct {
	Owner       string
	Name        string
	Description string
	Private     bool
	Stars       int
	URL         string // web page, for "open in browser"
	UpdatedAt   time.Time

	Language      string
	Forks         int
	Fork          bool
	Archived      bool
	DefaultBranch string
	License       string
	Topics        []string
	Starred       bool // starred by the authenticated user (a "favorite")
}

func (r Repo) FullName() string { return r.Owner + "/" + r.Name }

type State string

const (
	StateOpen   State = "open"
	StateClosed State = "closed"
	StateMerged State = "merged"
)

type PullRequest struct {
	Number     int
	Title      string
	Body       string
	Author     string
	State      State
	Draft      bool
	Head, Base string
	URL        string
	UpdatedAt  time.Time

	// Filled by GetPullRequest only (the list endpoint omits them).
	HeadSHA      string
	Additions    int
	Deletions    int
	ChangedFiles int
	Comments     int
}

type Issue struct {
	Number    int
	Title     string
	Body      string
	Author    string
	State     State
	Labels    []string
	URL       string
	UpdatedAt time.Time
}

type Comment struct {
	Author    string
	Body      string
	CreatedAt time.Time
}

// PRScope narrows a pull request listing to the current user's involvement.
type PRScope string

const (
	ScopeAll             PRScope = "all"
	ScopeAuthored        PRScope = "authored"
	ScopeAssigned        PRScope = "assigned"
	ScopeReviewRequested PRScope = "review-requested"
)

// PRQuery selects which pull requests to list. A zero State means any state.
type PRQuery struct {
	State State
	Scope PRScope // zero value behaves like ScopeAll
}

// RateLimit is the API quota as last reported by the provider.
type RateLimit struct {
	Limit, Remaining int
	Reset            time.Time
}

// Org is an organization the authenticated user belongs to.
type Org struct {
	Login       string
	Description string
}

// EntryType tells directories from files in a repository tree.
type EntryType string

const (
	EntryDir  EntryType = "dir"
	EntryFile EntryType = "file"
)

// TreeEntry is one item of a directory listing. Path is relative to the
// repository root.
type TreeEntry struct {
	Path string
	Name string
	Type EntryType
	Size int
}

// FileContent is a file as shown in the code viewer. Binary or oversized
// files carry no Content.
type FileContent struct {
	Path       string
	Size       int
	Content    string
	Binary     bool
	TooLarge   bool
	CommitSHA  string
	CommitTime time.Time
}

// CheckStatus is the outcome of a CI check, or of a set of checks.
type CheckStatus string

const (
	CheckNone    CheckStatus = ""
	CheckPending CheckStatus = "pending"
	CheckSuccess CheckStatus = "success"
	CheckFailure CheckStatus = "failure"
)

type Check struct {
	Name     string
	Status   CheckStatus
	Duration time.Duration
}

// CombineChecks reduces checks to one status: any failure wins, then any
// pending one, then success.
func CombineChecks(cs []Check) CheckStatus {
	if len(cs) == 0 {
		return CheckNone
	}
	out := CheckSuccess
	for _, c := range cs {
		switch c.Status {
		case CheckFailure:
			return CheckFailure
		case CheckPending, CheckNone:
			out = CheckPending
		}
	}
	return out
}

// PRFile is one file changed by a pull request. Status is "added",
// "modified", "removed" or "renamed".
type PRFile struct {
	Name      string
	Status    string
	Additions int
	Deletions int
}
