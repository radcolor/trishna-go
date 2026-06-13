package monitor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/radcolor/trishna-go/internal/shawnb/heartbeat"
)

func TestHealthOfflineWithoutHeartbeat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")

	health := New(path).Health()
	if health.Running {
		t.Fatal("expected shawnb offline")
	}
	if health.Name != "shawnb" {
		t.Fatalf("name = %q", health.Name)
	}
}

func TestHealthOnlineWithFreshHeartbeat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")

	store, err := heartbeat.NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Write(heartbeat.Snapshot{
		Ready:     true,
		UpdatedAt: time.Now().UTC(),
		Bot:       "shawnb",
		Model:     "gemma4:e2b",
	}); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}

	health := New(path).Health()
	if !health.Running {
		t.Fatalf("expected shawnb online, got %+v", health)
	}
	if health.Detail == "" {
		t.Fatal("expected detail")
	}
}

func TestHealthStaleHeartbeat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")

	if err := os.WriteFile(path, []byte(`{"ready":true,"updated_at":"2020-01-01T00:00:00Z","bot":"shawnb"}`), 0o600); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}

	health := New(path).Health()
	if health.Running {
		t.Fatal("expected stale heartbeat to be offline")
	}
}
