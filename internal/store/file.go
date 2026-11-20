//go:build !js

package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

type persistedTeam struct {
	types.TeamBudget
	APIKeyHash string `json:"api_key_hash"`
}

type fileSnapshot struct {
	Teams  map[string]persistedTeam `json:"teams"`
	Audits []types.AuditEntry       `json:"audits"`
}

// FileStore wraps MemoryStore with atomic JSON file persistence for standalone/personal deployments.
type FileStore struct {
	*MemoryStore
	path   string
	saveMu sync.Mutex
}

// NewFileStore opens or initializes a durable JSON-backed store at path.
func NewFileStore(path string, seed bool) (*FileStore, error) {
	mem := NewMemoryStore(false)
	fs := &FileStore{MemoryStore: mem, path: path}
	if err := fs.load(); err != nil {
		return nil, err
	}
	if seed && len(mem.teams) == 0 {
		for _, t := range DemoTeams(time.Now().UTC()) {
			_, _ = mem.UpsertTeam(context.Background(), t)
		}
		if err := fs.persist(); err != nil {
			return nil, err
		}
	}
	return fs, nil
}

func (f *FileStore) BackendName() string { return "file:" + filepath.Base(f.path) }

func (f *FileStore) load() error {
	raw, err := os.ReadFile(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	var snap fileSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if snap.Teams != nil {
		f.teams = make(map[string]types.TeamBudget, len(snap.Teams))
		f.keyIdx = make(map[string]string, len(snap.Teams))
		for id, pt := range snap.Teams {
			t := pt.TeamBudget
			t.APIKeyHash = pt.APIKeyHash
			f.teams[id] = t
			if t.APIKeyHash != "" {
				f.keyIdx[t.APIKeyHash] = id
			}
		}
	}
	if snap.Audits != nil {
		f.audits = snap.Audits
	}
	return nil
}
