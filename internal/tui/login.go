package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kevin/gitui/internal/auth"
	"github.com/kevin/gitui/internal/provider"
)

// AuthOptions wires authentication into the TUI. When nil in Options, the TUI
// skips login and uses Options.Provider directly.
type AuthOptions struct {
	Flow        auth.Flow
	Store       auth.TokenStore
	NewProvider func(token string) (provider.Provider, error)
	ConfigPath  string // only used in help text

	// StaticToken, when set, is a personal access token that replaces the
	// OAuth login. It is verified but never stored. StaticSource names where
	// it came from (e.g. "GITUI_TOKEN"), for messages only.
	StaticToken  string
	StaticSource string
}

type loginState int

const (
	loginChecking   loginState = iota // looking for a stored token
	loginNeedClient                   // no client_id configured
	loginStarting                     // requesting a device code
	loginWaiting                      // user must authorize in the browser
	loginFailed
)

// Messages carry the screen's generation so results from an abandoned
// attempt (e.g. after retry) are ignored.
type (
	storedTokenMsg struct {
		token string
		err   error
	}
	// staticVerifiedMsg is the outcome of checking AuthOptions.StaticToken.
	staticVerifiedMsg struct {
		gen int
		err error
	}
	deviceCodeMsg struct {
		gen int
		dc  auth.DeviceCode
		err error
	}
	pollTickMsg   struct{ gen int }
	pollResultMsg struct {
		gen int
		res auth.PollResult
		err error
	}
	authenticatedMsg struct{ token string }
)

const slowDownStep = 5 * time.Second

type loginScreen struct {
	a        *AuthOptions
	state    loginState
	gen      int
	dc       auth.DeviceCode
	interval time.Duration
	deadline time.Time
	err      error
	spinner  spinner.Model
	w, h     int

	copyKey, retryKey key.Binding
}

func newLoginScreen(a *AuthOptions) *loginScreen {
	return &loginScreen{
		a:        a,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		copyKey:  bind("copy"),
		retryKey: bind("retry"),
	}
}

func (s *loginScreen) Title() string { return "Connexion" }

func (s *loginScreen) static() bool { return s.a.StaticToken != "" }

func (s *loginScreen) Init() tea.Cmd {
	if s.static() {
		return tea.Batch(s.verifyStatic(), s.spinner.Tick)
	}
	return tea.Batch(s.checkStored(), s.spinner.Tick)
}

// verifyStatic checks the personal access token against GitHub. It never
// touches the token store: a static token is not ours to persist.
func (s *loginScreen) verifyStatic() tea.Cmd {
	gen := s.gen
	return func() tea.Msg {
		p, err := s.a.NewProvider(s.a.StaticToken)
		if err != nil {
			return staticVerifiedMsg{gen, err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, err = p.CurrentUser(ctx)
		return staticVerifiedMsg{gen, err}
	}
}

// staticErr words a verification failure without ever echoing the token.
func (s *loginScreen) staticErr(err error) error {
	src := s.a.StaticSource
	if src == "" {
		src = "token"
	}
	if errors.Is(err, provider.ErrUnauthorized) {
		return fmt.Errorf("token refusé par GitHub (invalide, expiré ou révoqué), source : %s", src)
	}
	return fmt.Errorf("impossible de vérifier le token (source : %s) : %w", src, err)
}

func (s *loginScreen) checkStored() tea.Cmd {
	return func() tea.Msg {
		tok, err := s.a.Store.Get()
		return storedTokenMsg{tok, err}
	}
}

func (s *loginScreen) requestCode() tea.Cmd {
	gen := s.gen
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		dc, err := s.a.Flow.Start(ctx)
		return deviceCodeMsg{gen, dc, err}
	}
}

func (s *loginScreen) poll() tea.Cmd {
	gen, dc := s.gen, s.dc
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		res, err := s.a.Flow.Check(ctx, dc)
		if err == nil && res.Token != "" {
			// Persist before announcing success so a crash can't lose the token.
			if serr := s.a.Store.Set(res.Token); serr != nil {
				return pollResultMsg{gen, auth.PollResult{}, fmt.Errorf("stockage du token : %w", serr)}
			}
		}
		return pollResultMsg{gen, res, err}
	}
}

func (s *loginScreen) tick() tea.Cmd {
	gen := s.gen
	return tea.Tick(s.interval, func(time.Time) tea.Msg { return pollTickMsg{gen} })
}

func (s *loginScreen) start() tea.Cmd {
	s.gen++
	s.err = nil
	s.state = loginStarting
	return tea.Batch(s.requestCode(), s.spinner.Tick)
}

func (s *loginScreen) fail(err error) {
	s.state = loginFailed
	s.err = err
}

