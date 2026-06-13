package runtime

import (
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"github.com/radcolor/trishna-go/internal/buildinfo"
)

type State struct {
	startedAt time.Time
}

func NewState() *State {
	return &State{}
}

func (s *State) MarkReady() {
	s.startedAt = time.Now()
}

func (s *State) StartedAt() time.Time {
	return s.startedAt
}

type BotSnapshot struct {
	Ready             bool
	Uptime            time.Duration
	Version           string
	Commit            string
	BuiltAt           string
	GoVersion         string
	Goroutines        int
	HeapInUse         uint64
	ProcessRSS        uint64
	ProcessCPUPercent float64
}

func (s *State) BotSnapshot() BotSnapshot {
	snap := BotSnapshot{
		Version:    buildinfo.Version,
		Commit:     buildinfo.Commit,
		BuiltAt:    buildinfo.BuiltAt,
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
	}

	if s.startedAt.IsZero() {
		return snap
	}

	snap.Ready = true
	snap.Uptime = time.Since(s.startedAt)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	snap.HeapInUse = mem.HeapInuse

	if proc, err := process.NewProcess(int32(os.Getpid())); err == nil {
		if rss, err := proc.MemoryInfo(); err == nil && rss != nil {
			snap.ProcessRSS = rss.RSS
		}
		if cpu, err := proc.CPUPercent(); err == nil {
			snap.ProcessCPUPercent = cpu
		}
	}

	return snap
}
