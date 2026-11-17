// Package auth implements the GitHub OAuth device flow and token storage.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kevin/gitui/internal/i18n"
)

var (
	ErrExpired = i18n.Err("the login code has expired")
	ErrDenied  = i18n.Err("authorization denied")
)

const (
	defaultBaseURL = "https://github.com"
	grantType      = "urn:ietf:params:oauth:grant-type:device_code"
)

// DeviceCode is the server response that starts a device flow.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// PollResult is the outcome of one poll. Exactly one of Token / Pending is set.
type PollResult struct {
	Token    string
	Pending  bool
	SlowDown bool // the server asked to poll less often
}

// Flow talks to GitHub's device flow endpoints.
type Flow struct {
	ClientID string
	Scopes   []string
	BaseURL  string       // defaults to https://github.com
	HTTP     *http.Client // defaults to a client with a 15s timeout
}

func (f Flow) base() string {
	if f.BaseURL != "" {
		return strings.TrimRight(f.BaseURL, "/")
	}
	return defaultBaseURL
}

func (f Flow) client() *http.Client {
	if f.HTTP != nil {
		return f.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (f Flow) post(ctx context.Context, path string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.base()+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := f.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf(i18n.T("github replied %s"), resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Start requests a device and user code.
func (f Flow) Start(ctx context.Context) (DeviceCode, error) {
	if f.ClientID == "" {
		return DeviceCode{}, errors.New(i18n.T("client_id not configured"))
	}
	var r struct {
		DeviceCode
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}
	form := url.Values{"client_id": {f.ClientID}, "scope": {strings.Join(f.Scopes, " ")}}
	if err := f.post(ctx, "/login/device/code", form, &r); err != nil {
		return DeviceCode{}, err
	}
	if r.Error != "" {
		return DeviceCode{}, fmt.Errorf("%s: %s", r.Error, r.ErrorDesc)
	}
	if r.DeviceCode.DeviceCode == "" || r.UserCode == "" {
		return DeviceCode{}, errors.New(i18n.T("incomplete GitHub response"))
	}
	if r.Interval <= 0 {
		r.Interval = 5
	}
	return r.DeviceCode, nil
}

// Check polls once. The caller is responsible for waiting dc.Interval
// between calls (and 5s more after SlowDown), which keeps the TUI non-blocking.
func (f Flow) Check(ctx context.Context, dc DeviceCode) (PollResult, error) {
	var r struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	form := url.Values{
		"client_id":   {f.ClientID},
		"device_code": {dc.DeviceCode},
		"grant_type":  {grantType},
	}
	if err := f.post(ctx, "/login/oauth/access_token", form, &r); err != nil {
		return PollResult{}, err
	}
	switch r.Error {
	case "":
		if r.AccessToken == "" {
			return PollResult{}, errors.New(i18n.T("GitHub response without a token"))
		}
		return PollResult{Token: r.AccessToken}, nil
	case "authorization_pending":
		return PollResult{Pending: true}, nil
	case "slow_down":
		return PollResult{Pending: true, SlowDown: true}, nil
	case "expired_token":
		return PollResult{}, ErrExpired
	case "access_denied":
		return PollResult{}, ErrDenied
	default:
		return PollResult{}, fmt.Errorf("%s: %s", r.Error, r.ErrorDesc)
	}
}
