// Package tui contains the Bubble Tea models.
package tui

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kevin/gitui/internal/provider"
)

// Model is the root model: it owns the navigation stack, header, status line
// and help, and forwards everything else to the top screen.
type Model struct {
	p             provider.Provider // nil until authenticated
	auth          *AuthOptions      // nil: no login (preset provider)
	stack         []screen
	help          help.Model
	width, height int
	err           error
	info          string // transient success message
	autoTheme     bool   // follow the terminal background
}

// Options configures the root model.
type Options struct {
	// Provider is used directly when Auth is nil (e.g. the fake provider).
	Provider provider.Provider
	// Auth enables the login flow; the provider is then built after login.
	Auth *AuthOptions
	// Theme is "auto" (detect the terminal background), "light" or "dark".
	Theme string
}

func New(o Options) Model {
	m := Model{p: o.Provider, auth: o.Auth, help: help.New()}
	switch o.Theme {
	case "light", "dark":
		m.setTheme(o.Theme == "dark")
	default:
		m.autoTheme = true
		m.setTheme(true) // until the terminal answers
	}
	if o.Auth != nil {
		m.p = nil
		m.stack = []screen{newLoginScreen(o.Auth)}
	} else {
		m.stack = []screen{newReposScreen(o.Provider)}
	}
	return m
}

// logout forgets the stored token and returns to the login screen.
func (m Model) logout() (tea.Model, tea.Cmd) {
	if err := m.auth.Store.Delete(); err != nil {
		m.err = err
		return m, nil
	}
	m.p = nil
	m.err = nil
	m.stack = []screen{newLoginScreen(m.auth)}
	m.layout()
	return m, m.top().Init()
}

func (m Model) providerName() string {
	if m.p == nil {
		return "—"
	}
	return m.p.Name()
}

func (m Model) top() screen { return m.stack[len(m.stack)-1] }

func (m Model) Init() tea.Cmd {
	if m.autoTheme {
		return tea.Batch(m.top().Init(), tea.RequestBackgroundColor)
	}
	return m.top().Init()
}

// setTheme restyles the global styles and every screen on the stack.
func (m *Model) setTheme(dark bool) {
	applyTheme(dark)
	m.help.Styles = help.DefaultStyles(dark)
	for _, s := range m.stack {
		if r, ok := s.(restyler); ok {
			r.Restyle(dark)
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
		m.layout()
		return m, nil
	case pushMsg:
		m.stack = append(m.stack, msg.s)
		m.err = nil
		m.layout()
		return m, msg.s.Init()
	case replaceMsg:
		m.stack[len(m.stack)-1] = msg.s
		m.err, m.info = nil, ""
		m.layout()
		return m, msg.s.Init()
	case infoMsg:
		m.info, m.err = msg.text, nil
		return m, nil
	case tea.BackgroundColorMsg:
		if m.autoTheme {
			m.setTheme(msg.IsDark())
			m.layout()
		}
		return m, nil
	case popMsg:
		m.pop()
		return m, nil
	case authenticatedMsg:
		p, err := m.auth.NewProvider(msg.token)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.p = p
		m.err = nil
		m.stack = []screen{newReposScreen(p)}
		m.layout()
		return m, m.top().Init()
	case errMsg:
		m.err = msg.err
		// let the screen stop its spinner too
	case tea.KeyPressMsg:
		m.err, m.info = nil, ""
		if msg.String() == "ctrl+c" { // never rebindable, never swallowed
			return m, tea.Quit
		}
		if !m.top().Capturing() {
			switch {
			case key.Matches(msg, keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, keys.Help):
				m.help.ShowAll = !m.help.ShowAll
				m.layout()
				return m, nil
			case key.Matches(msg, keys.Logout) && m.auth != nil && m.p != nil:
				return m.logout()
			case key.Matches(msg, keys.Back) && len(m.stack) > 1:
				m.pop()
				return m, nil
			}
		}
	}
	s, cmd := m.top().Update(msg)
	m.stack[len(m.stack)-1] = s
	return m, cmd
}

func (m *Model) pop() {
	m.stack = m.stack[:len(m.stack)-1]
	m.err = nil
	m.layout()
}

func (m Model) helpView() string {
	return m.help.View(helpKeys{screen: m.top().Keys(), canBack: len(m.stack) > 1 && !m.top().Capturing(), canLogout: m.auth != nil && m.p != nil})
}

// layout gives the active screen whatever height is left after chrome.
func (m *Model) layout() {
	chrome := 2 + lipgloss.Height(m.helpView()) // header + status + help
	h := m.height - chrome
	if h < 1 {
		h = 1
	}
	m.top().SetSize(m.width, h)
}

func (m Model) View() tea.View {
	crumbs := make([]string, len(m.stack))
	for i, s := range m.stack {
		crumbs[i] = s.Title()
	}
	header := headerStyle.Render("gitui") + " " +
		crumbStyle.Render(m.providerName()+" › "+strings.Join(crumbs, " › "))

	status := statusStyle.Render(m.quotaText())
	if m.info != "" {
		status = infoStyle.Render("✓ " + m.info)
	}
	if m.err != nil {
		status = errorStyle.Render("erreur : " + m.err.Error())
		if errors.Is(m.err, provider.ErrUnauthorized) && m.auth != nil {
			status += statusStyle.Render("  (L pour vous reconnecter)")
		}
	}

	body := lipgloss.NewStyle().Height(max(m.height-2-lipgloss.Height(m.helpView()), 1)).
		Render(m.top().View())

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, body, status, m.helpView()))
	v.AltScreen = true
	return v
}

// quotaText shows the remaining API quota when the provider reports one.
func (m Model) quotaText() string {
	rl, ok := m.p.(provider.RateLimiter)
	if !ok {
		return " "
	}
	q, ok := rl.RateLimit()
	if !ok {
		return " "
	}
	return fmt.Sprintf("API %d/%d", q.Remaining, q.Limit)
}
