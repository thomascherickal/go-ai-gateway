//go:build !js

package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnvFiles loads KEY=VALUE pairs from the first readable files without overriding existing env vars.
func LoadEnvFiles(paths ...string) []string {
	var loaded []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := loadSingleEnvFile(p); err == nil {
			loaded = append(loaded, p)
		}
	}
	return loaded
}

// DefaultPersonalEnvPaths returns standard local and XDG config paths for personal deployments.
func DefaultPersonalEnvPaths() []string {
	paths := []string{".env.personal", ".env"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".config", "go-ai-gateway", "env"))
	}
	return paths
}

func loadSingleEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		val := strings.TrimSpace(v)
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		_ = os.Setenv(key, val)
	}
	return scanner.Err()
}

