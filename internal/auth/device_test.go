package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func server(t *testing.T, accessResp string) Flow {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login/device/code", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("client_id") != "cid" || r.Form.Get("scope") != "repo read:org" {
			http.Error(w, "bad form", 400)
			return
		}
		if r.Header.Get("Accept") != "application/json" {
			http.Error(w, "bad accept", 400)
			return
		}
		_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900}`))
	})
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("device_code") != "dc" || r.Form.Get("grant_type") != grantType {
			http.Error(w, "bad form", 400)
			return
		}
		_, _ = w.Write([]byte(accessResp))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return Flow{ClientID: "cid", Scopes: []string{"repo", "read:org"}, BaseURL: srv.URL}
}

func TestStartDefaultsInterval(t *testing.T) {
	dc, err := server(t, `{}`).Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dc.UserCode != "ABCD-1234" || dc.Interval != 5 {
		t.Fatalf("unexpected %+v", dc)
	}
}

func TestStartWithoutClientID(t *testing.T) {
	if _, err := (Flow{}).Start(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name, body string
		want       PollResult
		wantErr    error
		anyErr     bool
	}{
		{"token", `{"access_token":"tok"}`, PollResult{Token: "tok"}, nil, false},
		{"pending", `{"error":"authorization_pending"}`, PollResult{Pending: true}, nil, false},
		{"slow", `{"error":"slow_down"}`, PollResult{Pending: true, SlowDown: true}, nil, false},
		{"expired", `{"error":"expired_token"}`, PollResult{}, ErrExpired, false},
		{"denied", `{"error":"access_denied"}`, PollResult{}, ErrDenied, false},
		{"other", `{"error":"incorrect_client_credentials","error_description":"nope"}`, PollResult{}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := server(t, c.body)
			got, err := f.Check(context.Background(), DeviceCode{DeviceCode: "dc"})
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if c.wantErr == nil && !c.anyErr && err != nil {
				t.Fatal(err)
			}
			if c.anyErr && err == nil {
				t.Fatal("expected error")
			}
			if got != c.want {
				t.Fatalf("got %+v want %+v", got, c.want)
			}
		})
	}
}
