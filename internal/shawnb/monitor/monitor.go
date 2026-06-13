package monitor

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/radcolor/trishna-go/internal/runtime"
	"github.com/radcolor/trishna-go/internal/shawnb/heartbeat"
)

const (
	serviceName    = "shawnb"
	staleAfter     = 120 * time.Second
	detailDisabled = "not configured"
)

type Status struct {
	Running           bool
	Detail            string
	Model             string
	LastOK            *time.Time
	LastError         string
	Uptime            time.Duration
	Goroutines        int
	ProcessRSS        uint64
	ProcessCPUPercent float64
}

type Monitor struct {
	path string
}

func New(path string) Monitor {
	if strings.TrimSpace(path) == "" {
		path = heartbeat.DefaultPath
	}
	return Monitor{path: path}
}

func (m Monitor) Status() Status {
	snapshot, err := heartbeat.Read(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Status{
				Running: false,
				Detail:  "offline (no heartbeat)",
			}
		}
		return Status{
			Running:   false,
			Detail:    "offline",
			LastError: err.Error(),
		}
	}

	age := time.Since(snapshot.UpdatedAt)
	lastOK := snapshot.UpdatedAt
	status := Status{
		Model:             snapshot.Model,
		LastOK:            &lastOK,
		Uptime:            time.Duration(snapshot.UptimeSec * float64(time.Second)),
		Goroutines:        snapshot.Goroutines,
		ProcessRSS:        snapshot.ProcessRSS,
		ProcessCPUPercent: snapshot.ProcessCPUPercent,
	}

	if !snapshot.Ready {
		status.Running = false
		status.Detail = "offline (shutting down)"
		return status
	}
	if age > staleAfter {
		status.Running = false
		status.Detail = "offline (stale heartbeat)"
		return status
	}

	status.Running = true
	status.Detail = "discord connected"
	return status
}

func (m Monitor) Health() runtime.ServiceHealth {
	status := m.Status()
	health := runtime.ServiceHealth{
		Name:      serviceName,
		Running:   status.Running,
		Detail:    status.Detail,
		LastOK:    status.LastOK,
		LastError: status.LastError,
	}
	if status.Running && status.Model != "" {
		health.Detail += " · " + status.Model
	}
	return health
}
