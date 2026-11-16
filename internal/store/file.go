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
// When seed is true and no teams exist yet on disk, demo teams are seeded.
func NewFileStore(path string, seed bool) (*FileStore, error) {
	mem := NewMemoryStore(false)
	fs := &FileStore{
		MemoryStore: mem,
		path:        path,
	}

	if err := fs.load(); err != nil {
		return nil, err
	}
	if seed && len(mem.teams) == 0 {
		for _, t := range DemoTeams(time.Now().UTC()) {
			if _, err := mem.UpsertTeam(context.Background(), t); err != nil {
				return nil, err
			}
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

func (f *FileStore) persist() error {
	f.saveMu.Lock()
	defer f.saveMu.Unlock()

	f.mu.RLock()
	snap := fileSnapshot{
		Teams:  make(map[string]persistedTeam, len(f.teams)),
		Audits: make([]types.AuditEntry, len(f.audits)),
	}
	for k, v := range f.teams {
		snap.Teams[k] = persistedTeam{
			TeamBudget: v,
			APIKeyHash: v.APIKeyHash,
		}
	}
	copy(snap.Audits, f.audits)
	f.mu.RUnlock()

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func (f *FileStore) UpsertTeam(ctx context.Context, in UpsertTeamInput) (types.TeamBudget, error) {
	t, err := f.MemoryStore.UpsertTeam(ctx, in)
	if err != nil {
		return types.TeamBudget{}, err
	}
	return t, f.persist()
}

func (f *FileStore) ReserveBudget(ctx context.Context, teamID string, amountMicros int64, now time.Time) (types.TeamBudget, error) {
	t, err := f.MemoryStore.ReserveBudget(ctx, teamID, amountMicros, now)
	if err != nil {
		return t, err
	}
	return t, f.persist()
}

func (f *FileStore) ReconcileBudget(ctx context.Context, teamID string, deltaMicros int64, now time.Time) (types.TeamBudget, error) {
	t, err := f.MemoryStore.ReconcileBudget(ctx, teamID, deltaMicros, now)
	if err != nil {
		return t, err
	}
	return t, f.persist()
}

func (f *FileStore) ResetTeamBudget(ctx context.Context, teamID string, now time.Time) (types.TeamBudget, error) {
	t, err := f.MemoryStore.ResetTeamBudget(ctx, teamID, now)
	if err != nil {
		return t, err
	}
	return t, f.persist()
}

func (f *FileStore) RecordAudit(ctx context.Context, entry types.AuditEntry) error {
	if err := f.MemoryStore.RecordAudit(ctx, entry); err != nil {
		return err
	}
	return f.persist()
}
