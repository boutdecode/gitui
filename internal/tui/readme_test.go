package tui

import (
	"os"
	"strings"
	"testing"
)

// The README documents every rebindable action; keep them in sync.
func TestReadmeDocumentsEveryAction(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	for _, name := range ActionNames() {
		if !strings.Contains(readme, "`"+name+"`") {
			t.Errorf("README.md does not document action %q", name)
		}
	}
	// and the documented default keys match the code for the common ones
	for _, want := range []string{"`ctrl+s`", "`alt+enter`", "`backspace`"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md is missing default key %s", want)
		}
	}
}
