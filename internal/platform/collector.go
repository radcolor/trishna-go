package platform

import "context"

type Collector interface {
	Snapshot(ctx context.Context) (HostSnapshot, error)
}

func NewCollector() Collector {
	return newGopsutilCollector()
}
