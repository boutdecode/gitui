// Package config loads the user configuration.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	// EnvClientID overrides client_id from the config file.
	EnvClientID = "GITUI_CLIENT_ID"
	dirName     = "gitui"
)

type Config struct {
	// ClientID is the client ID of a GitHub OAuth App with the device flow
	// enabled. It is public (not a secret).
	ClientID string `toml:"client_id"`
	// Scopes requested during login.
	Scopes []string `toml:"scopes"`
	// Theme is "auto" (default, follows the terminal), "light" or "dark".
	Theme string `toml:"theme"`
	// Keys maps an action name to its keys, e.g. merge = ["M"].
	// Names are validated by the TUI, which owns the action list.
	Keys map[string][]string `toml:"keys"`
}

func defaults() Config {
	return Config{Scopes: []string{"repo", "read:org"}, Theme: "auto"}
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
	return filepath.Join(d, "config.toml")
}

// Load reads the config at path (a missing file is not an error), then applies
// environment overrides.
func Load(path string) (Config, error) {
	c := defaults()
	if path != "" {
		if _, err := toml.DecodeFile(path, &c); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return c, fmt.Errorf("lecture de %s : %w", path, err)
		}
	}
	if v := os.Getenv(EnvClientID); v != "" {
		c.ClientID = v
	}
	if len(c.Scopes) == 0 {
		c.Scopes = defaults().Scopes
	}
	switch c.Theme {
	case "auto", "light", "dark":
	default:
		return c, fmt.Errorf("theme %q invalide dans %s (attendu : auto, light ou dark)", c.Theme, path)
	}
	return c, nil
}
