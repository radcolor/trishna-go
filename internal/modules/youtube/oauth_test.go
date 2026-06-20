package youtube

import (
	"net/url"
	"strings"
	"testing"
)

func TestAuthURLUsesPKCEAndOfflineAccess(t *testing.T) {
	rawURL, verifier, err := AuthURL("client", "http://127.0.0.1/callback", "verifier-value", nil)
	if err != nil {
		t.Fatalf("auth url: %v", err)
	}
	if verifier != "verifier-value" {
		t.Fatalf("verifier = %q", verifier)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	values := parsed.Query()
	if values.Get("access_type") != "offline" {
		t.Fatalf("access_type = %q", values.Get("access_type"))
	}
	if values.Get("prompt") != "consent select_account" {
		t.Fatalf("prompt = %q", values.Get("prompt"))
	}
	if values.Get("code_challenge_method") != "S256" {
		t.Fatalf("code_challenge_method = %q", values.Get("code_challenge_method"))
	}
	if !strings.Contains(values.Get("scope"), ScopeYouTubeForceSSL) {
		t.Fatalf("scope = %q", values.Get("scope"))
	}
}
