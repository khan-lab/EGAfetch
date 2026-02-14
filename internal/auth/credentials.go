package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	credentialsDirName  = ".egafetch"
	credentialsFileName = "credentials.json"
	dirPermissions      = 0700
	filePermissions     = 0600
)

// Credentials holds the persisted authentication state.
type Credentials struct {
	Username     string    `json:"username"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// IsExpired returns true if the access token has expired or will expire
// within the given margin.
func (c *Credentials) IsExpired(margin time.Duration) bool {
	return time.Now().Add(margin).After(c.ExpiresAt)
}

// credentialsDir returns the path to ~/.egafetch/, creating it if needed.
func credentialsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, credentialsDirName)
	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return "", fmt.Errorf("cannot create credentials directory: %w", err)
	}
	return dir, nil
}

// credentialsPath returns the full path to credentials.json.
func credentialsPath() (string, error) {
	dir, err := credentialsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credentialsFileName), nil
}

// LoadCredentials reads credentials from disk.
// Returns (nil, nil) if the file does not exist (user never logged in).
func LoadCredentials() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("cannot parse credentials: %w", err)
	}
	return &creds, nil
}

// SaveCredentials writes credentials to disk atomically (write to temp file, then rename).
func SaveCredentials(creds *Credentials) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal credentials: %w", err)
	}

	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("cannot write credentials: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("cannot sync credentials file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("cannot close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, filePermissions); err != nil {
		return fmt.Errorf("cannot set permissions on credentials file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("cannot rename credentials file: %w", err)
	}

	success = true
	return nil
}

// DeleteCredentials removes the credentials file (logout).
func DeleteCredentials() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
