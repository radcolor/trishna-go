package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/radcolor/trishna-go/internal/runtime"
)

const (
	DefaultPath    = "data/shawnb/heartbeat.json"
	serviceName    = "heartbeat"
	refreshPeriod  = 10 * time.Second
	startupRefresh = 3 * time.Second
)

type Snapshot struct {
	Ready             bool      `json:"ready"`
	UpdatedAt         time.Time `json:"updated_at"`
	Bot               string    `json:"bot"`
	Model             string    `json:"model,omitempty"`
	UptimeSec         float64   `json:"uptime_sec,omitempty"`
	Goroutines        int       `json:"goroutines,omitempty"`
	ProcessRSS        uint64    `json:"process_rss,omitempty"`
	ProcessCPUPercent float64   `json:"process_cpu_percent,omitempty"`
}

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
		return nil, fmt.Errorf("create heartbeat dir: %w", err)
	}
	return &Store{path: path}, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Write(snapshot Snapshot) error {
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now().UTC()
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal heartbeat: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write heartbeat temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename heartbeat: %w", err)
	}
	return nil
}

func Read(path string) (Snapshot, error) {
	if path == "" {
		path = DefaultPath
	}
	path = filepath.Clean(path)

	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode heartbeat: %w", err)
	}
	return snapshot, nil
}

type Service struct {
	store   *Store
	runtime *runtime.State
	bot     string
	model   string
}

func NewService(store *Store, runtimeState *runtime.State, bot, model string) *Service {
	return &Service{store: store, runtime: runtimeState, bot: bot, model: model}
}

func (s *Service) Name() string {
	return serviceName
}

func (s *Service) Run(ctx context.Context) error {
	write := func(ready bool) error {
		snap := Snapshot{Ready: ready, Bot: s.bot, Model: s.model}
		if ready && s.runtime != nil {
			bot := s.runtime.BotSnapshot()
			snap.UptimeSec = bot.Uptime.Seconds()
			snap.Goroutines = bot.Goroutines
			snap.ProcessRSS = bot.ProcessRSS
			snap.ProcessCPUPercent = bot.ProcessCPUPercent
		}
		return s.store.Write(snap)
	}

	ready := s.runtime != nil && s.runtime.BotSnapshot().Ready
	if err := write(ready); err != nil {
		return err
	}

	ticker := time.NewTicker(refreshPeriod)
	defer ticker.Stop()

	startup := time.NewTimer(startupRefresh)
	defer startup.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = write(false)
			return nil
		case <-startup.C:
			ready := s.runtime != nil && s.runtime.BotSnapshot().Ready
			if err := write(ready); err != nil {
				return err
			}
		case <-ticker.C:
			ready := s.runtime != nil && s.runtime.BotSnapshot().Ready
			if err := write(ready); err != nil {
				return err
			}
		}
	}
}
