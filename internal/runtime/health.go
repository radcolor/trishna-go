package runtime

import "time"

type HealthReporter interface {
	Health() ServiceHealth
}

type ServiceHealth struct {
	Name      string
	Running   bool
	Detail    string
	LastOK    *time.Time
	LastError string
}
