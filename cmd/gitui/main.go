package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/auth"
	"github.com/kevin/gitui/internal/config"
	"github.com/kevin/gitui/internal/i18n"
	"github.com/kevin/gitui/internal/provider"
	"github.com/kevin/gitui/internal/provider/fake"
	"github.com/kevin/gitui/internal/provider/github"
	"github.com/kevin/gitui/internal/recents"
	"github.com/kevin/gitui/internal/tui"
)

func main() {
	// The language must be known before the flags are declared (their help is
	// translated), so --lang is looked up by hand first.
	langFlag := langArg(os.Args[1:])
	lang, err := i18n.Detect(langFlag, configLang())
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitui:", err)
		os.Exit(2)
	}
	i18n.SetLang(lang)

	useFake := flag.Bool("fake", false, i18n.T("use the fake provider (no login)"))
	doLogout := flag.Bool("logout", false, i18n.T("delete the stored credentials (keyring and file) and quit"))
	transparent := flag.Bool("transparent", false, i18n.T("keep the terminal background (do not paint the theme background)"))
	flag.String("lang", "auto", i18n.T("interface language: auto (from the system), en or fr"))
	flag.Parse()

	if *doLogout {
		// Before anything else: no config file is created, nothing is parsed.
		if err := logout(os.Stdout, tokenStore(), config.Path()); err != nil {
			fmt.Fprintln(os.Stderr, "gitui:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*useFake, *transparent); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// langArg returns the value of --lang / -lang (with "=" or a separate value),
// or "" when absent.
func langArg(args []string) string {
	for i, a := range args {
		if a == "--" {
			return ""
		}
		a = strings.TrimLeft(a, "-")
		if a == "lang" && strings.HasPrefix(args[i], "-") {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if v, ok := strings.CutPrefix(a, "lang="); ok && strings.HasPrefix(args[i], "-") {
			return v
		}
	}
	return ""
}

// configLang reads the language from the config file, ignoring any error: a
// broken config is reported later, by run, in the resolved language.
func configLang() string {
	if p := config.Path(); p != "" {
		cfg, _ := config.Load(p)
		return cfg.Lang
	}
	return ""
}

func run(useFake, transparent bool) error {
	cfgPath := config.Path()
	// A read-only home must not prevent using the app: warn and carry on.
	if _, err := config.EnsureFile(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "gitui: "+i18n.T("could not create %s: %v")+"\n", cfgPath, err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if cfg.TokenSource == config.FileTokenSource && config.InsecurePermissions(cfgPath) {
		fmt.Fprintf(os.Stderr, "gitui: "+i18n.T("%s contains a token but is readable by other users; run: chmod 600 %s")+"\n", cfgPath, cfgPath)
	}
	if err := tui.SetKeyOverrides(cfg.Keys); err != nil {
		return fmt.Errorf("%s: %w", cfgPath, err)
	}

	opts := tui.Options{Theme: cfg.Theme, Transparent: transparent}
	if !useFake {
		if dir, err := config.Dir(); err == nil {
			opts.Recents = recents.New(filepath.Join(dir, "recent.json"))
		}
	}
	if useFake {
		opts.Provider = fake.New()
	} else {
		opts.Auth = &tui.AuthOptions{
			Flow:        auth.Flow{ClientID: cfg.ClientID, Scopes: cfg.Scopes},
			Store:       tokenStore(),
			ConfigPath:  cfgPath,
			NewProvider: func(token string) (provider.Provider, error) { return github.New(token) },

			StaticToken:  cfg.Token,
			StaticSource: cfg.TokenSource,
		}
	}
	_, err = tea.NewProgram(tui.New(opts)).Run()
	return err
}
