package auth

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

// TokenStore persists the access token.
type TokenStore interface {
	// Get returns "" with a nil error when no token is stored.
	Get() (string, error)
	Set(token string) error
	Delete() error
}

const (
	keyringService = "gitui"
	keyringUser    = "github"
)

// Store keeps the token in the OS keyring and falls back to a 0600 file when
// no keyring is available (e.g. headless Linux).
type Store struct {
	// FilePath is the fallback file ("" disables the fallback).
	FilePath string
}

func (s Store) Get() (string, error) {
	tok, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		return tok, nil
	}
	if s.FilePath == "" {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	b, ferr := os.ReadFile(s.FilePath)
	if errors.Is(ferr, fs.ErrNotExist) {
		return "", nil
	}
	if ferr != nil {
		return "", ferr
	}
	return strings.TrimSpace(string(b)), nil
}

func (s Store) Set(token string) error {
	if err := keyring.Set(keyringService, keyringUser, token); err == nil {
		// don't leave a stale copy in the fallback file
		if s.FilePath != "" {
			_ = os.Remove(s.FilePath)
		}
		return nil
	} else if s.FilePath == "" {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.FilePath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.FilePath, []byte(token), 0o600)
}

func (s Store) Delete() error {
	kerr := keyring.Delete(keyringService, keyringUser)
	if s.FilePath != "" {
		if err := os.Remove(s.FilePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if kerr == nil || errors.Is(kerr, keyring.ErrNotFound) {
		return nil
	}
	// A keyring error is only a problem if the token is really still there
	// (on a headless box the keyring is simply unavailable).
	if tok, err := keyring.Get(keyringService, keyringUser); err == nil && tok != "" {
		return kerr
	}
	return nil
}
