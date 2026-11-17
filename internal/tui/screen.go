package tui

import (
	"charm.land/bubbles/v2/key"

	tea "charm.land/bubbletea/v2"
	"github.com/kevin/gitui/internal/browser"
	"github.com/kevin/gitui/internal/i18n"
)

// screen is one view in the navigation stack.
type screen interface {
	Title() string
	Init() tea.Cmd
	Update(tea.Msg) (screen, tea.Cmd)
	View() string
	SetSize(w, h int)
	// Capturing reports whether the screen wants raw key input (e.g. filter
	// typing), so the root must not treat esc/q as navigation.
	Capturing() bool
	Keys() []key.Binding
}

// tabbed is implemented by screens that show a tab bar. The root draws the
// bar and routes 1-9 and the switch key to SwitchTab.
type tabbed interface {
	Tabs() (tabs []tab, active int)
	// SwitchTab activates tab i; it may replace the screen.
	SwitchTab(i int) tea.Cmd
}

// headerer is implemented by screens that describe their context for the
// header (e.g. "github.com/o/r ⎇ main"); others show the breadcrumb.
type headerer interface{ Header() string }

type pushMsg struct{ s screen }
type popMsg struct{}
type errMsg struct{ err error }

func push(s screen) tea.Cmd  { return func() tea.Msg { return pushMsg{s} } }
func fail(err error) tea.Cmd { return func() tea.Msg { return errMsg{err} } }

// replaceMsg swaps the top screen without growing the stack (sibling views).
type replaceMsg struct{ s screen }

// infoMsg is a transient success message shown in the status line.
type infoMsg struct{ text string }

func replace(s screen) tea.Cmd { return func() tea.Msg { return replaceMsg{s} } }
func info(text string) tea.Cmd { return func() tea.Msg { return infoMsg{text} } }

// openURL is a variable so tests don't launch a browser.
var openURL = browser.Open

// openInBrowser returns a command that opens url and reports the outcome.
func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		if err := openURL(url); err != nil {
			return errMsg{err}
		}
		return infoMsg{i18n.T("opened in the browser")}
	}
}
