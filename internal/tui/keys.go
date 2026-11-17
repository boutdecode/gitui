package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"

	"github.com/kevin/gitui/internal/i18n"
)

// def describes one user-rebindable action.
type def struct {
	keys  []string
	label string // shown in the help bar when not overridden
	desc  string
}

// defs is the single source of truth for every rebindable action. Keys of
// widgets we embed (list navigation, viewport scrolling, text editing) are
// not part of it.
var defs = map[string]def{
	"quit":       {[]string{"q", "ctrl+c"}, "q", "quit"},
	"help":       {[]string{"?"}, "?", "help"},
	"back":       {[]string{"esc", "backspace"}, "esc", "back"},
	"logout":     {[]string{"L"}, "L", "log out"},
	"open":       {[]string{"enter"}, "enter", "open"},
	"switch":     {[]string{"tab"}, "tab", "tab"},
	"switchback": {[]string{"shift+tab"}, "shift+tab", "prev tab"},
	"browser":    {[]string{"o"}, "o", "browser"},
	"search":     {[]string{"/"}, "/", "search"},
	"filter":     {[]string{"f"}, "f", "filter"},
	"scope":      {[]string{"p"}, "p", "scope"},
	"star":       {[]string{"s"}, "s", "favorite"},
	"sort":       {[]string{"t"}, "t", "sort"},
	"refresh":    {[]string{"r"}, "r", "refresh"},
	"branch":     {[]string{"b"}, "b", "branch"},
	"parent":     {[]string{"h"}, "h", "parent dir"},
	"comment":    {[]string{"c"}, "c", "comment"},
	"approve":    {[]string{"a"}, "a", "approve"},
	"merge":      {[]string{"m"}, "m", "merge"},
	"diff":       {[]string{"tab"}, "tab", "diff/conversation"},
	"send":       {[]string{"ctrl+s", "alt+enter"}, "ctrl+s", "send"},
	"cancel":     {[]string{"esc"}, "esc", "cancel"},
	"yes":        {[]string{"y"}, "y", "yes"},
	"no":         {[]string{"n", "esc"}, "n", "no"},
	"copy":       {[]string{"c"}, "c", "copy code"},
	"retry":      {[]string{"r"}, "r", "retry"},
}

// contexts lists the actions that are live together on one screen/mode. Two
// actions of a context must not share a key, or one silently shadows the other.
var contexts = map[string][]string{
	"repos":         {"quit", "help", "back", "logout", "open", "browser", "search", "filter", "switch", "switchback", "star", "sort", "refresh"},
	"code":          {"quit", "help", "back", "logout", "open", "parent", "branch", "browser", "switch", "switchback", "refresh"},
	"pull requests": {"quit", "help", "back", "logout", "open", "filter", "scope", "switch", "switchback", "browser", "search", "refresh"},
	"issues":        {"quit", "help", "back", "logout", "open", "filter", "switch", "switchback", "browser", "search", "refresh"},
	"detail":        {"quit", "help", "back", "logout", "comment", "approve", "merge", "diff", "browser"},
	"login":         {"quit", "help", "back", "copy", "retry"},
	"editing":       {"send", "cancel"},
	"confirmation":  {"yes", "no"},
}

var overrides map[string][]string

// keysFor returns the effective keys of an action.
func keysFor(action string) []string {
	if o, ok := overrides[action]; ok {
		return o
	}
	return defs[action].keys
}

// bind builds the binding for an action, honoring user overrides.
func bind(action string) key.Binding {
	d, ok := defs[action]
	if !ok {
		panic("tui: unknown action " + action) // programming error
	}
	ks, label := keysFor(action), d.label
	if _, ok := overrides[action]; ok {
		label = ks[0]
	}
	return key.NewBinding(key.WithKeys(ks...), key.WithHelp(label, i18n.T(d.desc)))
}

// ActionNames lists the rebindable actions, sorted.
func ActionNames() []string {
	names := make([]string, 0, len(defs))
	for n := range defs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// SetKeyOverrides applies user key bindings (action -> keys). It validates
// everything first and leaves the current bindings untouched on error.
// ctrl+c always quits regardless of bindings.
func SetKeyOverrides(o map[string][]string) error {
	var problems []string
	for action, ks := range o {
		if _, ok := defs[action]; !ok {
			problems = append(problems, i18n.T("unknown action %q", action))
			continue
		}
		if len(ks) == 0 {
			problems = append(problems, i18n.T("%q: empty key list", action))
		}
		for _, k := range ks {
			if strings.TrimSpace(k) == "" {
				problems = append(problems, i18n.T("%q: empty key", action))
			}
		}
	}
	if len(problems) == 0 {
		old := overrides
		overrides = o
		problems = conflicts()
		overrides = old
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf(i18n.T("invalid shortcuts: %s"), strings.Join(problems, "; "))
	}
	overrides = o
	rebuildKeys()
	return nil
}

func conflicts() []string {
	var out []string
	names := make([]string, 0, len(contexts))
	for n := range contexts {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, ctx := range names {
		owner := map[string]string{}
		for _, action := range contexts[ctx] {
			for _, k := range keysFor(action) {
				if prev, dup := owner[k]; dup && prev != action {
					out = append(out, i18n.T("key %q shared by %q and %q (%s)", k, prev, action, i18n.T(ctx)))
				}
				owner[k] = action
			}
		}
	}
	return out
}

type globalKeys struct {
	Help, Back, Quit, Logout key.Binding
}

// Bindings shared by several screens; rebuilt by rebuildKeys.
var (
	keys          globalKeys
	keyOpenItem   key.Binding
	keySwitch     key.Binding
	keyBrowser    key.Binding
	keySearch     key.Binding
	keyRefresh    key.Binding
	keySwitchBack key.Binding
)

func init() { rebuildKeys() }

func rebuildKeys() {
	keys = globalKeys{Help: bind("help"), Back: bind("back"), Quit: bind("quit"), Logout: bind("logout")}
	keyOpenItem, keySwitch = bind("open"), bind("switch")
	keyBrowser, keySearch = bind("browser"), bind("search")
	keyRefresh = bind("refresh")
	keySwitchBack = bind("switchback")
}

// helpKeys merges the active screen's bindings with the global ones.
type helpKeys struct {
	screen    []key.Binding
	canBack   bool
	canLogout bool
}

func (h helpKeys) ShortHelp() []key.Binding {
	out := append([]key.Binding{}, h.screen...)
	if h.canBack {
		out = append(out, keys.Back)
	}
	if h.canLogout {
		out = append(out, keys.Logout)
	}
	return append(out, keys.Help, keys.Quit)
}

func (h helpKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{h.ShortHelp()}
}
