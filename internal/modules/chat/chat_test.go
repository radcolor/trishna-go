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

func TestTrimForDiscord(t *testing.T) {
	long := strings.Repeat("a", 2005)
	got := trimForDiscord(long)
	if len(got) != maxDiscordMessage {
		t.Fatalf("len = %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatal("expected ellipsis suffix")
	}
}

func ptrSnowflake(id uint64) *snowflake.ID {
	value := snowflake.ID(id)
	return &value
}
