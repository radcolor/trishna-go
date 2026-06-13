package youtube

import (
	"testing"
)

func TestFindNewVideosFirstRun(t *testing.T) {
	videos := []Video{
		{ID: "newest"},
		{ID: "older"},
	}

	newVideos, rebaseline := findNewVideos(videos, "")
	if rebaseline {
		t.Fatal("expected no rebaseline on first run")
	}
	if len(newVideos) != 0 {
		t.Fatalf("new videos len = %d", len(newVideos))
	}
}

func TestFindNewVideosDetectsNewEntries(t *testing.T) {
	videos := []Video{
		{ID: "newest"},
		{ID: "middle"},
		{ID: "last-seen"},
		{ID: "older"},
	}

	newVideos, rebaseline := findNewVideos(videos, "last-seen")
	if rebaseline {
		t.Fatal("expected no rebaseline")
	}
	if len(newVideos) != 2 {
		t.Fatalf("new videos len = %d", len(newVideos))
	}
	if newVideos[0].ID != "newest" || newVideos[1].ID != "middle" {
		t.Fatalf("new videos = %#v", newVideos)
	}
}

func TestFindNewVideosRebaselineWhenLastSeenMissing(t *testing.T) {
	videos := []Video{
		{ID: "newest"},
		{ID: "older"},
	}

	newVideos, rebaseline := findNewVideos(videos, "missing")
	if !rebaseline {
		t.Fatal("expected rebaseline")
	}
	if len(newVideos) != 0 {
		t.Fatalf("new videos len = %d", len(newVideos))
	}
}

func TestResolveWatchesSkipsMissingWebhook(t *testing.T) {
	service, err := NewServiceWithDeps(nil, nil, nil, mapLookup(nil))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	watches, err := service.resolveWatches()
	if err != nil {
		t.Fatalf("resolve watches: %v", err)
	}
	if len(watches) != 0 {
		t.Fatalf("watches len = %d", len(watches))
	}
}

func TestResolveWatchesIncludesConfiguredWebhook(t *testing.T) {
	service, err := NewServiceWithDeps(nil, nil, nil, mapLookup(map[string]string{
		EnvWebhookShnkplays: "https://discord.com/api/webhooks/1/token",
	}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	watches, err := service.resolveWatches()
	if err != nil {
		t.Fatalf("resolve watches: %v", err)
	}
	if len(watches) != 1 {
		t.Fatalf("watches len = %d", len(watches))
	}
	if watches[0].channel.Name != "shnk" {
		t.Fatalf("channel name = %q", watches[0].channel.Name)
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
