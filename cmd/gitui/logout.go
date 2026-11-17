package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevin/gitui/internal/auth"
	"github.com/kevin/gitui/internal/config"
)

// tokenStore is where the OAuth session lives: the OS keyring, with a 0600
// file next to config.yaml as fallback.
func tokenStore() auth.Store {
	var file string
	if dir, err := config.Dir(); err == nil {
		file = filepath.Join(dir, "token")
	}
	return auth.Store{FilePath: file}
}

// logout deletes the stored credentials and reports what happened. It never
// prints the token, and it works even if config.yaml is broken (that is
// exactly when a user needs to reset things).
func logout(w io.Writer, store auth.TokenStore, cfgPath string) error {
	tok, getErr := store.Get()
	if err := store.Delete(); err != nil {
		return fmt.Errorf("suppression des identifiants impossible : %w", err)
	}
	switch {
	case tok != "":
		fmt.Fprintln(w, "Identifiants supprimés (trousseau et fichier). Relancez gitui pour vous reconnecter.")
	case getErr != nil:
		// Delete succeeded, but we could not tell whether something was there.
		fmt.Fprintf(w, "Aucun identifiant supprimé : lecture du trousseau impossible (%v).\n", getErr)
	default:
		fmt.Fprintln(w, "Aucun identifiant stocké : rien à supprimer.")
	}
	if src := staticTokenSource(cfgPath); src != "" {
		fmt.Fprintf(w, "Note : un token est fourni par %s ; gitui continuera de l'utiliser.\n"+
			"Retirez-le pour changer d'identité.\n", src)
	}
	return nil
}

// staticTokenSource reports where a PAT would come from ("" if none). It is
// best effort: an unreadable config must not hide an environment token.
func staticTokenSource(cfgPath string) string {
	if cfg, err := config.Load(cfgPath); err == nil {
		return cfg.TokenSource
	}
	for _, name := range []string{config.EnvToken, config.EnvGitHubToken} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return name
		}
	}
	return ""
}
