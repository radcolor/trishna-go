package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMonitorSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			_, _ = w.Write([]byte(`{"version":"0.30.8"}`))
		case "/api/ps":
			_, _ = w.Write([]byte(`{"models":[{"name":"gemma4:e2b","size_vram":1744809491,"context_length":4096,"details":{"parameter_size":"5.1B","quantization_level":"Q4_K_M"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	monitor := NewMonitor(server.URL, "gemma4:e2b")
	status := monitor.Snapshot(context.Background())
	if !status.Available {
		t.Fatalf("expected ollama available, err=%q", status.Error)
	}
	if status.Version != "0.30.8" {
		t.Fatalf("version = %q", status.Version)
	}
	if len(status.LoadedModels) != 1 || status.LoadedModels[0].Name != "gemma4:e2b" {
		t.Fatalf("models = %+v", status.LoadedModels)
	}
}

func TestMonitorSnapshotUnavailable(t *testing.T) {
	monitor := NewMonitor("http://127.0.0.1:1", "gemma4:e2b")
	status := monitor.Snapshot(context.Background())
	if status.Available {
		t.Fatal("expected ollama unavailable")
	}
	if status.Error == "" {
		t.Fatal("expected error")
	}
}
