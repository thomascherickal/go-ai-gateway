package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPersonalEnvPathsAndEdgeCases(t *testing.T) {
	paths := DefaultPersonalEnvPaths()
	if len(paths) == 0 {
		t.Fatal("expected at least one default personal env path")
	}

	dir := t.TempDir()
	envPath := filepath.Join(dir, "edge.env")
	content := "# comment\n\nINVALID_NO_EQUALS\n=emptykey\nexport QUOTED_SINGLE='hello'\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("QUOTED_SINGLE") })

	loaded := LoadEnvFiles("", filepath.Join(dir, "missing.env"), envPath)
	if len(loaded) != 1 || loaded[0] != envPath {
		t.Fatalf("unexpected loaded: %v", loaded)
	}
	if got := os.Getenv("QUOTED_SINGLE"); got != "hello" {
		t.Fatalf("got %q want hello", got)
	}
}
