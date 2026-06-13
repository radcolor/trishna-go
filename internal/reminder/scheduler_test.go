package reminder

import (
	"context"
	"testing"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/radcolor/trishna-go/internal/runtime"
)

func TestSchedulerTickSkipsWhenNotReady(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/reminders.json")
	if err != nil {
		t.Fatal(err)
	}

	dueAt := time.Now().UTC().Add(-time.Minute)
	if _, err := store.Add(snowflake.ID(1), snowflake.ID(2), "test", dueAt, ""); err != nil {
		t.Fatal(err)
	}

	sender := &mockSender{}
	runtimeState := runtime.NewState()
	sched := NewScheduler(store, &mockSoulLLM{reply: "ping"}, sender, runtimeState, nil, nil)

	if err := sched.tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 0 {
		t.Fatalf("expected no sends when not ready, got %d", sender.calls)
	}
}

func TestSchedulerTickFiresWhenReady(t *testing.T) {
	store, err := NewStore(t.TempDir() + "/reminders.json")
	if err != nil {
		t.Fatal(err)
	}

	dueAt := time.Now().UTC().Add(-time.Minute)
	item, err := store.Add(snowflake.ID(1), snowflake.ID(2), "drink water", dueAt, "")
	if err != nil {
		t.Fatal(err)
	}

	sender := &mockSender{}
	runtimeState := runtime.NewState()
	runtimeState.MarkReady()

	sched := NewScheduler(store, &mockSoulLLM{reply: "Reminder: drink water"}, sender, runtimeState, nil, nil)
	if err := sched.tick(t.Context()); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 {
		t.Fatalf("expected 1 send, got %d", sender.calls)
	}

	remaining, err := store.Due(time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected reminder removed, still have %d including %s", len(remaining), item.ID)
	}
}

type mockSender struct {
	calls int
}

func (m *mockSender) CreateMessage(_ snowflake.ID, _ discord.MessageCreate, _ ...rest.RequestOpt) (*discord.Message, error) {
	m.calls++
	return nil, nil
}

type mockSoulLLM struct {
	reply string
}

func (m *mockSoulLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.reply, nil
}

func (m *mockSoulLLM) Soul() string {
	return "soul"
}
