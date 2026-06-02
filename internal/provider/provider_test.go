package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEnvOr_UsesAttrValue(t *testing.T) {
	os.Setenv("TEST_VAR", "env_value")
	defer os.Unsetenv("TEST_VAR")

	got := envOr(types.StringValue("attr_value"), "TEST_VAR", "default")
	if got != "attr_value" {
		t.Errorf("want %q, got %q", "attr_value", got)
	}
}

func TestEnvOr_FallsBackToEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "env_value")
	defer os.Unsetenv("TEST_VAR")

	got := envOr(types.StringNull(), "TEST_VAR", "default")
	if got != "env_value" {
		t.Errorf("want %q, got %q", "env_value", got)
	}
}

func TestEnvOr_FallsBackToDefault(t *testing.T) {
	os.Unsetenv("TEST_VAR")

	got := envOr(types.StringNull(), "TEST_VAR", "default")
	if got != "default" {
		t.Errorf("want %q, got %q", "default", got)
	}
}

func TestEnvOr_EmptyAttrFallsToEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "env_value")
	defer os.Unsetenv("TEST_VAR")

	// Empty string attribute should be treated as unset.
	got := envOr(types.StringValue(""), "TEST_VAR", "default")
	if got != "env_value" {
		t.Errorf("want %q, got %q", "env_value", got)
	}
}

func TestFileExists_ExistingFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if !fileExists(f.Name()) {
		t.Error("expected true for existing file")
	}
}

func TestFileExists_MissingFile(t *testing.T) {
	if fileExists("/nonexistent/path/file.txt") {
		t.Error("expected false for missing file")
	}
}

func TestFileExists_Directory(t *testing.T) {
	dir := t.TempDir()
	if fileExists(dir) {
		t.Error("expected false for directory")
	}
}
