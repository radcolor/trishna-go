package ollama

import "testing"

func TestValidateLocalhostBaseURL(t *testing.T) {
	tests := []struct {
		url string
		ok  bool
	}{
		{"http://127.0.0.1:11434", true},
		{"http://localhost:11434", true},
		{"http://[::1]:11434", true},
		{"https://127.0.0.1:11434", false},
		{"http://192.168.1.1:11434", false},
		{"http://evil.example.com:11434", false},
	}
	for _, tc := range tests {
		err := ValidateLocalhostBaseURL(tc.url)
		if tc.ok && err != nil {
			t.Fatalf("%q: %v", tc.url, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q: expected error", tc.url)
		}
	}
}
