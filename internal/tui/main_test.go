package tui

import (
	"os"
	"testing"

	"github.com/kevin/gitui/internal/i18n"
)

// Most assertions in this package are written against the French UI; the
// English UI is covered by i18n_test.go.
func TestMain(m *testing.M) {
	i18n.SetLang(i18n.FR)
	rebuildKeys()
	os.Exit(m.Run())
}
