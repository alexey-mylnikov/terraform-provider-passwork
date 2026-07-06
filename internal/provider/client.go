package provider

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/alexey-mylnikov/passwork-go/passwork"
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

	if err := c.authenticate(ctx, accessToken, refreshToken, masterPassword, masterKey); err != nil {
		return nil, err
	}

	c.saveSession()
	return c, nil
}

// authenticate establishes a working session on c, preferring a cached
// session loaded from disk when one is available. A cached session only has
// to decrypt successfully to be tried — it may still be dead (both its
// access and refresh tokens expired, or purged server-side after a long
// TTL), which the server reports as a plain 401 rather than the
// "accessTokenExpired" code that triggers transparent refresh. When that
// happens, fall back to the token pair from the provider configuration
// instead of surfacing the error immediately, so a freshly issued
// access_token/refresh_token is not silently ignored in favour of a stale
// cache.
func (c *pwClient) authenticate(ctx context.Context, accessToken, refreshToken, masterPassword, masterKey string) error {
	loaded := c.tryLoadSession(ctx)
	if !loaded {
		if err := c.applyConfiguredTokens(ctx, accessToken, refreshToken, masterPassword, masterKey); err != nil {
			return err
		}
	}

	// Verify connectivity; WithAutoRefresh will transparently renew an expired
	// access token. The result is discarded — we only care about auth success.
	_, cacheErr := c.GetFeatures(ctx)
	if cacheErr == nil {
		return nil
	}
	if !loaded {
		return fmt.Errorf("passwork: authentication failed: %w", cacheErr)
	}

	// The cached session turned out to be unusable. Retry once with the
	// token pair from provider configuration before giving up.
	if err := c.applyConfiguredTokens(ctx, accessToken, refreshToken, masterPassword, masterKey); err != nil {
		return fmt.Errorf("passwork: authentication failed: cached session invalid (%v), and %w", cacheErr, err)
	}
	if _, err := c.GetFeatures(ctx); err != nil {
		return fmt.Errorf("passwork: authentication failed: cached session invalid (%v), configured credentials also failed: %w", cacheErr, err)
	}
	return nil
}

// applyConfiguredTokens sets the client to use the access/refresh token pair
// from provider configuration, along with client-side encryption if requested.
func (c *pwClient) applyConfiguredTokens(ctx context.Context, accessToken, refreshToken, masterPassword, masterKey string) error {
	c.SetTokens(accessToken, refreshToken)
	if masterPassword != "" {
		if err := c.SetMasterPassword(ctx, masterPassword); err != nil {
			return fmt.Errorf("set master password: %w", err)
		}
	} else if masterKey != "" {
		if err := c.SetMasterKey(ctx, masterKey); err != nil {
			return fmt.Errorf("set master key: %w", err)
		}
	}
	return nil
}

// defaultSessionPaths returns the global session file and key file paths for
// the given host. Sessions are stored under ~/.terraform.d/passwork/sessions/
// in a directory named by the first 8 bytes of SHA-256(host), so each
// Passwork instance gets its own slot and all Terraform workspaces pointing at
// the same host share one session automatically.
func defaultSessionPaths(host string) (sessionFile, keyFile string) {
	home, err := os.UserHomeDir()
	if err != nil {
		// Unreachable in practice; fall back to cwd-relative path.
		return ".terraform/passwork_session", ".terraform/passwork_session.key"
	}
	sum := sha256.Sum256([]byte(host))
	dir := filepath.Join(home, ".terraform.d", "passwork", "sessions", hex.EncodeToString(sum[:8]))
	return filepath.Join(dir, "session"), filepath.Join(dir, "session.key")
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
