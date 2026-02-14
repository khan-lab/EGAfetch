package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// EGA OAuth2 token endpoint.
	tokenEndpoint = "https://ega.ebi.ac.uk:8443/ega-openid-connect-server/token"

	// Refresh the token 5 minutes before it expires.
	tokenRefreshMargin = 5 * time.Minute

	// Default token lifetime if the server does not specify expires_in.
	// EGA tokens typically last ~1 hour.
	defaultTokenLifetime = 1 * time.Hour

	// Client credentials for the EGA OIDC application.
	// These are public values from pyEGA3 and are not user secrets.
	clientID     = "f20cd2d3-682a-4568-a53e-4262ef54c8f4"
	clientSecret = "AMenuDLjVdVo4BSwi0QD54LL6NeVDEZRzEQUJ7hJOM3g4imDZBHHX0hNfKHPeQIGkskhtCmqAJtt_jm7EKq-rWw"
	grantScope   = "openid"

	// Metadata API uses a separate IdP and client.
	metadataTokenEndpoint = "https://idp.ega-archive.org/realms/EGA/protocol/openid-connect/token"
	metadataClientID      = "metadata-api"
)

// TokenProvider is the interface that the API client uses to get a valid
// access token. This allows the API client to be tested with a mock.
type TokenProvider interface {
	GetAccessToken(ctx context.Context) (string, error)
}

// Manager manages OAuth2 authentication against the EGA AAI.
// It implements TokenProvider and is safe for concurrent use.
type Manager struct {
	mu         sync.Mutex
	creds      *Credentials
	httpClient *http.Client
}

// Compile-time check that Manager implements TokenProvider.
var _ TokenProvider = (*Manager)(nil)

// NewManager creates an auth manager. It attempts to load existing
// credentials from disk. If none exist, methods that require authentication
// will return an error prompting the user to log in.
func NewManager() (*Manager, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}
	return &Manager{
		creds:      creds,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Login authenticates with username and password, stores the resulting tokens.
func (m *Manager) Login(ctx context.Context, username, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	creds, err := m.requestToken(ctx, tokenEndpoint, url.Values{
		"grant_type":    {"password"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {grantScope},
		"username":      {username},
		"password":      {password},
	})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	creds.Username = username
	m.creds = creds

	if err := SaveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}
	return nil
}

// GetAccessToken returns a valid access token. If the token is expired
// or about to expire, it refreshes automatically.
func (m *Manager) GetAccessToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.creds == nil {
		return "", fmt.Errorf("not authenticated; run 'egafetch auth login' first")
	}

	if !m.creds.IsExpired(tokenRefreshMargin) {
		return m.creds.AccessToken, nil
	}

	if err := m.refreshLocked(ctx); err != nil {
		return "", err
	}
	return m.creds.AccessToken, nil
}

// refreshLocked performs a token refresh. Caller must hold m.mu.
func (m *Manager) refreshLocked(ctx context.Context) error {
	if m.creds == nil || m.creds.RefreshToken == "" {
		return fmt.Errorf("no refresh token available; run 'egafetch auth login'")
	}

	creds, err := m.requestToken(ctx, tokenEndpoint, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {m.creds.RefreshToken},
	})
	if err != nil {
		return fmt.Errorf("token refresh failed: %w", err)
	}
	creds.Username = m.creds.Username
	m.creds = creds

	if err := SaveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save refreshed credentials: %w", err)
	}
	return nil
}

// Logout clears stored credentials from memory and disk.
func (m *Manager) Logout() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.creds = nil
	return DeleteCredentials()
}

// Status returns the current credentials (or nil if not logged in).
// Does not refresh the token.
func (m *Manager) Status() *Credentials {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.creds
}

// Username returns the stored username, or empty string if not logged in.
func (m *Manager) Username() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.creds == nil {
		return ""
	}
	return m.creds.Username
}

// GetMetadataToken authenticates against the EGA metadata API IdP and returns
// a short-lived access token. This uses a separate IdP from the download API.
func (m *Manager) GetMetadataToken(ctx context.Context, password string) (string, error) {
	m.mu.Lock()
	username := ""
	if m.creds != nil {
		username = m.creds.Username
	}
	m.mu.Unlock()

	if username == "" {
		return "", fmt.Errorf("not authenticated; run 'egafetch auth login' first")
	}

	creds, err := m.requestToken(ctx, metadataTokenEndpoint, url.Values{
		"grant_type": {"password"},
		"client_id":  {metadataClientID},
		"username":   {username},
		"password":   {password},
	})
	if err != nil {
		return "", fmt.Errorf("metadata auth failed: %w", err)
	}
	return creds.AccessToken, nil
}

// tokenResponse is the JSON structure returned by the EGA token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// requestToken performs the actual HTTP POST to the given token endpoint.
func (m *Manager) requestToken(ctx context.Context, endpoint string, params url.Values) (*Credentials, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var tokResp tokenResponse
		_ = json.Unmarshal(body, &tokResp)
		if tokResp.ErrorDesc != "" {
			return nil, fmt.Errorf("authentication error (%d): %s", resp.StatusCode, tokResp.ErrorDesc)
		}
		return nil, fmt.Errorf("authentication error (%d): %s", resp.StatusCode, string(body))
	}

	var tokResp tokenResponse
	if err := json.Unmarshal(body, &tokResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	lifetime := time.Duration(tokResp.ExpiresIn) * time.Second
	if lifetime <= 0 {
		lifetime = defaultTokenLifetime
	}

	return &Credentials{
		AccessToken:  tokResp.AccessToken,
		RefreshToken: tokResp.RefreshToken,
		ExpiresAt:    time.Now().Add(lifetime),
	}, nil
}
