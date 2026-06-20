package youtube

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ScopeYouTubeForceSSL = "https://www.googleapis.com/auth/youtube.force-ssl"
	defaultTokenPath     = "data/youtube-token.json"
	googleAuthURL        = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL       = "https://oauth2.googleapis.com/token"
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	TokenPath    string
	Scopes       []string
	HTTPClient   *http.Client
}

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

type TokenSource struct {
	cfg        OAuthConfig
	httpClient *http.Client
	mu         chan struct{}
}

func NewTokenSource(cfg OAuthConfig) *TokenSource {
	if cfg.TokenPath == "" {
		cfg.TokenPath = defaultTokenPath
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{ScopeYouTubeForceSSL}
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &TokenSource{
		cfg:        cfg,
		httpClient: client,
		mu:         make(chan struct{}, 1),
	}
}

func (s *TokenSource) Token(ctx context.Context) (Token, error) {
	s.mu <- struct{}{}
	defer func() { <-s.mu }()

	token, err := LoadToken(s.cfg.TokenPath)
	if err != nil {
		return Token{}, err
	}
	if token.AccessToken != "" && time.Until(token.Expiry) > time.Minute {
		return token, nil
	}
	if token.RefreshToken == "" {
		return Token{}, errors.New("youtube token expired and has no refresh_token")
	}

	values := url.Values{}
	values.Set("client_id", s.cfg.ClientID)
	if s.cfg.ClientSecret != "" {
		values.Set("client_secret", s.cfg.ClientSecret)
	}
	values.Set("refresh_token", token.RefreshToken)
	values.Set("grant_type", "refresh_token")

	refreshed, err := s.exchange(ctx, values)
	if err != nil {
		return Token{}, err
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
	}
	if err := SaveToken(s.cfg.TokenPath, refreshed); err != nil {
		return Token{}, err
	}
	return refreshed, nil
}

func (s *TokenSource) ExchangeCode(ctx context.Context, code, redirectURI, verifier string) (Token, error) {
	values := url.Values{}
	values.Set("client_id", s.cfg.ClientID)
	if s.cfg.ClientSecret != "" {
		values.Set("client_secret", s.cfg.ClientSecret)
	}
	values.Set("code", code)
	values.Set("code_verifier", verifier)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", redirectURI)

	token, err := s.exchange(ctx, values)
	if err != nil {
		return Token{}, err
	}
	if err := SaveToken(s.cfg.TokenPath, token); err != nil {
		return Token{}, err
	}
	return token, nil
}

func (s *TokenSource) exchange(ctx context.Context, values url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Token{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Token{}, fmt.Errorf("exchange youtube token: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Token{}, err
	}
	if raw.AccessToken == "" {
		return Token{}, errors.New("youtube token response missing access_token")
	}

	expiry := time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	if raw.ExpiresIn == 0 {
		expiry = time.Now().Add(time.Hour)
	}
	return Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		TokenType:    raw.TokenType,
		Expiry:       expiry,
	}, nil
}

func AuthURL(clientID, redirectURI, verifier string, scopes []string) (string, string, error) {
	if strings.TrimSpace(clientID) == "" {
		return "", "", errors.New("YOUTUBE_CLIENT_ID is required")
	}
	if verifier == "" {
		var err error
		verifier, err = codeVerifier()
		if err != nil {
			return "", "", err
		}
	}
	if len(scopes) == 0 {
		scopes = []string{ScopeYouTubeForceSSL}
	}

	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	values := url.Values{}
	values.Set("client_id", clientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("response_type", "code")
	values.Set("scope", strings.Join(scopes, " "))
	values.Set("access_type", "offline")
	values.Set("prompt", "consent select_account")
	values.Set("code_challenge", challenge)
	values.Set("code_challenge_method", "S256")

	return googleAuthURL + "?" + values.Encode(), verifier, nil
}

func RunOAuthCLI(ctx context.Context, cfg OAuthConfig, stdout io.Writer) error {
	if stdout == nil {
		stdout = os.Stdout
	}
	if cfg.TokenPath == "" {
		cfg.TokenPath = defaultTokenPath
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen for oauth callback: %w", err)
	}
	defer listener.Close()

	redirectURI := "http://" + listener.Addr().String() + "/oauth2callback"
	authURL, verifier, err := AuthURL(cfg.ClientID, redirectURI, "", cfg.Scopes)
	if err != nil {
		return err
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{Handler: oauthCallbackHandler(codeCh)}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	defer server.Shutdown(context.Background())

	fmt.Fprintln(stdout, "Open this URL and approve YouTube access:")
	fmt.Fprintln(stdout, authURL)
	fmt.Fprintln(stdout, "Waiting for browser callback on "+redirectURI)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case code := <-codeCh:
		source := NewTokenSource(cfg)
		token, err := source.ExchangeCode(ctx, code, redirectURI, verifier)
		if err != nil {
			return err
		}
		if token.RefreshToken == "" {
			fmt.Fprintln(stdout, "Token saved, but refresh_token missing. Revoke app access and run auth again if refresh fails.")
		} else {
			fmt.Fprintln(stdout, "YouTube token saved to "+cfg.TokenPath)
		}
		return nil
	}
}

func oauthCallbackHandler(codeCh chan<- string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		select {
		case codeCh <- code:
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, "YouTube auth complete. Return to terminal.")
		default:
			http.Error(w, "code already received", http.StatusConflict)
		}
	})
}

func LoadToken(path string) (Token, error) {
	if path == "" {
		path = defaultTokenPath
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return Token{}, err
	}
	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return Token{}, err
	}
	return token, nil
}

func SaveToken(path string, token Token) error {
	if path == "" {
		path = defaultTokenPath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o600)
}

func codeVerifier() (string, error) {
	raw := make([]byte, 48)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
