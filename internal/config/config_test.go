package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv(EnvClientID, "")
	dir := t.TempDir()

	c, err := Load(filepath.Join(dir, "missing.yaml"))
	if err != nil || c.ClientID != "" || len(c.Scopes) == 0 {
		t.Fatalf("missing file: %+v %v", c, err)
	}

	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("client_id: file\n"), 0o600); err != nil {
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

	if err := os.WriteFile(p, []byte("client_id: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("invalid yaml should error")
	}
}

func TestLoadThemeAndKeys(t *testing.T) {
	t.Setenv(EnvClientID, "")
	p := filepath.Join(t.TempDir(), "config.yaml")

	c, _ := Load(p)
	if c.Theme != "auto" {
		t.Fatalf("default theme = %q", c.Theme)
	}

	body := "theme: light\nkeys:\n  merge: [M]\n  quit: [q, x]\n"
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

	if err := os.WriteFile(p, []byte("theme: solarized\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("invalid theme must be rejected")
	}
}

func TestLoadYAMLEdgeCases(t *testing.T) {
	t.Setenv(EnvClientID, "")
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "c.yaml")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// an empty file (or only comments) is a valid, default config
	for _, body := range []string{"", "# rien ici\n"} {
		c, err := Load(write(body))
		if err != nil || c.Theme != "auto" || len(c.Scopes) == 0 {
			t.Fatalf("%q: %+v %v", body, c, err)
		}
	}

	// unknown fields are errors: a typo must not be silently ignored
	if _, err := Load(write("clientid: abc\n")); err == nil {
		t.Fatal("unknown field should be rejected")
	}
	// wrong type
	if _, err := Load(write("scopes: repo\n")); err == nil {
		t.Fatal("scopes must be a list")
	}

	// block-style lists work too
	c, err := Load(write("scopes:\n  - repo\n  - gist\nkeys:\n  quit:\n    - x\n"))
	if err != nil || len(c.Scopes) != 2 || c.Keys["quit"][0] != "x" {
		t.Fatalf("%+v %v", c, err)
	}
}
