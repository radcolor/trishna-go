package reminder

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/disgoorg/snowflake/v2"
)

func TestStoreAddDueRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reminders.json")

	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}

	userID := snowflake.ID(111)
	channelID := snowflake.ID(222)
	dueAt := time.Now().UTC().Add(time.Hour)

	item, err := store.Add(userID, channelID, "drink water", dueAt, "remind me")
	if err != nil {
		t.Fatal(err)
	}
	if item.ID == "" {
		t.Fatal("expected id")
	}

	due, err := store.Due(time.Now().UTC().Add(2 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 {
		t.Fatalf("due count = %d", len(due))
	}

	due, err = store.Due(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("expected no due reminders yet, got %d", len(due))
	}

	if err := store.Remove(item.ID); err != nil {
		t.Fatal(err)
	}

	all, err := store.Due(time.Now().UTC().Add(2 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty store, got %d", len(all))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("expected reminders file")
	}
}

func TestStoreRemoveAllForUser(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "reminders.json"))
	if err != nil {
		t.Fatal(err)
	}

	userA := snowflake.ID(1)
	userB := snowflake.ID(2)
	channelID := snowflake.ID(99)
	dueAt := time.Now().UTC().Add(time.Hour)

	if _, err := store.Add(userA, channelID, "a", dueAt, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(userA, channelID, "b", dueAt, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(userB, channelID, "c", dueAt, ""); err != nil {
		t.Fatal(err)
	}

	removed, err := store.RemoveAllForUser(userA)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d", removed)
	}

	left, err := store.ListForUser(userB)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 {
		t.Fatalf("user B reminders = %d", len(left))
	}
}

func TestStoreIncrementSendAttempts(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "reminders.json"))
	if err != nil {
		t.Fatal(err)
	}

	item, err := store.Add(snowflake.ID(1), snowflake.ID(2), "x", time.Now().UTC().Add(time.Minute), "")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.IncrementSendAttempts(item.ID); err != nil {
		t.Fatal(err)
	}

	due, err := store.Due(time.Now().UTC().Add(2 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 {
		t.Fatal("expected reminder")
	}
	if due[0].SendAttempts != 1 {
		t.Fatalf("attempts = %d", due[0].SendAttempts)
	}
}
