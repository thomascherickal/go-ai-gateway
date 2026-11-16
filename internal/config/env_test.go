//go:build !js

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFiles(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "personal.env")
	content := `# Personal AI keys
ANTHROPIC_API_KEY="sk-ant-test-123"
export GEMINI_API_KEY='gem-test-456'
EXISTING_VAR=from_file
`
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EXISTING_VAR", "from_shell")
	t.Cleanup(func() {
		_ = os.Unsetenv("ANTHROPIC_API_KEY")
		_ = os.Unsetenv("GEMINI_API_KEY")
	})

	loaded := LoadEnvFiles(filepath.Join(dir, "missing.env"), envPath)
	if len(loaded) != 1 || loaded[0] != envPath {
		t.Fatalf("unexpected loaded list: %v", loaded)
	}
	if got := os.Getenv("ANTHROPIC_API_KEY"); got != "sk-ant-test-123" {
		t.Fatalf("ANTHROPIC_API_KEY = %q", got)
	}
	if got := os.Getenv("GEMINI_API_KEY"); got != "gem-test-456" {
		t.Fatalf("GEMINI_API_KEY = %q", got)
	}
	if got := os.Getenv("EXISTING_VAR"); got != "from_shell" {
		t.Fatalf("existing env var was overwritten: %q", got)
	}
}

