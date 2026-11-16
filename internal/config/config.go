// Package config loads the user configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	// EnvClientID overrides client_id from the config file.
	EnvClientID = "GITUI_CLIENT_ID"
	dirName     = "gitui"
)

type Config struct {
	// ClientID is the client ID of a GitHub OAuth App with the device flow
	// enabled. It is public (not a secret).
	ClientID string `yaml:"client_id"`
	// Scopes requested during login.
	Scopes []string `yaml:"scopes"`
	// Theme is "auto" (default, follows the terminal), "light" or "dark".
	Theme string `yaml:"theme"`
	// Keys maps an action name to its keys, e.g. merge = ["M"].
	// Names are validated by the TUI, which owns the action list.
	Keys map[string][]string `yaml:"keys"`
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
	return filepath.Join(d, "config.yaml")
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
