// Package config loads the user configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/kevin/gitui/internal/i18n"
)

const (
	// EnvClientID overrides client_id from the config file.
	EnvClientID = "GITUI_CLIENT_ID"
	// EnvToken and EnvGitHubToken provide a personal access token. They
	// override the config file; EnvToken wins over EnvGitHubToken.
	EnvToken       = "GITUI_TOKEN"
	EnvGitHubToken = "GITHUB_TOKEN"
	// FileTokenSource labels a token that comes from the config file.
	FileTokenSource = "config.yaml"
	dirName         = "gitui"
)

type Config struct {
	// ClientID is the client ID of a GitHub OAuth App with the device flow
	// enabled. It is public (not a secret).
	ClientID string `yaml:"client_id"`
	// Scopes requested during login.
	Scopes []string `yaml:"scopes"`
	// Theme is "auto" (default, follows the terminal), "light" or "dark".
	Theme string `yaml:"theme"`
	// Lang is the UI language: "auto" (default, from the system), "en" or "fr".
	Lang string `yaml:"lang"`
	// Token is a personal access token. When set (here or via the
	// environment) it replaces the OAuth login entirely.
	Token string `yaml:"token"`
	// TokenSource says where Token came from (EnvToken, EnvGitHubToken,
	// FileTokenSource, or "" if none); it is never read from the file.
	TokenSource string `yaml:"-"`
	// Keys maps an action name to its keys, e.g. merge = ["M"].
	// Names are validated by the TUI, which owns the action list.
	Keys map[string][]string `yaml:"keys"`
}

func defaults() Config {
	return Config{Scopes: []string{"repo", "read:org"}, Theme: "auto", Lang: "auto"}
}

// Dir returns the directory holding gitui's config and fallback token file.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, dirName), nil
}

// Path returns the default config file path ("" if it cannot be determined).
func Path() string {
	d, err := Dir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "config.yaml")
}

// template is written on first launch. Every active line equals the built-in
// default, so creating the file never changes behavior.
const templateEN = `# gitui configuration.

# Client ID of a GitHub OAuth App with "Enable Device Flow" turned on
# (Settings > Developer settings > OAuth Apps). It is public.
# Can also be given through the GITUI_CLIENT_ID environment variable.
# client_id: Iv1.xxxxxxxxxxxx

# GitHub Personal Access Token: replaces the OAuth login (no client_id needed).
# WARNING: stored in clear text on disk. Prefer the GITUI_TOKEN environment
# variable (it takes precedence over this value). Keep this file at mode 0600.
# token: ghp_xxxxxxxxxxxxxxxxxxxx

# Permissions requested at login.
scopes: [repo, read:org]

# auto (follows the terminal background) | light | dark
theme: auto

# Interface language: auto (from the system) | en | fr
# The --lang flag takes precedence over this value.
lang: auto

# Custom shortcuts: action -> list of keys.
# Actions: see the README. ctrl+c always quits.
# keys:
#   merge: [M]
#   quit: [q, x]
`

const templateFR = `# Configuration de gitui.

# Client ID d'une OAuth App GitHub avec "Enable Device Flow" activé
# (Settings > Developer settings > OAuth Apps). Il est public.
# Peut aussi être donné via la variable d'environnement GITUI_CLIENT_ID.
# client_id: Iv1.xxxxxxxxxxxx

# Personal Access Token GitHub : remplace la connexion OAuth (client_id inutile).
# ATTENTION : stocké en clair sur disque. Préférez la variable d'environnement
# GITUI_TOKEN (qui l'emporte sur cette valeur). Gardez ce fichier en mode 0600.
# token: ghp_xxxxxxxxxxxxxxxxxxxx

# Permissions demandées à la connexion.
scopes: [repo, read:org]

# auto (suit le fond du terminal) | light | dark
theme: auto

# Langue de l'interface : auto (selon le système) | en | fr
# L'option --lang l'emporte sur cette valeur.
lang: auto

# Raccourcis personnalisés : action -> liste de touches.
# Actions : voir le README. ctrl+c quitte toujours.
# keys:
#   merge: [M]
#   quit: [q, x]
`

func template() string {
	if i18n.Current() == i18n.FR {
		return templateFR
	}
	return templateEN
}

// EnsureFile creates a commented default config at path if none exists. It
// never overwrites an existing file; created reports whether it wrote one.
func EnsureFile(path string) (created bool, err error) {
	if path == "" {
		return false, errors.New(i18n.T("configuration path not found"))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}
	// O_EXCL: atomic "create only if absent", safe against concurrent launches.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(template()); err != nil {
		f.Close()
		return false, err
	}
	return true, f.Close()
}

// decodeFile fills c from a YAML file. A missing or empty file is fine;
// unknown fields are rejected so a typo like "clientid" is not silently ignored.
func decodeFile(path string, c *Config) error {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) { // io.EOF: empty file
		return err
	}
	return nil
}

// Load reads the config at path (a missing file is not an error), then applies
// environment overrides.
func Load(path string) (Config, error) {
	c := defaults()
	if path != "" {
		if err := decodeFile(path, &c); err != nil {
			return c, fmt.Errorf(i18n.T("reading %s: %w"), path, err)
		}
	}
	if v := os.Getenv(EnvClientID); v != "" {
		c.ClientID = v
	}
	// Whitespace is trimmed: env values often carry a trailing newline.
	c.Token = strings.TrimSpace(c.Token)
	c.TokenSource = ""
	if c.Token != "" {
		c.TokenSource = FileTokenSource
	}
	for _, name := range []string{EnvToken, EnvGitHubToken} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			c.Token, c.TokenSource = v, name
			break
		}
	}
	if len(c.Scopes) == 0 {
		c.Scopes = defaults().Scopes
	}
	if !i18n.ValidConfig(c.Lang) {
		return c, fmt.Errorf(i18n.T("invalid lang %q in %s (expected: auto, en or fr)"), c.Lang, path)
	}
	switch c.Theme {
	case "auto", "light", "dark":
	default:
		return c, fmt.Errorf(i18n.T("invalid theme %q in %s (expected: auto, light or dark)"), c.Theme, path)
	}
	return c, nil
}

// InsecurePermissions reports whether the file is readable or writable by
// group/others, which matters when it holds a token.
func InsecurePermissions(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().Perm()&0o077 != 0
}
