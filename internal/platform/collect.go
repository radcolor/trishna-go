package platform

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

type gopsutilCollector struct {
	cacheUntil time.Time
	cached     HostSnapshot
}

func newGopsutilCollector() Collector {
	return &gopsutilCollector{}
}

func (c *gopsutilCollector) Snapshot(ctx context.Context) (HostSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return HostSnapshot{}, err
	}

	now := time.Now()
	if !c.cacheUntil.IsZero() && now.Before(c.cacheUntil) {
		return c.cached, nil
	}

	snap, err := collectHostSnapshot()
	if err != nil {
		return HostSnapshot{}, err
	}

	c.cached = snap
	c.cacheUntil = now.Add(3 * time.Second)
	return snap, nil
}

func collectHostSnapshot() (HostSnapshot, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return HostSnapshot{}, err
	}

	hostInfo, err := host.Info()
	if err != nil {
		return HostSnapshot{Available: false, Unavailable: err.Error()}, nil
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return HostSnapshot{Available: false, Unavailable: err.Error()}, nil
	}

	cpuPercents, err := cpu.Percent(0, false)
	if err != nil {
		return HostSnapshot{Available: false, Unavailable: err.Error()}, nil
	}

	snap := HostSnapshot{
		Available:     true,
		Hostname:      hostname,
		OS:            hostInfo.OS,
		Arch:          runtime.GOARCH,
		Platform:      hostInfo.Platform,
		PlatformVer:   hostInfo.PlatformVersion,
		KernelVersion: hostInfo.KernelVersion,
		BootTime:      time.Unix(int64(hostInfo.BootTime), 0),
		Uptime:        time.Duration(hostInfo.Uptime) * time.Second,
		CPUCores:      runtime.NumCPU(),
		MemTotal:      memInfo.Total,
		MemUsed:       memInfo.Used,
		MemPercent:    memInfo.UsedPercent,
		GPUName:       gpuName(),
	}

	if model := hostModel(); model != "" {
		snap.Model = model
	} else if hostInfo.PlatformFamily != "" {
		snap.Model = hostInfo.PlatformFamily
	} else {
		snap.Model = hostInfo.Platform
	}

	if logical, err := cpu.Counts(true); err == nil && logical > 0 {
		snap.CPUCores = logical
	}

	if len(cpuPercents) > 0 {
		snap.CPUPercent = cpuPercents[0]
	}

	if avg, err := load.Avg(); err == nil && avg != nil {
		snap.Load1 = avg.Load1
		snap.Load5 = avg.Load5
		snap.Load15 = avg.Load15
	}

	for _, path := range diskPaths() {
		usage, err := disk.Usage(path)
		if err != nil {
			continue
		}
		snap.Disks = append(snap.Disks, DiskUsage{
			Path:        path,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: usage.UsedPercent,
		})
	}

	return snap, nil
}

func diskPaths() []string {
	paths := []string{"/"}
	if wd, err := os.Getwd(); err == nil && wd != "" && wd != "/" {
		paths = append(paths, wd)
	}
	return paths
}
