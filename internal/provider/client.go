package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/alexey-mylnikov/passwork-go/passwork"
)

const (
	defaultSessionFile    = ".terraform/passwork_session"
	defaultSessionKeyFile = ".terraform/passwork_session.key"
)

// pwClient wraps the passwork Client to add session persistence.
type pwClient struct {
	*passwork.Client
	sessionFile    string
	sessionKeyFile string
	// inlineKey is set when the user provides the encryption key in provider
	// config instead of relying on the auto-generated key file.
	inlineKey string
	mu        sync.Mutex
}

// newClient creates and configures a passwork client. It tries to load a
// previously saved session from sessionFile; when no cache exists it
// initialises from the supplied token pair. When masterPassword or masterKey
// is non-empty, client-side encryption is activated. sessionEncryptionKey, when
// non-empty, is used directly for session file encryption instead of the
// auto-generated key stored in sessionKeyFile.
func newClient(
	ctx context.Context,
	host string,
	accessToken, refreshToken string,
	masterPassword, masterKey string,
	skipTLS bool,
	sessionFile, sessionKeyFile, sessionEncryptionKey string,
) (*pwClient, error) {
	opts := []passwork.Option{passwork.WithAutoRefresh()}
	if skipTLS {
		opts = append(opts, passwork.WithSkipTLSVerify())
	}

	c := &pwClient{
		Client:         passwork.New(host, opts...),
		sessionFile:    sessionFile,
		sessionKeyFile: sessionKeyFile,
		inlineKey:      sessionEncryptionKey,
	}

	loaded := c.tryLoadSession(ctx)

	if !loaded {
		c.SetTokens(accessToken, refreshToken)
		if masterPassword != "" {
			if err := c.SetMasterPassword(ctx, masterPassword); err != nil {
				return nil, fmt.Errorf("set master password: %w", err)
			}
		} else if masterKey != "" {
			if err := c.SetMasterKey(ctx, masterKey); err != nil {
				return nil, fmt.Errorf("set master key: %w", err)
			}
		}
	}

	// Verify connectivity; WithAutoRefresh will transparently renew an expired
	// access token. The result is discarded — we only care about auth success.
	if _, err := c.GetFeatures(ctx); err != nil {
		return nil, fmt.Errorf("passwork: authentication failed: %w", err)
	}

	c.saveSession()
	return c, nil
}

// tryLoadSession attempts to restore a previously saved session from disk.
// Returns true when the session was successfully loaded.
func (c *pwClient) tryLoadSession(ctx context.Context) bool {
	if c.sessionFile == "" {
		return false
	}
	if _, err := os.Stat(c.sessionFile); os.IsNotExist(err) {
		return false
	}
	key, err := c.resolveKey(false)
	if err != nil || key == "" {
		return false
	}
	return c.LoadSession(ctx, c.sessionFile, key) == nil
}

// saveSession persists the current session tokens to disk. Errors are
// silently ignored so a transient write failure never aborts Terraform.
func (c *pwClient) saveSession() {
	if c.sessionFile == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	key, err := c.resolveKey(true)
	if err != nil || key == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.sessionFile), 0o700); err != nil {
		return
	}
	_, _ = c.SaveSession(c.sessionFile, key, true)
}

// resolveKey returns the encryption key to use for session file operations.
// When generate is true it creates and persists a new random key if none
// exists yet. Returns an empty string (no error) when session caching is
// disabled (no sessionFile configured).
func (c *pwClient) resolveKey(generate bool) (string, error) {
	if c.inlineKey != "" {
		return c.inlineKey, nil
	}
	if c.sessionKeyFile == "" {
		return "", nil
	}
	if data, err := os.ReadFile(c.sessionKeyFile); err == nil {
		return string(data), nil
	}
	if !generate {
		return "", fmt.Errorf("session key file not found: %s", c.sessionKeyFile)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	key := hex.EncodeToString(raw)
	_ = os.MkdirAll(filepath.Dir(c.sessionKeyFile), 0o700)
	_ = os.WriteFile(c.sessionKeyFile, []byte(key), 0o600)
	return key, nil
}
