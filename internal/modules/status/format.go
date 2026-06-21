package status

import (
	"fmt"
	stdhtml "html"
	"strings"
	"time"

	"github.com/radcolor/trishna-go/internal/buildinfo"
	"github.com/radcolor/trishna-go/internal/llm/ollama"
	"github.com/radcolor/trishna-go/internal/platform"
	"github.com/radcolor/trishna-go/internal/runtime"
	"github.com/radcolor/trishna-go/internal/shawnb/monitor"
)

type Report struct {
	TrishnaBot      runtime.BotSnapshot
	TrishnaServices []runtime.ServiceHealth
	Shawnb          monitor.Status
	Ollama          ollama.Status
	Host            platform.HostSnapshot
	HostErr         error
}

func BuildMessage(r Report) string {
	var b strings.Builder
	b.WriteString("**Trishna Bot Status**\n```\n")
	b.WriteString(formatTrishnaSection(r.TrishnaBot, r.TrishnaServices))
	b.WriteString("```\n**shawnb Bot Status**\n```\n")
	b.WriteString(formatShawnbSection(r.Shawnb))
	b.WriteString("```\n**Mac Server Status**\n```\n")
	b.WriteString(formatServerSection(r.Host, r.HostErr, r.Ollama))
	b.WriteString("```")
	return b.String()
}

func BuildHTMLMessage(r Report) string {
	var b strings.Builder
	writeHTMLSection(&b, "Trishna Bot Status", formatTrishnaSection(r.TrishnaBot, r.TrishnaServices))
	writeHTMLSection(&b, "shawnb Bot Status", formatShawnbSection(r.Shawnb))
	writeHTMLSection(&b, "Mac Server Status", formatServerSection(r.Host, r.HostErr, r.Ollama))
	return b.String()
}

func writeHTMLSection(b *strings.Builder, title, body string) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString("<b>")
	b.WriteString(stdhtml.EscapeString(title))
	b.WriteString("</b>\n<pre>")
	b.WriteString(stdhtml.EscapeString(body))
	b.WriteString("</pre>")
}

func formatTrishnaSection(bot runtime.BotSnapshot, services []runtime.ServiceHealth) string {
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

func formatShawnbSection(shawnb monitor.Status) string {
	status := "Offline"
	if shawnb.Running {
		status = "Online"
	}

	model := shawnb.Model
	if model == "" {
		model = "n/a"
	}

	lines := []string{
		line("Status", status),
		line("Discord", shawnb.Detail),
		line("Model", model),
	}
	if shawnb.Uptime > 0 {
		lines = append(lines, line("Uptime", platform.FormatDuration(shawnb.Uptime)))
	}
	if shawnb.Goroutines > 0 {
		lines = append(lines, line("Goroutines", fmt.Sprintf("%d", shawnb.Goroutines)))
	}
	if shawnb.ProcessCPUPercent > 0 || shawnb.ProcessRSS > 0 {
		lines = append(lines, line("Process CPU", formatShawnbCPU(shawnb.ProcessCPUPercent)))
		lines = append(lines, line("Process RAM", formatShawnbRAM(shawnb.ProcessRSS)))
	}
	if shawnb.LastOK != nil {
		lines = append(lines, line("Heartbeat", platform.FormatDuration(time.Since(*shawnb.LastOK))+" ago"))
	}
	if shawnb.LastError != "" {
		lines = append(lines, line("Error", shawnb.LastError))
	}
	return strings.Join(lines, "\n")
}

func formatShawnbCPU(cpu float64) string {
	if cpu <= 0 {
		return "n/a"
	}
	return platform.FormatPercent(cpu)
}

func formatShawnbRAM(rss uint64) string {
	if rss == 0 {
		return "n/a"
	}
	return platform.FormatBytes(rss)
}

func formatServerSection(host platform.HostSnapshot, hostErr error, ollamaStatus ollama.Status) string {
	var sections []string

	if hostErr != nil {
		sections = append(sections, fmt.Sprintf("error: %v", hostErr))
	} else if !host.Available {
		msg := host.Unavailable
		if msg == "" {
			msg = "host metrics unavailable"
		}
		sections = append(sections, msg)
	} else {
		sections = append(sections, strings.Join([]string{
			line("Host", host.Hostname),
			line("Uptime", platform.FormatDuration(host.Uptime)),
			line("System", formatSystem(host)),
			line("CPU", fmt.Sprintf("%d cores · %s", host.CPUCores, platform.FormatPercent(host.CPUPercent))),
			line("Memory", formatMemory(host)),
			line("GPU", formatGPU(host)),
			line("Load Avg", formatLoad(host)),
			line("Storage", formatDisks(host.Disks)),
		}, "\n"))
	}

	sections = append(sections, formatOllamaSection(ollamaStatus))
	return strings.Join(sections, "\n\n")
}

func formatOllamaSection(status ollama.Status) string {
	ollamaState := "stopped"
	if status.Available {
		ollamaState = "running"
	}

	configured := status.ConfiguredModel
	if configured == "" {
		configured = "n/a"
	}

	lines := []string{
		line("Ollama", ollamaState),
		line("Configured", configured),
	}
	if status.Available {
		if status.Version != "" {
			lines = append(lines, line("Version", status.Version))
		}
		lines = append(lines, line("Loaded", formatOllamaLoadedModels(status.LoadedModels)))
	}
	if status.Error != "" {
		lines = append(lines, line("Error", status.Error))
	}
	return strings.Join(lines, "\n")
}

func formatOllamaLoadedModels(models []ollama.LoadedModel) string {
	if len(models) == 0 {
		return "none (will load on first chat)"
	}

	parts := make([]string, 0, len(models))
	for _, model := range models {
		part := model.Name
		if model.SizeVRAM > 0 {
			part += " · " + platform.FormatBytes(uint64(model.SizeVRAM))
		}
		if model.ParameterSize != "" {
			part += " · " + model.ParameterSize
		}
		if model.Quantization != "" {
			part += " " + model.Quantization
		}
		if model.ContextLength > 0 {
			part += " · ctx " + fmt.Sprintf("%d", model.ContextLength)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n              ")
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
