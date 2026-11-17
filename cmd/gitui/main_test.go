package main

import (
	"os"
	"testing"

	"github.com/kevin/gitui/internal/i18n"
)

func TestMain(m *testing.M) {
	i18n.SetLang(i18n.FR)
	os.Exit(m.Run())
}

func TestLangArg(t *testing.T) {
	cases := map[string]string{
		"--lang fr":        "fr",
		"-lang fr":         "fr",
		"--lang=en":        "en",
		"--fake --lang=fr": "fr",
		"--fake":           "",
		"--lang":           "",
		"-- --lang fr":     "",
	}
	for in, want := range cases {
		if got := langArg(splitFields(in)); got != want {
			t.Errorf("langArg(%q) = %q, want %q", in, got, want)
		}
	}
}

func splitFields(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ' ' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}
