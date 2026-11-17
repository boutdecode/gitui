package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
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
	"quit":    {[]string{"q", "ctrl+c"}, "q", "quitter"},
	"help":    {[]string{"?"}, "?", "aide"},
	"back":    {[]string{"esc", "backspace"}, "esc", "retour"},
	"logout":  {[]string{"L"}, "L", "déconnexion"},
	"open":    {[]string{"enter"}, "enter", "ouvrir"},
	"switch":  {[]string{"tab"}, "tab", "onglet"},
	"browser": {[]string{"o"}, "o", "navigateur"},
	"search":  {[]string{"/"}, "/", "chercher"},
	"filter":  {[]string{"f"}, "f", "filtre"},
	"scope":   {[]string{"p"}, "p", "portée"},
	"star":    {[]string{"s"}, "s", "favori"},
	"sort":    {[]string{"t"}, "t", "tri"},
	"refresh": {[]string{"r"}, "r", "rafraîchir"},
	"branch":  {[]string{"b"}, "b", "branche"},
	"parent":  {[]string{"h"}, "h", "dossier parent"},
	"comment": {[]string{"c"}, "c", "commenter"},
	"approve": {[]string{"a"}, "a", "approuver"},
	"merge":   {[]string{"m"}, "m", "merger"},
	"diff":    {[]string{"tab"}, "tab", "diff/conversation"},
	"send":    {[]string{"ctrl+s", "alt+enter"}, "ctrl+s", "envoyer"},
	"cancel":  {[]string{"esc"}, "esc", "annuler"},
	"yes":     {[]string{"y"}, "y", "oui"},
	"no":      {[]string{"n", "esc"}, "n", "non"},
	"copy":    {[]string{"c"}, "c", "copier le code"},
	"retry":   {[]string{"r"}, "r", "réessayer"},
}

// contexts lists the actions that are live together on one screen/mode. Two
// actions of a context must not share a key, or one silently shadows the other.
var contexts = map[string][]string{
	"dépôts":        {"quit", "help", "back", "logout", "open", "browser", "search", "filter", "switch", "star", "sort", "refresh"},
	"code":          {"quit", "help", "back", "logout", "open", "parent", "branch", "browser", "switch", "refresh"},
	"pull requests": {"quit", "help", "back", "logout", "open", "filter", "scope", "switch", "browser", "search", "refresh"},
	"issues":        {"quit", "help", "back", "logout", "open", "filter", "switch", "browser", "search", "refresh"},
	"détail":        {"quit", "help", "back", "logout", "comment", "approve", "merge", "diff", "browser"},
	"connexion":     {"quit", "help", "back", "copy", "retry"},
	"édition":       {"send", "cancel"},
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
	return key.NewBinding(key.WithKeys(ks...), key.WithHelp(label, d.desc))
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
			problems = append(problems, fmt.Sprintf("action inconnue %q", action))
			continue
		}
		if len(ks) == 0 {
			problems = append(problems, fmt.Sprintf("%q : liste de touches vide", action))
		}
		for _, k := range ks {
			if strings.TrimSpace(k) == "" {
				problems = append(problems, fmt.Sprintf("%q : touche vide", action))
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
		return fmt.Errorf("raccourcis invalides : %s", strings.Join(problems, " ; "))
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
					out = append(out, fmt.Sprintf("touche %q partagée par %q et %q (%s)", k, prev, action, ctx))
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
	keys        globalKeys
	keyOpenItem key.Binding
	keySwitch   key.Binding
	keyBrowser  key.Binding
	keySearch   key.Binding
	keyRefresh  key.Binding
)

func init() { rebuildKeys() }

func rebuildKeys() {
	keys = globalKeys{Help: bind("help"), Back: bind("back"), Quit: bind("quit"), Logout: bind("logout")}
	keyOpenItem, keySwitch = bind("open"), bind("switch")
	keyBrowser, keySearch = bind("browser"), bind("search")
	keyRefresh = bind("refresh")
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
