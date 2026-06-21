package chat

import (
	"strings"
	"testing"

	"github.com/disgoorg/snowflake/v2"
)

func TestAllowedLocation(t *testing.T) {
	channelID := snowflake.ID(999)
	mod := New(Deps{
		AllowedUserIDs:    []snowflake.ID{1},
		AllowedChannelIDs: []snowflake.ID{channelID},
		HistoryLimit:      5,
	})

	if !mod.allowedLocation(nil, channelID) {
		t.Fatal("expected DM to be allowed")
	}
	if !mod.allowedLocation(ptrSnowflake(1), channelID) {
		t.Fatal("expected allowed channel")
	}
	if mod.allowedLocation(ptrSnowflake(1), 123) {
		t.Fatal("expected disallowed channel")
	}
}

func TestSplitForDiscord(t *testing.T) {
	long := strings.Repeat("a", 2005)
	got := splitForDiscord(long)
	if len(got) != 2 {
		t.Fatalf("parts = %d", len(got))
	}
	if len([]rune(got[0])) != maxDiscordMessage {
		t.Fatalf("first len = %d", len([]rune(got[0])))
	}
	if len([]rune(got[1])) != 5 {
		t.Fatalf("second len = %d", len([]rune(got[1])))
	}
	if strings.Join(got, "") != long {
		t.Fatal("expected content to remain complete")
	}
}

func ptrSnowflake(id uint64) *snowflake.ID {
	value := snowflake.ID(id)
	return &value
}
