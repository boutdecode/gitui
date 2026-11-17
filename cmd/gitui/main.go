package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/kevin/gitui/internal/auth"
	"github.com/kevin/gitui/internal/config"
	"github.com/kevin/gitui/internal/provider"
	"github.com/kevin/gitui/internal/provider/fake"
	"github.com/kevin/gitui/internal/provider/github"
	"github.com/kevin/gitui/internal/recents"
	"github.com/kevin/gitui/internal/tui"
)

func main() {
	useFake := flag.Bool("fake", false, "utiliser le provider factice (sans connexion)")
	doLogout := flag.Bool("logout", false, "supprimer les identifiants stockés (trousseau et fichier) et quitter")
	transparent := flag.Bool("transparent", false, "garder le fond du terminal (ne pas peindre le fond du thème)")
	flag.Parse()

	if *doLogout {
		// Before anything else: no config file is created, nothing is parsed.
		if err := logout(os.Stdout, tokenStore(), config.Path()); err != nil {
			fmt.Fprintln(os.Stderr, "gitui :", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*useFake, *transparent); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(useFake, transparent bool) error {
	cfgPath := config.Path()
	// A read-only home must not prevent using the app: warn and carry on.
	if _, err := config.EnsureFile(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "gitui : impossible de créer %s : %v\n", cfgPath, err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if cfg.TokenSource == config.FileTokenSource && config.InsecurePermissions(cfgPath) {
		fmt.Fprintf(os.Stderr, "gitui : %s contient un token mais est lisible par d'autres utilisateurs ; faites : chmod 600 %s\n", cfgPath, cfgPath)
	}
	if err := tui.SetKeyOverrides(cfg.Keys); err != nil {
		return fmt.Errorf("%s : %w", cfgPath, err)
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
