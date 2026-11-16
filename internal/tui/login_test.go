package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/zalando/go-keyring"

	"github.com/kevin/gitui/internal/auth"
	"github.com/kevin/gitui/internal/provider"
	"github.com/kevin/gitui/internal/provider/fake"
)

func authOpts(t *testing.T, clientID, baseURL string) *AuthOptions {
	t.Helper()
	keyring.MockInit()
	return &AuthOptions{
		Flow:        auth.Flow{ClientID: clientID, BaseURL: baseURL},
		Store:       auth.Store{},
		NewProvider: func(string) (provider.Provider, error) { return fake.New(), nil },
		ConfigPath:  "/cfg/config.yaml",
	}
}

// githubStub answers pending `pendings` times, then returns a token.
func githubStub(t *testing.T, pendings int32) string {
	t.Helper()
	var n atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/login/device/code", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"WXYZ-9999","verification_uri":"https://github.com/login/device","expires_in":900,"interval":5}`))
	})
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, _ *http.Request) {
		if n.Add(1) <= pendings {
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"gho_secret"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestLoginNeedsClientID(t *testing.T) {
	s := newLoginScreen(authOpts(t, "", ""))
	s2, _ := s.Update(s.checkStored()())
	out := s2.View()
	if !strings.Contains(out, "GITUI_CLIENT_ID") || !strings.Contains(out, "/cfg/config.yaml") {
		t.Fatalf("missing setup help:\n%s", out)
	}
}

func TestLoginUsesStoredToken(t *testing.T) {
	a := authOpts(t, "cid", "")
	if err := a.Store.Set("stored"); err != nil {
		t.Fatal(err)
	}
	s := newLoginScreen(a)
	_, cmd := s.Update(s.checkStored()())
	if got, ok := cmd().(authenticatedMsg); !ok || got.token != "stored" {
		t.Fatalf("got %#v", cmd())
	}
}

func TestLoginDeviceFlowEndToEnd(t *testing.T) {
	a := authOpts(t, "cid", githubStub(t, 1))
	s := newLoginScreen(a)

	_, _ = s.Update(storedTokenMsg{}) // no token -> starts the flow (gen 1)
	if s.state != loginStarting {
		t.Fatalf("state = %v", s.state)
	}
	_, cmd := s.Update(s.requestCode()())
	if s.state != loginWaiting || cmd == nil {
		t.Fatalf("state = %v", s.state)
	}
	if v := s.View(); !strings.Contains(v, "WXYZ-9999") || !strings.Contains(v, "github.com/login/device") {
		t.Fatalf("code/url not shown:\n%s", v)
	}

	// 1st poll: pending -> schedules another tick
	_, cmd = s.Update(s.poll()())
	if cmd == nil || s.state != loginWaiting {
		t.Fatal("should keep waiting")
	}
	// 2nd poll: token, persisted before announcing success
	_, cmd = s.Update(s.poll()())
	if got, ok := cmd().(authenticatedMsg); !ok || got.token != "gho_secret" {
		t.Fatalf("got %#v", cmd())
	}
	if tok, _ := a.Store.Get(); tok != "gho_secret" {
		t.Fatalf("token not persisted: %q", tok)
	}
}

func TestLoginIgnoresStaleGeneration(t *testing.T) {
	s := newLoginScreen(authOpts(t, "cid", githubStub(t, 0)))
	_, _ = s.Update(storedTokenMsg{})
	stale := s.requestCode()()
	s.gen++ // user retried meanwhile
	_, _ = s.Update(stale)
	if s.state == loginWaiting {
		t.Fatal("stale device code must be ignored")
	}
}

func TestLoginFailureAndRetry(t *testing.T) {
	s := newLoginScreen(authOpts(t, "cid", githubStub(t, 0)))
	_, _ = s.Update(storedTokenMsg{})
	_, _ = s.Update(s.requestCode()())
	_, _ = s.Update(pollResultMsg{gen: s.gen, err: auth.ErrDenied})
	if s.state != loginFailed || !strings.Contains(s.View(), "refusée") {
		t.Fatalf("state=%v\n%s", s.state, s.View())
	}
	_, cmd := s.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if s.state != loginStarting || cmd == nil {
		t.Fatalf("retry did not restart: %v", s.state)
	}
}

func TestAuthenticatedThenLogout(t *testing.T) {
	a := authOpts(t, "cid", "")
	_ = a.Store.Set("tok")
	m := New(Options{Auth: a})
	m = run(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})

	m = run(t, m, authenticatedMsg{token: "tok"})
	if _, ok := m.top().(*reposScreen); !ok || m.p == nil {
		t.Fatalf("expected repos screen, got %T", m.top())
	}

	m = run(t, m, tea.KeyPressMsg{Code: 'L', Text: "L"})
	if _, ok := m.top().(*loginScreen); !ok || m.p != nil {
		t.Fatalf("expected login screen after logout, got %T", m.top())
	}
	if tok, _ := a.Store.Get(); tok != "" {
		t.Fatalf("token survived logout: %q", tok)
	}
}
