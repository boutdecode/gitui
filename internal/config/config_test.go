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

func TestEnsureFileCreatesDefaultConfig(t *testing.T) {
	t.Setenv(EnvClientID, "")
	p := filepath.Join(t.TempDir(), "nested", "gitui", "config.yaml")

	created, err := EnsureFile(p)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v", fi.Mode().Perm())
	}
	if di, _ := os.Stat(filepath.Dir(p)); di.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode = %v", di.Mode().Perm())
	}

	// the generated file must parse and behave exactly like the defaults
	c, err := Load(p)
	if err != nil {
		t.Fatalf("generated file does not load: %v", err)
	}
	d := defaults()
	if c.Theme != d.Theme || c.ClientID != "" || len(c.Keys) != 0 ||
		len(c.Scopes) != len(d.Scopes) || c.Scopes[0] != d.Scopes[0] || c.Scopes[1] != d.Scopes[1] {
		t.Fatalf("template differs from defaults: %+v", c)
	}
}

func TestEnsureFileNeverOverwrites(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte("theme: dark\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	created, err := EnsureFile(p)
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if b, _ := os.ReadFile(p); string(b) != "theme: dark\n" {
		t.Fatalf("existing config was modified: %q", b)
	}
}

func TestEnsureFileErrors(t *testing.T) {
	if _, err := EnsureFile(""); err == nil {
		t.Fatal("empty path must error")
	}
	// parent is a regular file: directory creation must fail cleanly
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureFile(filepath.Join(blocker, "config.yaml")); err == nil {
		t.Fatal("expected an error")
	}
}
