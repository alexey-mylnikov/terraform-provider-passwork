package provider

import (
	"os"
	"path/filepath"
	"testing"
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
