package i18n

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for in, want := range map[string]Lang{"fr": FR, "fr_FR.UTF-8": FR, "FR-ca": FR, "en_US.UTF-8": EN, "en": EN, "fr:en": FR} {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "C", "de_DE", "french"} {
		if _, ok := Parse(in); ok {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestDetect(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cases := []struct {
		flag, cfg string
		env       map[string]string
		want      Lang
	}{
		{"", "", nil, EN},
		{"", "", map[string]string{"LANG": "fr_FR.UTF-8"}, FR},
		{"", "", map[string]string{"LANG": "C"}, EN},
		{"", "", map[string]string{"LC_ALL": "fr_FR.UTF-8", "LANG": "en_US.UTF-8"}, FR},
		{"", "", map[string]string{"LC_ALL": "de_DE", "LANG": "fr_FR"}, EN},
		{"en", "fr", map[string]string{"LANG": "fr_FR"}, EN},
		{"", "fr", map[string]string{"LANG": "en_US"}, FR},
		{"auto", "auto", map[string]string{"LANGUAGE": "fr:en"}, FR},
	}
	for i, c := range cases {
		got, err := detect(c.flag, c.cfg, env(c.env))
		if err != nil || got != c.want {
			t.Errorf("case %d: got %q,%v want %q", i, got, err, c.want)
		}
	}
	if _, err := detect("xx", "", env(nil)); err == nil {
		t.Error("invalid flag should fail")
	}
}

var verbs = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)

func TestCatalogVerbsMatch(t *testing.T) {
	for en, fr := range fr {
		a, b := verbs.FindAllString(en, -1), verbs.FindAllString(fr, -1)
		if len(a) != len(b) {
			t.Errorf("format verbs differ for %q: %v vs %v", en, a, b)
			continue
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("format verbs differ for %q: %v vs %v", en, a, b)
			}
		}
	}
}

func TestT(t *testing.T) {
	defer SetLang(EN)
	SetLang(EN)
	if T("hello %d", 3) != "hello 3" {
		t.Error("en format")
	}
	SetLang(FR)
	if T("missing message") != "missing message" {
		t.Error("fallback")
	}
}

// TestAllLiteralsTranslated makes sure every message passed to T / Err in the
// source has a French entry, so a new string cannot silently stay English.
func TestAllLiteralsTranslated(t *testing.T) {
	re := regexp.MustCompile(`i18n\.(?:T|Err)\(("(?:[^"\\]|\\.)*")`)
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			key, err := strconv.Unquote(m[1])
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			if _, ok := fr[key]; !ok {
				t.Errorf("%s: no French translation for %q", path, key)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTablesTranslated covers the labels that reach T through variables.
func TestTablesTranslated(t *testing.T) {
	for _, k := range []string{"My repos", "Favorites", "Organizations", "Recent", "all", "public", "private", "forks", "archived", "activity", "name", "stars",
		"state|open", "state|closed", "state|merged", "state|any", "to review", "assigned", "mine",
		"repos", "code", "pull requests", "issues", "detail", "login", "editing", "confirmation"} {
		if _, ok := fr[k]; !ok {
			t.Errorf("no French translation for %q", k)
		}
	}
}
