package reminder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/disgoorg/snowflake/v2"
)

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create reminders dir: %w", err)
	}

	store := &Store{path: path}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := store.write(fileData{Reminders: []Reminder{}}); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Add(userID, channelID snowflake.ID, event string, dueAt time.Time, originalMessage string) (Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readLocked()
	if err != nil {
		return Reminder{}, err
	}

	now := time.Now().UTC()
	item := Reminder{
		ID:              uuid.NewString(),
		UserID:          userID,
		ChannelID:       channelID,
		Event:           event,
		DueAt:           dueAt.UTC(),
		CreatedAt:       now,
		OriginalMessage: originalMessage,
	}
	data.Reminders = append(data.Reminders, item)

	if err := s.writeLocked(data); err != nil {
		return Reminder{}, err
	}
	return item, nil
}

func (s *Store) Due(before time.Time) ([]Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readLocked()
	if err != nil {
		return nil, err
	}

	before = before.UTC()
	var due []Reminder
	for _, item := range data.Reminders {
		if !item.DueAt.After(before) {
			due = append(due, item)
		}
	}
	return due, nil
}

func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readLocked()
	if err != nil {
		return err
	}

	filtered := data.Reminders[:0]
	for _, item := range data.Reminders {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	data.Reminders = filtered
	return s.writeLocked(data)
}

func (s *Store) RemoveAllForUser(userID snowflake.ID) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readLocked()
	if err != nil {
		return 0, err
	}

	filtered := data.Reminders[:0]
	removed := 0
	for _, item := range data.Reminders {
		if item.UserID == userID {
			removed++
			continue
		}
		filtered = append(filtered, item)
	}
	data.Reminders = filtered
	if err := s.writeLocked(data); err != nil {
		return 0, err
	}
	return removed, nil
}

func (s *Store) IncrementSendAttempts(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readLocked()
	if err != nil {
		return err
	}

	for i := range data.Reminders {
		if data.Reminders[i].ID == id {
			data.Reminders[i].SendAttempts++
			return s.writeLocked(data)
		}
	}
	return fmt.Errorf("reminder %q not found", id)
}

func (s *Store) ListForUser(userID snowflake.ID) ([]Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readLocked()
	if err != nil {
		return nil, err
	}

	var items []Reminder
	for _, item := range data.Reminders {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Store) readLocked() (fileData, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fileData{}, fmt.Errorf("read reminders: %w", err)
	}

	var parsed fileData
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fileData{}, fmt.Errorf("decode reminders: %w", err)
	}
	if parsed.Reminders == nil {
		parsed.Reminders = []Reminder{}
	}
	return parsed, nil
}

func (s *Store) write(data fileData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(data)
}

func (s *Store) writeLocked(data fileData) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal reminders: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, encoded, 0o600); err != nil {
		return fmt.Errorf("write reminders temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename reminders: %w", err)
	}
	return nil
}