func (s *loginScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case storedTokenMsg:
		switch {
		case msg.token != "":
			return s, func() tea.Msg { return authenticatedMsg{msg.token} }
		case s.a.Flow.ClientID == "":
			s.state = loginNeedClient
			return s, nil
		default:
			// A keyring read error is not fatal: just log in again.
			return s, s.start()
		}

	case staticVerifiedMsg:
		if msg.gen != s.gen || s.state != loginChecking {
			return s, nil
		}
		if msg.err != nil {
			s.fail(s.staticErr(msg.err))
			return s, nil
		}
		token := s.a.StaticToken
		return s, func() tea.Msg { return authenticatedMsg{token} }

	case deviceCodeMsg:
		if msg.gen != s.gen {
			return s, nil
		}
		if msg.err != nil {
			s.fail(msg.err)
			return s, nil
		}
		s.dc = msg.dc
		s.interval = time.Duration(msg.dc.Interval) * time.Second
		s.deadline = time.Now().Add(time.Duration(msg.dc.ExpiresIn) * time.Second)
		s.state = loginWaiting
		return s, s.tick()

	case pollTickMsg:
		if msg.gen != s.gen || s.state != loginWaiting {
			return s, nil
		}
		return s, s.poll()

	case pollResultMsg:
		if msg.gen != s.gen || s.state != loginWaiting {
			return s, nil
		}
		if msg.err != nil {
			s.fail(msg.err)
			return s, nil
		}
		if msg.res.Token != "" {
			return s, func() tea.Msg { return authenticatedMsg{msg.res.Token} }
		}
		if msg.res.SlowDown {
			s.interval += slowDownStep
		}
		if !s.deadline.IsZero() && time.Now().After(s.deadline) {
			s.fail(auth.ErrExpired)
			return s, nil
		}
		return s, s.tick()

	case spinner.TickMsg:
		if s.state != loginChecking && s.state != loginStarting {
			return s, nil
		}
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.copyKey) && s.state == loginWaiting:
			return s, tea.SetClipboard(s.dc.UserCode)
		case key.Matches(msg, s.retryKey) && s.state == loginFailed:
			if s.static() {
				// A bad explicit token is re-checked, never bypassed with OAuth.
				s.gen++
				s.err = nil
				s.state = loginChecking
				return s, tea.Batch(s.verifyStatic(), s.spinner.Tick)
			}
			return s, s.start()
		}
	}
	return s, nil
}

func (s *loginScreen) View() string {
	var body string
	switch s.state {
	case loginChecking:
		if s.static() {
			body = s.spinner.View() + " vérification du token…"
		} else {
			body = s.spinner.View() + " recherche d'une session existante…"
		}
	case loginNeedClient:
		body = "Aucune méthode de connexion configurée.\n\n" +
			"A. Token d'accès personnel (le plus simple) :\n" +
			"     - variable d'environnement  GITUI_TOKEN\n"
		if s.a.ConfigPath != "" {
			body += fmt.Sprintf("     - ou token: … dans %s (en clair sur disque)\n", s.a.ConfigPath)
		}
		body += "\nB. OAuth App (device flow) :\n" +
			"     1. Settings → Developer settings → OAuth Apps, cochez « Enable Device Flow »\n" +
			"     2. renseignez son client ID : variable GITUI_CLIENT_ID\n"
		if s.a.ConfigPath != "" {
			body += fmt.Sprintf("        ou client_id: … dans %s\n", s.a.ConfigPath)
		}
		body += "\nPuis relancez gitui."
	case loginStarting:
		body = s.spinner.View() + " demande d'un code à GitHub…"
	case loginWaiting:
		body = fmt.Sprintf("Ouvrez %s\net saisissez le code :\n\n%s\n\n%s",
			urlStyle.Render(s.dc.VerificationURI),
			codeStyle.Render(s.dc.UserCode),
			hintStyle.Render("En attente de votre autorisation…"))
	case loginFailed:
		hint := "Appuyez sur r pour réessayer."
		if s.static() {
			hint = "Corrigez le token puis relancez gitui, ou appuyez sur r pour revérifier."
		}
		body = errorStyle.Render("Connexion impossible : "+errText(s.err)) + "\n\n" +
			hintStyle.Render(hint)
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func errText(err error) string {
	if errors.Is(err, auth.ErrExpired) {
		return "le code a expiré"
	}
	return err.Error()
}

func (s *loginScreen) SetSize(w, h int) { s.w, s.h = w, h }
func (s *loginScreen) Capturing() bool  { return false }
func (s *loginScreen) Keys() []key.Binding {
	switch s.state {
	case loginWaiting:
		return []key.Binding{s.copyKey}
	case loginFailed:
		return []key.Binding{s.retryKey}
	}
	return nil
}
