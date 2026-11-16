package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestStoreKeyring(t *testing.T) {
	keyring.MockInit()
	s := Store{FilePath: filepath.Join(t.TempDir(), "token")}
	if tok, err := s.Get(); tok != "" || err != nil {
		t.Fatalf("empty store: %q %v", tok, err)
	}
	if err := s.Set("abc"); err != nil {
		t.Fatal(err)
	}
	if tok, _ := s.Get(); tok != "abc" {
		t.Fatalf("got %q", tok)
	}
	if _, err := os.Stat(s.FilePath); err == nil {
		t.Fatal("token must not be written to file when keyring works")
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if tok, _ := s.Get(); tok != "" {
		t.Fatalf("token survived delete: %q", tok)
	}
}

func TestStoreFileFallback(t *testing.T) {
	keyring.MockInitWithError(os.ErrPermission)
	s := Store{FilePath: filepath.Join(t.TempDir(), "sub", "token")}
	if err := s.Set("abc"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(s.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", fi.Mode().Perm())
	}
	if tok, err := s.Get(); tok != "abc" || err != nil {
		t.Fatalf("got %q %v", tok, err)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if tok, _ := s.Get(); tok != "" {
		t.Fatalf("token survived delete: %q", tok)
	}
}
