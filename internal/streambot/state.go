package streambot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const DefaultStatePath = "data/streambot/state.json"

type State struct {
	CurrentGame string `json:"current_game"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultStatePath
	}
	return &Store{path: path}
}

func (s *Store) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadLocked()
}

func (s *Store) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(s.path, body, 0o600)
}

func (s *Store) Update(fn func(State) State) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadLocked()
	if err != nil {
		return State{}, err
	}
	state = fn(state)

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return State{}, err
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return State{}, err
	}
	body = append(body, '\n')
	if err := os.WriteFile(s.path, body, 0o600); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Store) loadLocked() (State, error) {
	body, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}

	var state State
	if err := json.Unmarshal(body, &state); err != nil {
		return State{}, err
	}
	state.CurrentGame = NormalizeGame(state.CurrentGame)
	return state, nil
}
