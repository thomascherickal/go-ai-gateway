//go:build !js

package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/thomas/go-ai-gateway/internal/types"
)

func (f *FileStore) persist() error {
	f.saveMu.Lock()
	defer f.saveMu.Unlock()

	f.mu.RLock()
	snap := fileSnapshot{
		Teams:  make(map[string]persistedTeam, len(f.teams)),
		Audits: make([]types.AuditEntry, len(f.audits)),
	}
	for k, v := range f.teams {
		snap.Teams[k] = persistedTeam{TeamBudget: v, APIKeyHash: v.APIKeyHash}
	}
	copy(snap.Audits, f.audits)
	f.mu.RUnlock()

	data, _ := json.MarshalIndent(snap, "", "  ")
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
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
	_ = f.MemoryStore.RecordAudit(ctx, entry)
	return f.persist()
}
