package chatlog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreAppend(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ts := time.Date(2026, 6, 13, 20, 15, 0, 0, time.UTC)
	if err := store.Append(Entry{
		TS:        ts,
		UserID:    "123",
		ChannelID: "456",
		IsDM:      true,
		Role:      "user",
		Content:   "hey babe",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	path := filepath.Join(dir, "2026-06-13.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatal("expected one log line")
	}

	var entry Entry
	if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
		t.Fatalf("decode entry: %v", err)
	}
	if entry.Content != "hey babe" || entry.Role != "user" || !entry.IsDM {
		t.Fatalf("entry = %+v", entry)
	}
}
