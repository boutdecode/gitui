// Package browser opens URLs in the user's default web browser.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/kevin/gitui/internal/i18n"
)

// Open launches the default browser on url. Only http(s) URLs are accepted so
// a malformed value can never be interpreted as a command-line option.
func Open(url string) error {
	if err := validate(url); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf(i18n.T("opening the browser: %w"), err)
	}
	go func() { _ = cmd.Wait() }() // reap the child
	return nil
}

func validate(url string) error {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return fmt.Errorf(i18n.T("unsupported URL: %q"), url)
	}
	return nil
}
