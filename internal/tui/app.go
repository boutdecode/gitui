// Package tui contains the Bubble Tea models.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kevin/gitui/internal/provider"
	"github.com/kevin/gitui/internal/recents"
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
	user          string // login shown in the header
	autoTheme     bool   // follow the terminal background
	recents       *recents.Store
	transparent   bool // leave the terminal background alone
}

// Options configures the root model.
type Options struct {
	// Provider is used directly when Auth is nil (e.g. the fake provider).
	Provider provider.Provider
	// Auth enables the login flow; the provider is then built after login.
	Auth *AuthOptions
	// Recents remembers opened repositories; nil keeps them in memory only.
	Recents *recents.Store
	// Transparent keeps the terminal's own background instead of painting
	// the theme background.
	Transparent bool
	// Theme is "auto" (detect the terminal background), "light" or "dark".
	Theme string
}

func New(o Options) Model {
	m := Model{p: o.Provider, auth: o.Auth, help: help.New(), recents: o.Recents, transparent: o.Transparent}
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
		m.stack = []screen{newReposScreen(o.Provider, m.recents)}
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
	m.user = ""
	m.err = nil
	m.stack = []screen{newLoginScreen(m.auth)}
	m.layout()
	return m, m.top().Init()
}

// canLogout: only OAuth sessions can be logged out; a static token (env or
// config) is not ours to delete.
func (m Model) canLogout() bool {
	return m.auth != nil && m.p != nil && m.auth.StaticToken == ""
}

func (m Model) providerName() string {
	if m.p == nil {
		return "—"
	}
	return m.p.Name()
}

func (m Model) top() screen { return m.stack[len(m.stack)-1] }

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.top().Init(), fetchUser(m.p)}
	if m.autoTheme {
		cmds = append(cmds, tea.RequestBackgroundColor)
	}
	return tea.Batch(cmds...)
}

type userMsg struct{ login string }

// fetchUser asks the provider who is signed in, for the header. Failures are
// ignored: the header simply stays without a user.
func fetchUser(p provider.Provider) tea.Cmd {
	if p == nil {
		return nil
	}
	return func() tea.Msg {
		u, err := p.CurrentUser(context.Background())
		if err != nil {
			return nil
		}
		return userMsg{u.Login}
	}
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
	case userMsg:
		m.user = msg.login
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
		m.stack = []screen{newReposScreen(p, m.recents)}
		m.layout()
		return m, tea.Batch(m.top().Init(), fetchUser(p))
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
			case key.Matches(msg, keys.Logout) && m.canLogout():
				return m.logout()
			case key.Matches(msg, keys.Back) && len(m.stack) > 1:
				m.pop()
				return m, nil
			}
			if tb, ok := m.top().(tabbed); ok {
				if cmd, handled := m.switchTab(tb, msg); handled {
					return m, cmd
				}
			}
		}
	}
	s, cmd := m.top().Update(msg)
	m.stack[len(m.stack)-1] = s
	return m, cmd
}

// switchTab handles 1-9 and the switch key on a tabbed screen.
func (m *Model) switchTab(tb tabbed, msg tea.KeyPressMsg) (tea.Cmd, bool) {
	tabs, active := tb.Tabs()
	if key.Matches(msg, keySwitch) {
		return tb.SwitchTab((active + 1) % len(tabs)), true
	}
	if k := msg.String(); len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
		if i := int(k[0] - '1'); i < len(tabs) {
			if i == active {
				return nil, true
			}
			return tb.SwitchTab(i), true
		}
	}
	return nil, false
}

func (m *Model) pop() {
	m.stack = m.stack[:len(m.stack)-1]
	m.err = nil
	m.layout()
}

func (m Model) helpView() string {
	h := helpKeys{screen: m.top().Keys(), canBack: len(m.stack) > 1 && !m.top().Capturing(), canLogout: m.canLogout()}
	keep := 2 // help and quit always stay visible
	if h.canLogout {
		keep++
	}
	return helpBar(h.ShortHelp(), m.innerW(), m.help.ShowAll, keep)
}

// chromeHeight is every line the root draws around the screen body.
func (m Model) chromeHeight() int {
	n := 1 + 1 + 1 + lipgloss.Height(m.helpView()) // header, status, rule, help
	if _, ok := m.top().(tabbed); ok {
		n++
	}
	return n
}

// layout gives the active screen whatever height is left after chrome.
func (m *Model) layout() {
	m.top().SetSize(m.innerW(), max(m.innerH()-m.chromeHeight(), 1))
}

// Margin kept free around the app so it doesn't touch the terminal edges.
const (
	marginX = 2
	marginY = 1
)

// hasMargin is false on terminals too small to spare the space.
func (m Model) hasMargin() bool { return m.width >= 60 && m.height >= 16 }

func (m Model) innerW() int {
	if m.hasMargin() {
		return m.width - 2*marginX
	}
	return m.width
}

func (m Model) innerH() int {
	if m.hasMargin() {
		return m.height - 2*marginY
	}
	return m.height
}

// addMargin surrounds the frame with blank cells (painted later if needed).
func (m Model) addMargin(frame string) string {
	if !m.hasMargin() {
		return frame
	}
	pad := strings.Repeat(" ", marginX)
	lines := strings.Split(frame, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	blank := strings.Repeat("\n", marginY)
	return blank + strings.Join(lines, "\n") + blank
}

func (m Model) headerContext() string {
	if h, ok := m.top().(headerer); ok {
		return purpleStyle.Render(h.Header())
	}
	crumbs := make([]string, len(m.stack))
	for i, s := range m.stack {
		crumbs[i] = s.Title()
	}
	return purpleStyle.Render(m.providerName()) + dimStyle.Render(" › "+strings.Join(crumbs, " › "))
}

func (m Model) View() tea.View {
	parts := []string{headerView(m.innerW(), m.headerContext(), m.user)}
	if tb, ok := m.top().(tabbed); ok {
		tabs, active := tb.Tabs()
		parts = append(parts, tabsView(tabs, active))
	}

	status := statusStyle.Render(m.quotaText())
	if m.info != "" {
		status = infoStyle.Render("✓ " + m.info)
	}
	if m.err != nil {
		status = errorStyle.Render("erreur : " + m.err.Error())
		if errors.Is(m.err, provider.ErrUnauthorized) && m.auth != nil {
			if m.canLogout() {
				status += statusStyle.Render("  (L pour vous reconnecter)")
			} else {
				status += statusStyle.Render("  (vérifiez le token : " + m.auth.StaticSource + ")")
			}
		}
	}

	body := lipgloss.NewStyle().Height(max(m.innerH()-m.chromeHeight(), 1)).Render(m.top().View())
	parts = append(parts, body, status, rule(m.innerW(), ""), m.helpView())

	frame := m.addMargin(lipgloss.JoinVertical(lipgloss.Left, parts...))
	if !m.transparent {
		frame = paintBackground(frame, m.width, m.height)
	}
	v := tea.NewView(frame)
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
