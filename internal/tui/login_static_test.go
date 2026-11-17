package tui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/zalando/go-keyring"

	"github.com/kevin/gitui/internal/auth"
	"github.com/kevin/gitui/internal/domain"
	"github.com/kevin/gitui/internal/provider"
	"github.com/kevin/gitui/internal/provider/fake"
)

const secret = "ghp_SUPER_SECRET_VALUE"

// userProvider fails CurrentUser on demand and records the token it was built with.
type userProvider struct {
	*fake.Provider
	err error
}

func (u *userProvider) CurrentUser(context.Context) (domain.User, error) {
	if u.err != nil {
		return domain.User{}, u.err
	}
	return domain.User{Login: "octocat"}, nil
}

// staticOpts builds options with a static token, a recording provider factory
// and an OAuth server that must never be contacted.
func staticOpts(t *testing.T, verifyErr *error) (*AuthOptions, *int32, *string) {
	t.Helper()
	keyring.MockInit()
	var oauthHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		atomic.AddInt32(&oauthHits, 1)
	}))
	t.Cleanup(srv.Close)
	var seen string
	return &AuthOptions{
		Flow:  auth.Flow{ClientID: "cid", BaseURL: srv.URL},
		Store: auth.Store{},
		NewProvider: func(token string) (provider.Provider, error) {
			seen = token
			return &userProvider{Provider: fake.New(), err: *verifyErr}, nil
		},
		ConfigPath:   "/cfg/config.yaml",
		StaticToken:  secret,
		StaticSource: "GITUI_TOKEN",
	}, &oauthHits, &seen
}

func TestStaticTokenValidSkipsOAuthAndStore(t *testing.T) {
	var verifyErr error
	a, oauthHits, seen := staticOpts(t, &verifyErr)
	s := newLoginScreen(a)
	if s.Init() == nil {
		t.Fatal("Init should verify the token")
	}
	_, cmd := s.Update(s.verifyStatic()())
	got, ok := cmd().(authenticatedMsg)
	if !ok || got.token != secret || *seen != secret {
		t.Fatalf("got %#v, provider built with %q", cmd(), *seen)
	}
	if tok, _ := a.Store.Get(); tok != "" {
		t.Fatalf("a static token must never be stored, store has %q", tok)
	}
	if *oauthHits != 0 {
		t.Fatalf("OAuth server was contacted %d times", *oauthHits)
	}
}

func TestStaticTokenIgnoresStoredSession(t *testing.T) {
	var verifyErr error
	a, _, _ := staticOpts(t, &verifyErr)
	_ = a.Store.Set("old-oauth-token")
	s := newLoginScreen(a)
	_, cmd := s.Update(s.verifyStatic()())
	if got := cmd().(authenticatedMsg); got.token != secret {
		t.Fatalf("env/config token must win over the stored session, got %q", got.token)
	}
	if tok, _ := a.Store.Get(); tok != "old-oauth-token" {
		t.Fatalf("stored session must be left untouched, got %q", tok)
	}
}

func TestStaticTokenRejected(t *testing.T) {
	verifyErr := provider.ErrUnauthorized
	a, oauthHits, _ := staticOpts(t, &verifyErr)
	s := newLoginScreen(a)
	_, cmd := s.Update(s.verifyStatic()())
	if cmd != nil {
		t.Fatalf("no follow-up expected, got %v", cmd())
	}
	if s.state != loginFailed {
		t.Fatalf("state = %v", s.state)
	}
	v := s.View()
	for _, want := range []string{"refusé", "GITUI_TOKEN"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, secret) || strings.Contains(s.err.Error(), secret) {
		t.Fatal("the token must never appear in messages")
	}
	if *oauthHits != 0 {
		t.Fatal("a bad explicit token must not fall back to the OAuth flow")
	}
}

func TestStaticTokenNetworkErrorIsDistinguished(t *testing.T) {
	verifyErr := errors.New("dial tcp: no route to host")
	a, _, _ := staticOpts(t, &verifyErr)
	s := newLoginScreen(a)
	s.Update(s.verifyStatic()())
	v := s.View()
	if !strings.Contains(v, "impossible de vérifier") || !strings.Contains(v, "no route to host") {
		t.Fatalf("network failure not explained:\n%s", v)
	}
	if strings.Contains(v, "refusé") {
		t.Fatal("a network error is not a rejected token")
	}
}

