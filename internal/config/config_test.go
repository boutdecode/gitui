package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv(EnvClientID, "")
	dir := t.TempDir()

	c, err := Load(filepath.Join(dir, "missing.toml"))
	if err != nil || c.ClientID != "" || len(c.Scopes) == 0 {
		t.Fatalf("missing file: %+v %v", c, err)
	}

	p := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(p, []byte("client_id = \"file\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, _ = Load(p)
	if c.ClientID != "file" || len(c.Scopes) == 0 {
		t.Fatalf("file: %+v", c)
	}

	t.Setenv(EnvClientID, "env")
	c, _ = Load(p)
	if c.ClientID != "env" {
		t.Fatalf("env should win: %+v", c)
	}

	if err := os.WriteFile(p, []byte("client_id = "), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("invalid toml should error")
	}
}

func TestLoadThemeAndKeys(t *testing.T) {
	t.Setenv(EnvClientID, "")
	p := filepath.Join(t.TempDir(), "config.toml")

	c, _ := Load(p)
	if c.Theme != "auto" {
		t.Fatalf("default theme = %q", c.Theme)
	}

	body := "theme = \"light\"\n\n[keys]\nmerge = [\"M\"]\nquit = [\"q\", \"x\"]\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Theme != "light" || len(c.Keys["quit"]) != 2 || c.Keys["merge"][0] != "M" {
		t.Fatalf("parsed %+v", c)
	}

	if err := os.WriteFile(p, []byte("theme = \"solarized\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("invalid theme must be rejected")
	}
}
