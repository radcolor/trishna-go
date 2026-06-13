package youtube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateLoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "youtube-state.json")

	state, err := LoadState(path)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.LastSeen("channel-1") != "" {
		t.Fatalf("last seen = %q", state.LastSeen("channel-1"))
	}

	if err := state.SetLastSeen("channel-1", "video-1"); err != nil {
		t.Fatalf("set last seen: %v", err)
	}

	reloaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if reloaded.LastSeen("channel-1") != "video-1" {
		t.Fatalf("last seen = %q", reloaded.LastSeen("channel-1"))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	if !strings.Contains(string(data), `"video-1"`) {
		t.Fatalf("state file = %s", string(data))
	}
}
