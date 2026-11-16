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
	"github.com/kevin/gitui/internal/tui"
)

func main() {
	useFake := flag.Bool("fake", false, "utiliser le provider factice (sans connexion)")
	flag.Parse()

	if err := run(*useFake); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(useFake bool) error {
	cfgPath := config.Path()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if err := tui.SetKeyOverrides(cfg.Keys); err != nil {
		return fmt.Errorf("%s : %w", cfgPath, err)
	}

	opts := tui.Options{Theme: cfg.Theme}
	if useFake {
		opts.Provider = fake.New()
	} else {
		var tokenFile string
		if dir, err := config.Dir(); err == nil {
			tokenFile = filepath.Join(dir, "token")
		}
		opts.Auth = &tui.AuthOptions{
			Flow:        auth.Flow{ClientID: cfg.ClientID, Scopes: cfg.Scopes},
			Store:       auth.Store{FilePath: tokenFile},
			ConfigPath:  cfgPath,
			NewProvider: func(token string) (provider.Provider, error) { return github.New(token) },
		}
	}
	_, err = tea.NewProgram(tui.New(opts)).Run()
	return err
}
