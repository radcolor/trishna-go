package youtube

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const defaultStatePath = "data/youtube-state.json"

type State struct {
	path     string
	channels map[string]string
	mu       sync.Mutex
}

func LoadState(path string) (*State, error) {
	if path == "" {
		path = defaultStatePath
	}

	state := &State{
		path:     path,
		channels: make(map[string]string),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var stored struct {
		Channels map[string]string `json:"channels"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if stored.Channels != nil {
		state.channels = stored.Channels
	}
	return state, nil
}

func (s *State) LastSeen(channelID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.channels[channelID]
}

func (s *State) SetLastSeen(channelID, videoID string) error {
	s.mu.Lock()
	s.channels[channelID] = videoID
	s.mu.Unlock()
	return s.save()
}

func (s *State) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	payload, err := json.MarshalIndent(struct {
		Channels map[string]string `json:"channels"`
	}{Channels: s.channels}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(s.path, payload, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}
