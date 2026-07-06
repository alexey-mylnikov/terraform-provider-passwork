package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexey-mylnikov/passwork-go/passwork"
)

func TestResolveKey_InlineKey(t *testing.T) {
	c := &pwClient{inlineKey: "mykey123"}
	key, err := c.resolveKey(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "mykey123" {
		t.Errorf("want %q, got %q", "mykey123", key)
	}
}

func TestResolveKey_GeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "session.key")

	c := &pwClient{sessionKeyFile: keyFile}

	key1, err := c.resolveKey(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key1 == "" {
		t.Fatal("expected a non-empty key")
	}

	data, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("key file not written: %v", err)
	}
	if string(data) != key1 {
		t.Errorf("persisted key %q != returned key %q", string(data), key1)
	}

	// Second call should return the same key.
	key2, err := c.resolveKey(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key1 != key2 {
		t.Errorf("key changed between calls: %q vs %q", key1, key2)
	}
}

func TestResolveKey_NoKeyFile(t *testing.T) {
	c := &pwClient{sessionKeyFile: ""}
	key, err := c.resolveKey(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "" {
		t.Errorf("expected empty key when no file configured, got %q", key)
	}
}

func TestResolveKey_MissingFileNoGenerate(t *testing.T) {
	c := &pwClient{sessionKeyFile: "/nonexistent/path/session.key"}
	_, err := c.resolveKey(false)
	if err == nil {
		t.Fatal("expected error when key file missing and generate=false")
	}
}

func TestTryLoadSession_NoFile(t *testing.T) {
	c := &pwClient{sessionFile: "/nonexistent/session"}
	if c.tryLoadSession(nil) {
		t.Error("expected false when session file doesn't exist")
	}
}

func TestTryLoadSession_EmptySessionFile(t *testing.T) {
	c := &pwClient{sessionFile: ""}
	if c.tryLoadSession(nil) {
		t.Error("expected false when sessionFile is empty")
	}
}

func TestDefaultSessionPaths_DifferentHostsGetDifferentPaths(t *testing.T) {
	f1, k1 := defaultSessionPaths("https://passwork.example.com")
	f2, k2 := defaultSessionPaths("https://other.example.com")

	if f1 == f2 {
		t.Error("different hosts must produce different session file paths")
	}
	if k1 == k2 {
		t.Error("different hosts must produce different key file paths")
	}
}

func TestDefaultSessionPaths_SameHostGetsSamePath(t *testing.T) {
	f1, _ := defaultSessionPaths("https://passwork.example.com")
	f2, _ := defaultSessionPaths("https://passwork.example.com")

	if f1 != f2 {
		t.Error("same host must always produce the same session file path")
	}
}

func TestDefaultSessionPaths_UnderTerraformD(t *testing.T) {
	f, _ := defaultSessionPaths("https://passwork.example.com")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".terraform.d", "passwork", "sessions")

	if !filepath.HasPrefix(f, expected) {
		t.Errorf("session path %q should be under %q", f, expected)
	}
}

// TestAuthenticate_StaleCacheFallsBackToConfiguredTokens reproduces the bug
// where a session cached on disk had gone dead (server returns a plain 401,
// not "accessTokenExpired") and freshly issued access_token/refresh_token
// from provider configuration were silently ignored in favour of the stale
// cache, permanently locking the user out until they deleted the cache file
// by hand.
func TestAuthenticate_StaleCacheFallsBackToConfiguredTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/app/features" {
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
		switch r.Header.Get("Authorization") {
		case "Bearer fresh-access":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("[]"))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{
					{"code": "notFound", "message": "Access token not found"},
				},
			})
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session")
	sessionKeyFile := filepath.Join(dir, "session.key")

	// Seed a cached session on disk holding a dead token pair, exactly as a
	// previous, now-expired terraform apply would have left behind.
	stale := passwork.New(srv.URL)
	stale.SetTokens("stale-access", "stale-refresh")
	key, err := stale.SaveSession(sessionFile, "", true)
	if err != nil {
		t.Fatalf("seed stale session: %v", err)
	}
	if err := os.WriteFile(sessionKeyFile, []byte(key), 0o600); err != nil {
		t.Fatalf("write session key file: %v", err)
	}

	c := &pwClient{
		Client:         passwork.New(srv.URL, passwork.WithAutoRefresh()),
		sessionFile:    sessionFile,
		sessionKeyFile: sessionKeyFile,
	}

	// Simulate the user issuing a brand new token pair in provider config
	// after hitting the "Access token not found" error.
	if err := c.authenticate(context.Background(), "fresh-access", "fresh-refresh", "", ""); err != nil {
		t.Fatalf("authenticate should fall back to configured tokens, got error: %v", err)
	}

	// The stale cache must be overwritten with the now-working tokens so
	// subsequent applies don't need `rm -rf` as a workaround.
	c.saveSession()
	reloaded := &pwClient{
		Client:         passwork.New(srv.URL),
		sessionFile:    sessionFile,
		sessionKeyFile: sessionKeyFile,
	}
	if !reloaded.tryLoadSession(context.Background()) {
		t.Fatal("expected the refreshed session to be loadable")
	}
}

// TestAuthenticate_NoCacheUsesConfiguredTokens is the ordinary first-run path:
// no session file exists yet, so the configured tokens are used directly.
func TestAuthenticate_NoCacheUsesConfiguredTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh-access" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"code": "notFound", "message": "Access token not found"}},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := &pwClient{
		Client:         passwork.New(srv.URL, passwork.WithAutoRefresh()),
		sessionFile:    filepath.Join(dir, "session"),
		sessionKeyFile: filepath.Join(dir, "session.key"),
	}

	if err := c.authenticate(context.Background(), "fresh-access", "fresh-refresh", "", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestAuthenticate_BothStaleAndConfiguredTokensFail ensures the error surfaces
// clearly when neither the cached session nor the configured tokens work.
func TestAuthenticate_BothStaleAndConfiguredTokensFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"code": "notFound", "message": "Access token not found"}},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session")
	sessionKeyFile := filepath.Join(dir, "session.key")

	stale := passwork.New(srv.URL)
	stale.SetTokens("stale-access", "stale-refresh")
	key, err := stale.SaveSession(sessionFile, "", true)
	if err != nil {
		t.Fatalf("seed stale session: %v", err)
	}
	if err := os.WriteFile(sessionKeyFile, []byte(key), 0o600); err != nil {
		t.Fatalf("write session key file: %v", err)
	}

	c := &pwClient{
		Client:         passwork.New(srv.URL, passwork.WithAutoRefresh()),
		sessionFile:    sessionFile,
		sessionKeyFile: sessionKeyFile,
	}

	if err := c.authenticate(context.Background(), "also-bad-access", "also-bad-refresh", "", ""); err == nil {
		t.Fatal("expected an error when both cached and configured tokens fail")
	}
}
