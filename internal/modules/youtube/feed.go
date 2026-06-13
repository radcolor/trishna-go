package youtube

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Video struct {
	ID          string
	Title       string
	URL         string
	Thumbnail   string
	Description string
}

func (v Video) BestThumbnail() string {
	return "https://i.ytimg.com/vi/" + v.ID + "/maxresdefault.jpg"
}

type feed struct {
	Entries []feedEntry `xml:"http://www.w3.org/2005/Atom entry"`
}

type feedEntry struct {
	VideoID string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
	Title   string `xml:"http://www.w3.org/2005/Atom title"`
	Links   []struct {
		Rel  string `xml:"rel,attr"`
		Href string `xml:"href,attr"`
	} `xml:"http://www.w3.org/2005/Atom link"`
	MediaGroup struct {
		Title       string `xml:"http://search.yahoo.com/mrss/ title"`
		Description string `xml:"http://search.yahoo.com/mrss/ description"`
		Thumbnail   struct {
			URL string `xml:"url,attr"`
		} `xml:"http://search.yahoo.com/mrss/ thumbnail"`
	} `xml:"http://search.yahoo.com/mrss/ group"`
}

type FeedClient struct {
	httpClient *http.Client
}

func NewFeedClient() *FeedClient {
	return &FeedClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *FeedClient) FetchVideos(channelID string) ([]Video, error) {
	req, err := http.NewRequest(http.MethodGet, FeedURL(channelID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch feed for %s: %s", channelID, resp.Status)
	}

	return parseFeed(resp.Body)
}

func ParseFeed(r io.Reader) ([]Video, error) {
	return parseFeed(r)
}

func parseFeed(r io.Reader) ([]Video, error) {
	var parsed feed
	if err := xml.NewDecoder(r).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}

	videos := make([]Video, 0, len(parsed.Entries))
	for _, entry := range parsed.Entries {
		if entry.VideoID == "" {
			continue
		}
		videos = append(videos, Video{
			ID:          entry.VideoID,
			Title:       entry.Title,
			URL:         entryURL(entry),
			Thumbnail:   entry.MediaGroup.Thumbnail.URL,
			Description: entry.MediaGroup.Description,
		})
	}
	return videos, nil
}

func entryURL(entry feedEntry) string {
	for _, link := range entry.Links {
		if link.Rel == "alternate" && link.Href != "" {
			return link.Href
		}
	}
	return "https://www.youtube.com/watch?v=" + entry.VideoID
}
