package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/kevin/gitui/internal/auth"
	"github.com/kevin/gitui/internal/config"
)

const sekret = "gho_NEVER_PRINT_ME"

func hermetic(t *testing.T) (auth.Store, string) {
	t.Helper()
	keyring.MockInit()
	t.Setenv(config.EnvClientID, "")
	t.Setenv(config.EnvToken, "")
	t.Setenv(config.EnvGitHubToken, "")
	dir := t.TempDir()
	return auth.Store{FilePath: filepath.Join(dir, "token")}, filepath.Join(dir, "config.yaml")
}

func TestLogoutRemovesStoredSession(t *testing.T) {
	store, cfg := hermetic(t)
	if err := store.Set(sekret); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := logout(&out, store, cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "supprimés") {
		t.Fatalf("output: %q", out.String())
	}
	if tok, _ := store.Get(); tok != "" {
		t.Fatalf("token survived: %q", tok)
	}
	if strings.Contains(out.String(), sekret) {
		t.Fatal("the token must never be printed")
	}
}

func TestLogoutRemovesFallbackFileToo(t *testing.T) {
	keyring.MockInitWithError(os.ErrPermission) // headless: no keyring
	t.Setenv(config.EnvToken, "")
	t.Setenv(config.EnvGitHubToken, "")
	dir := t.TempDir()
	store := auth.Store{FilePath: filepath.Join(dir, "token")}
	if err := store.Set(sekret); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.FilePath); err != nil {
		t.Fatal("setup: fallback file should exist")
	}
	var out bytes.Buffer
	if err := logout(&out, store, filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.FilePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback file must be removed, stat err = %v", err)
	}
}

func TestLogoutWithNothingStoredIsNotAnError(t *testing.T) {
	store, cfg := hermetic(t)
	var out bytes.Buffer
	if err := logout(&out, store, cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "rien à supprimer") {
		t.Fatalf("output: %q", out.String())
	}
}

func TestLogoutDoesNotCreateConfig(t *testing.T) {
	store, cfg := hermetic(t)
	var out bytes.Buffer
	if err := logout(&out, store, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("--logout must not create config.yaml")
	}
}

func TestLogoutWarnsAboutStaticToken(t *testing.T) {
	store, cfg := hermetic(t)

	t.Setenv(config.EnvToken, "ghp_env")
	var out bytes.Buffer
	if err := logout(&out, store, cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "GITUI_TOKEN") || strings.Contains(out.String(), "ghp_env") {
		t.Fatalf("expected a note naming the source only: %q", out.String())
	}

	t.Setenv(config.EnvToken, "")
	if err := os.WriteFile(cfg, []byte("token: ghp_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := logout(&out, store, cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "config.yaml") || strings.Contains(out.String(), "ghp_file") {
		t.Fatalf("config token note wrong: %q", out.String())
	}
}

func TestLogoutWorksWithBrokenConfig(t *testing.T) {
	store, cfg := hermetic(t)
	_ = store.Set(sekret)
	if err := os.WriteFile(cfg, []byte("theme: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvGitHubToken, "ghp_x")
	var out bytes.Buffer
	if err := logout(&out, store, cfg); err != nil {
		t.Fatalf("a broken config must not block the reset: %v", err)
	}
	if tok, _ := store.Get(); tok != "" {
		t.Fatal("token survived")
	}
	if !strings.Contains(out.String(), "GITHUB_TOKEN") {
		t.Fatalf("env note missing when config is unreadable: %q", out.String())
	}
}

type failingStore struct{}

func (failingStore) Get() (string, error) { return sekret, nil }
func (failingStore) Set(string) error     { return nil }
func (failingStore) Delete() error        { return errors.New("trousseau verrouillé") }

func TestLogoutReportsDeleteFailure(t *testing.T) {
	var out bytes.Buffer
	err := logout(&out, failingStore{}, "")
	if err == nil || !strings.Contains(err.Error(), "trousseau verrouillé") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(out.String(), "supprimés") {
		t.Fatal("must not claim success when deletion failed")
	}
	if strings.Contains(err.Error()+out.String(), sekret) {
		t.Fatal("token leaked")
	}
}
