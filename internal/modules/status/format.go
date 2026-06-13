package status

import (
	"fmt"
	"strings"
	"time"

	"github.com/radcolor/trishna-go/internal/buildinfo"
	"github.com/radcolor/trishna-go/internal/platform"
	"github.com/radcolor/trishna-go/internal/runtime"
)

func BuildMessage(
	bot runtime.BotSnapshot,
	host platform.HostSnapshot,
	hostErr error,
	services []runtime.ServiceHealth,
) string {
	var b strings.Builder
	b.WriteString("**Trishna Bot Status**\n```\n")
	b.WriteString(formatBotSection(bot, services))
	b.WriteString("```\n**Mac Server Status**\n```\n")
	b.WriteString(formatHostSection(host, hostErr))
	b.WriteString("```")
	return b.String()
}

func formatBotSection(bot runtime.BotSnapshot, services []runtime.ServiceHealth) string {
	status := "Starting"
	if bot.Ready {
		status = "Online"
	}

	lines := []string{
		line("Status", status),
		line("Uptime", formatUptime(bot.Uptime, bot.Ready)),
		line("Version", buildinfo.Label()),
		line("Built", bot.BuiltAt),
		line("Go", bot.GoVersion),
		line("Goroutines", fmt.Sprintf("%d", bot.Goroutines)),
		line("Process CPU", platform.FormatPercent(bot.ProcessCPUPercent)),
		line("Process RAM", formatProcessRAM(bot)),
		line("Services", formatServices(services)),
	}
	return strings.Join(lines, "\n")
}

func formatHostSection(host platform.HostSnapshot, hostErr error) string {
	if hostErr != nil {
		return fmt.Sprintf("error: %v", hostErr)
	}
	if !host.Available {
		msg := host.Unavailable
		if msg == "" {
			msg = "host metrics unavailable"
		}
		return msg
	}

	lines := []string{
		line("Host", host.Hostname),
		line("Uptime", platform.FormatDuration(host.Uptime)),
		line("System", formatSystem(host)),
		line("CPU", fmt.Sprintf("%d cores · %s", host.CPUCores, platform.FormatPercent(host.CPUPercent))),
		line("Memory", formatMemory(host)),
		line("GPU", formatGPU(host)),
		line("Load Avg", formatLoad(host)),
		line("Storage", formatDisks(host.Disks)),
	}
	return strings.Join(lines, "\n")
}

func line(label, value string) string {
	return fmt.Sprintf("%-12s %s", label+":", value)
}

func formatUptime(d time.Duration, ready bool) string {
	if !ready {
		return "not ready"
	}
	return platform.FormatDuration(d)
}

func formatProcessRAM(bot runtime.BotSnapshot) string {
	if bot.ProcessRSS > 0 {
		return platform.FormatBytes(bot.ProcessRSS)
	}
	if bot.HeapInUse > 0 {
		return platform.FormatBytes(bot.HeapInUse) + " heap"
	}
	return "n/a"
}

func formatServices(services []runtime.ServiceHealth) string {
	if len(services) == 0 {
		return "none"
	}

	lines := make([]string, 0, len(services))
	for _, svc := range services {
		state := "stopped"
		if svc.Running {
			state = "running"
		}
		line := fmt.Sprintf("%s · %s", svc.Name, state)
		if svc.Detail != "" {
			line += " · " + svc.Detail
		}
		if svc.LastOK != nil {
			line += fmt.Sprintf(" · last ok %s ago", platform.FormatDuration(time.Since(*svc.LastOK)))
		}
		if svc.LastError != "" {
			line += " · error: " + svc.LastError
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n              ")
}

func formatSystem(host platform.HostSnapshot) string {
	parts := []string{host.Model}
	if host.PlatformVer != "" {
		parts = append(parts, host.Platform+" "+host.PlatformVer)
	}
	parts = append(parts, host.Arch)
	return strings.Join(parts, " · ")
}

func formatMemory(host platform.HostSnapshot) string {
	return fmt.Sprintf(
		"%s / %s (%s)",
		platform.FormatBytes(host.MemUsed),
		platform.FormatBytes(host.MemTotal),
		platform.FormatPercent(host.MemPercent),
	)
}

func formatGPU(host platform.HostSnapshot) string {
	if host.GPUName == "" || host.GPUName == "N/A" {
		return "usage N/A"
	}
	return host.GPUName + " (usage N/A)"
}

func formatLoad(host platform.HostSnapshot) string {
	if host.Load1 == 0 && host.Load5 == 0 && host.Load15 == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.2f · %.2f · %.2f", host.Load1, host.Load5, host.Load15)
}

func formatDisks(disks []platform.DiskUsage) string {
	if len(disks) == 0 {
		return "n/a"
	}
	lines := make([]string, 0, len(disks))
	for _, d := range disks {
		lines = append(lines, fmt.Sprintf(
			"%s · %s free / %s (%s used)",
			d.Path,
			platform.FormatBytes(d.Free),
			platform.FormatBytes(d.Total),
			platform.FormatPercent(d.UsedPercent),
		))
	}
	return strings.Join(lines, "\n              ")
}
