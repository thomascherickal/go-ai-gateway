package config

import "testing"

func TestDefaultPersonalEnvPaths(t *testing.T) {
	paths := DefaultPersonalEnvPaths()
	if len(paths) == 0 {
		t.Fatal("expected at least one default personal env path")
	}
}
