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
