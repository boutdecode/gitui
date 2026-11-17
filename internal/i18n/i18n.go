// Package i18n provides the tiny translation layer of gitui.
//
// Messages are written in English in the source and used as the lookup key:
// T("Quit") returns "Quit" in English and the French catalog entry otherwise.
// A message missing from the catalog is returned unchanged, so English is
// always the fallback.
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// Lang is a supported UI language.
type Lang string

const (
	EN Lang = "en"
	FR Lang = "fr"
)

// Langs lists the supported languages.
var Langs = []Lang{EN, FR}

var current atomic.Value // Lang

func init() { current.Store(EN) }

// SetLang selects the language used by T.
func SetLang(l Lang) { current.Store(l) }

// Current returns the active language.
func Current() Lang { return current.Load().(Lang) }

// T translates msg into the active language and formats it with args (if any).
//
// A message may start with a disambiguating context such as "state|open"; the
// context is part of the catalog key and is dropped when nothing is translated.
func T(msg string, args ...any) string {
	if Current() == FR {
		if s, ok := fr[msg]; ok {
			msg = s
		}
	}
	msg = stripContext(msg)
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

func stripContext(msg string) string {
	i := strings.IndexByte(msg, '|')
	if i <= 0 {
		return msg
	}
	for _, c := range msg[:i] {
		if c < 'a' || c > 'z' {
			return msg
		}
	}
	return msg[i+1:]
}

// Parse extracts a supported language from a locale or language tag such as
// "fr", "fr_FR.UTF-8", "en-US" or "fr:en".
func Parse(s string) (Lang, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", false
	}
	if i := strings.IndexAny(s, ":"); i >= 0 { // LANGUAGE=fr:en
		s = s[:i]
	}
	for _, l := range Langs {
		code := string(l)
		if s == code || strings.HasPrefix(s, code+"_") || strings.HasPrefix(s, code+"-") || strings.HasPrefix(s, code+".") || strings.HasPrefix(s, code+"@") {
			return l, true
		}
	}
	return "", false
}

// Detect resolves the language. Priority: explicit flag, config value, then
// LC_ALL, LC_MESSAGES, LANGUAGE and LANG; English when nothing matches.
// flag and cfg may be empty or "auto". An invalid flag value is an error.
func Detect(flag, cfg string) (Lang, error) {
	return detect(flag, cfg, os.Getenv)
}

func detect(flag, cfg string, getenv func(string) string) (Lang, error) {
	if flag != "" && flag != "auto" {
		l, ok := Parse(flag)
		if !ok {
			return EN, fmt.Errorf("unsupported language %q (expected: en, fr)", flag)
		}
		return l, nil
	}
	if cfg != "" && cfg != "auto" {
		if l, ok := Parse(cfg); ok {
			return l, nil
		}
	}
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		val := getenv(v)
		if val == "" || val == "C" || val == "POSIX" {
			continue
		}
		if l, ok := Parse(val); ok {
			return l, nil
		}
		if v != "LANGUAGE" {
			break // a set but unsupported locale means English, not the next variable
		}
	}
	return EN, nil
}

// ValidConfig reports whether s is an acceptable value for the config file.
func ValidConfig(s string) bool {
	if s == "" || s == "auto" {
		return true
	}
	_, ok := Parse(s)
	return ok
}

type msgErr struct{ msg string }

func (e *msgErr) Error() string { return T(e.msg) }

// Err returns a sentinel error whose text is translated when it is displayed,
// not when it is created. Compare it with errors.Is as usual.
func Err(msg string) error { return &msgErr{msg} }
