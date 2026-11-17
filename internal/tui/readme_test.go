package tui

import (
	"os"
	"strings"
	"testing"
)

// The READMEs document every rebindable action; keep them in sync.
func TestReadmeDocumentsEveryAction(t *testing.T) {
	for _, file := range []string{"README.md"} {
		b, err := os.ReadFile("../../" + file)
		if err != nil {
			t.Fatal(err)
		}
		readme := string(b)
		for _, name := range ActionNames() {
			if !strings.Contains(readme, "`"+name+"`") {
				t.Errorf("%s does not document action %q", file, name)
			}
		}
		// and the documented default keys match the code for the common ones
		for _, want := range []string{"`ctrl+s`", "`alt+enter`", "`backspace`", "`--lang"} {
			if !strings.Contains(readme, want) {
				t.Errorf("%s is missing %s", file, want)
			}
		}
	}
}
