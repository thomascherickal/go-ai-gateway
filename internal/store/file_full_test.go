package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStoreErrorPathsFull(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	dir := t.TempDir()
	if _, err := NewFileStore(dir, false); err == nil {
		t.Fatal("expected read dir error")
	}
	emptyFile := filepath.Join(dir, "empty.json")
	_ = os.WriteFile(emptyFile, nil, 0o600)
	if _, err := NewFileStore(emptyFile, false); err != nil {
		t.Fatalf("empty file load: %v", err)
	}
	badJSON := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(badJSON, []byte("{bad"), 0o600)
	if _, err := NewFileStore(badJSON, false); err == nil {
		t.Fatal("expected bad json error")
	}
	fs, _ := NewFileStore(filepath.Join(dir, "ok.json"), false)
	_, _ = fs.UpsertTeam(ctx, UpsertTeamInput{})
	_, _ = fs.ReserveBudget(ctx, "missing", 10, now)
	_, _ = fs.ReconcileBudget(ctx, "missing", 10, now)
	_, _ = fs.ResetTeamBudget(ctx, "missing", now)
	roDir := filepath.Join(dir, "ro")
	_ = os.Mkdir(roDir, 0o500)
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o700) })
	if _, err := NewFileStore(filepath.Join(roDir, "sub", "state.json"), true); err == nil {
		t.Fatal("expected persist mkdir error")
	}
	fs.path = filepath.Join(roDir, "state.json")
	if err := fs.persist(); err == nil {
		t.Fatal("expected write tmp error")
	}
}
