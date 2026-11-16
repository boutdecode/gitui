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

func (s *loginScreen) Init() tea.Cmd {
	return tea.Batch(s.checkStored(), s.spinner.Tick)
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
			return s, s.start()
		}
	}
	return s, nil
}

func (s *loginScreen) View() string {
	var body string
	switch s.state {
	case loginChecking:
		body = s.spinner.View() + " recherche d'une session existante…"
	case loginNeedClient:
		body = "Aucun client ID GitHub configuré.\n\n" +
			"1. Créez une OAuth App (Settings → Developer settings → OAuth Apps)\n" +
			"   et cochez « Enable Device Flow ».\n" +
			"2. Renseignez son client ID :\n" +
			"     - variable d'environnement  GITUI_CLIENT_ID\n"
		if s.a.ConfigPath != "" {
			body += fmt.Sprintf("     - ou client_id = \"…\" dans %s\n", s.a.ConfigPath)
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
		body = errorStyle.Render("Connexion impossible : "+errText(s.err)) + "\n\n" +
			hintStyle.Render("Appuyez sur r pour réessayer.")
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