func TestStaticTokenRetryRechecksWithoutOAuth(t *testing.T) {
	verifyErr := provider.ErrUnauthorized
	a, oauthHits, _ := staticOpts(t, &verifyErr)
	s := newLoginScreen(a)
	s.Update(s.verifyStatic()())
	if s.state != loginFailed {
		t.Fatal("setup: expected failure")
	}

	_, cmd := s.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if s.state != loginChecking || cmd == nil {
		t.Fatalf("retry should re-verify, state=%v", s.state)
	}
	// the user fixed the token's validity in the meantime
	verifyErr = nil
	_, cmd = s.Update(s.verifyStatic()())
	if _, ok := cmd().(authenticatedMsg); !ok {
		t.Fatalf("second check should succeed, got %#v", cmd())
	}
	if *oauthHits != 0 {
		t.Fatal("retry must not start the device flow")
	}
}

func TestStaticVerificationStaleResultIgnored(t *testing.T) {
	var verifyErr error
	a, _, _ := staticOpts(t, &verifyErr)
	s := newLoginScreen(a)
	stale := s.verifyStatic()()
	s.gen++
	if _, cmd := s.Update(stale); cmd != nil {
		t.Fatal("stale verification must be ignored")
	}
}

func TestNeedClientHelpMentionsToken(t *testing.T) {
	a := authOpts(t, "", "")
	s := newLoginScreen(a)
	s.Update(s.checkStored()())
	v := s.View()
	for _, want := range []string{"GITUI_TOKEN", "token:", "GITUI_CLIENT_ID"} {
		if !strings.Contains(v, want) {
			t.Errorf("help missing %q:\n%s", want, v)
		}
	}
}

func authedModel(t *testing.T, a *AuthOptions) Model {
	t.Helper()
	m := New(Options{Auth: a})
	m = run(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	return run(t, m, authenticatedMsg{token: "whatever"})
}

func TestLogoutUnavailableWithStaticToken(t *testing.T) {
	var verifyErr error
	a, _, _ := staticOpts(t, &verifyErr)
	_ = a.Store.Set("stored") // must survive the keypress below
	m := authedModel(t, a)
	if _, ok := m.top().(*reposScreen); !ok {
		t.Fatalf("expected repos screen, got %T", m.top())
	}
	if strings.Contains(m.helpView(), "déconnexion") {
		t.Fatal("logout must not be advertised with a static token")
	}
	m = run(t, m, tea.KeyPressMsg{Code: 'L', Text: "L"})
	if _, ok := m.top().(*reposScreen); !ok || m.p == nil {
		t.Fatalf("L must be ignored, got %T", m.top())
	}
	if tok, _ := a.Store.Get(); tok != "stored" {
		t.Fatalf("L must not delete anything, store has %q", tok)
	}
}

func TestLogoutStillAvailableWithOAuth(t *testing.T) {
	a := authOpts(t, "cid", "")
	m := authedModel(t, a)
	if !strings.Contains(m.helpView(), "déconnexion") {
		t.Fatal("logout should be advertised for OAuth sessions")
	}
}

func TestUnauthorizedHintDependsOnLogout(t *testing.T) {
	var verifyErr error
	a, _, _ := staticOpts(t, &verifyErr)
	m := authedModel(t, a)
	m = run(t, m, errMsg{provider.ErrUnauthorized})
	if v := view(m); !strings.Contains(v, "vérifiez le token : GITUI_TOKEN") || strings.Contains(v, "L pour") {
		t.Fatalf("static-token hint wrong:\n%s", v)
	}

	m = authedModel(t, authOpts(t, "cid", ""))
	m = run(t, m, errMsg{provider.ErrUnauthorized})
	if v := view(m); !strings.Contains(v, "L pour vous reconnecter") {
		t.Fatalf("OAuth hint wrong:\n%s", v)
	}
}
