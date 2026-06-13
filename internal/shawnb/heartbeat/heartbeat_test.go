package heartbeat

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/radcolor/trishna-go/internal/runtime"
)

func TestStoreWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ts := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	if err := store.Write(Snapshot{
		Ready: true, UpdatedAt: ts, Bot: "shawnb", Model: "gemma4:e2b",
		UptimeSec: 3600, Goroutines: 12, ProcessRSS: 1024, ProcessCPUPercent: 1.5,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !got.Ready || got.Bot != "shawnb" || got.Model != "gemma4:e2b" || got.Goroutines != 12 {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestServiceWritesOfflineOnShutdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	runtimeState := runtime.NewState()
	runtimeState.MarkReady()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- NewService(store, runtimeState, "shawnb", "gemma4:e2b").Run(ctx)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("service run: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Ready {
		t.Fatal("expected ready=false after shutdown")
	}
}
