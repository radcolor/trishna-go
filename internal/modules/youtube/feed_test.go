package youtube

import (
	"strings"
	"testing"
)

const sampleFeedXML = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns:media="http://search.yahoo.com/mrss/" xmlns="http://www.w3.org/2005/Atom">
 <entry>
  <yt:videoId>jJ_z_hxJPhg</yt:videoId>
  <title>Live Sky: COTL</title>
  <link rel="alternate" href="https://www.youtube.com/watch?v=jJ_z_hxJPhg"/>
  <media:group>
   <media:description>Line one of the stream description.
Line two with more details.
Line three should be hidden.</media:description>
   <media:thumbnail url="https://i3.ytimg.com/vi/jJ_z_hxJPhg/hqdefault.jpg"/>
  </media:group>
 </entry>
 <entry>
  <yt:videoId>ZEWZH4CH35k</yt:videoId>
  <title>Older Stream</title>
  <link rel="alternate" href="https://www.youtube.com/watch?v=ZEWZH4CH35k"/>
  <media:group>
   <media:thumbnail url="https://i3.ytimg.com/vi/ZEWZH4CH35k/hqdefault.jpg"/>
  </media:group>
 </entry>
</feed>`

func TestParseFeed(t *testing.T) {
	videos, err := ParseFeed(strings.NewReader(sampleFeedXML))
	if err != nil {
		t.Fatalf("parse feed: %v", err)
	}
	if len(videos) != 2 {
		t.Fatalf("videos len = %d", len(videos))
	}

	first := videos[0]
	if first.ID != "jJ_z_hxJPhg" {
		t.Fatalf("first id = %q", first.ID)
	}
	if first.Title != "Live Sky: COTL" {
		t.Fatalf("first title = %q", first.Title)
	}
	if first.URL != "https://www.youtube.com/watch?v=jJ_z_hxJPhg" {
		t.Fatalf("first url = %q", first.URL)
	}
	if first.Thumbnail != "https://i3.ytimg.com/vi/jJ_z_hxJPhg/hqdefault.jpg" {
		t.Fatalf("first thumbnail = %q", first.Thumbnail)
	}
	if !strings.Contains(first.Description, "Line one") {
		t.Fatalf("first description = %q", first.Description)
	}
	if first.BestThumbnail() != "https://i.ytimg.com/vi/jJ_z_hxJPhg/maxresdefault.jpg" {
		t.Fatalf("first best thumbnail = %q", first.BestThumbnail())
	}
}

func TestFeedURL(t *testing.T) {
	url := FeedURL("UCRMARomI-6vCRfOXQOayBPg")
	want := "https://www.youtube.com/feeds/videos.xml?channel_id=UCRMARomI-6vCRfOXQOayBPg"
	if url != want {
		t.Fatalf("feed url = %q", url)
	}
}
