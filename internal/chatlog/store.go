package chatlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	TS        time.Time `json:"ts"`
	UserID    string    `json:"user_id,omitempty"`
	ChannelID string    `json:"channel_id,omitempty"`
	IsDM      bool      `json:"is_dm,omitempty"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
}

type Store struct {
	dir string
	mu  sync.Mutex
}

func NewStore(dir string) (*Store, error) {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" {
		dir = filepath.Join("data", "shawnb", "chats")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create chat log dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) Append(entry Entry) error {
	if entry.TS.IsZero() {
		entry.TS = time.Now().UTC()
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal chat log entry: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, entry.TS.UTC().Format("2006-01-02")+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open chat log: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write chat log: %w", err)
	}
	return nil
}
