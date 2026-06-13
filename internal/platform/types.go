package platform

import "time"

type HostSnapshot struct {
	Available     bool
	Unavailable   string
	Hostname      string
	OS            string
	Arch          string
	Platform      string
	PlatformVer   string
	KernelVersion string
	Model         string
	BootTime      time.Time
	Uptime        time.Duration
	CPUCores      int
	CPUPercent    float64
	MemTotal      uint64
	MemUsed       uint64
	MemPercent    float64
	Disks         []DiskUsage
	GPUName       string
	Load1         float64
	Load5         float64
	Load15        float64
}

type DiskUsage struct {
	Path       string
	Total      uint64
	Used       uint64
	Free       uint64
	UsedPercent float64
}
