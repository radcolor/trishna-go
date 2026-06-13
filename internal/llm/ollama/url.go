package ollama

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func ValidateLocalhostBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("ollama base URL is empty")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse ollama base URL: %w", err)
	}
	if parsed.Scheme != "http" {
		return fmt.Errorf("ollama base URL must use http (got %q)", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("ollama base URL missing host")
	}
	if !isLocalhostHost(host) {
		return fmt.Errorf("ollama base URL host must be localhost (got %q)", host)
	}
	return nil
}

func isLocalhostHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
