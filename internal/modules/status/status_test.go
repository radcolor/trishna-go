package status

import (
	"strings"
	"testing"
	"time"

	"github.com/radcolor/trishna-go/internal/llm/ollama"
	"github.com/radcolor/trishna-go/internal/platform"
	"github.com/radcolor/trishna-go/internal/runtime"
	"github.com/radcolor/trishna-go/internal/shawnb/monitor"
)

func TestCommands(t *testing.T) {
	commands := New(Deps{}).Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d", len(commands))
	}
	if commands[0].CommandName() != CommandName {
		t.Fatalf("command name = %q", commands[0].CommandName())
	}
}

func TestBuildMessage(t *testing.T) {
	now := time.Now()
	report := Report{
		TrishnaBot: runtime.BotSnapshot{
			Ready:             true,
			Uptime:            2 * time.Hour,
			Version:           "v0.1.0",
			Commit:            "abc123",
			BuiltAt:           "2026-06-13T00:00:00Z",
			GoVersion:         "go1.26.4",
			Goroutines:        12,
			ProcessRSS:        40 * 1024 * 1024,
			ProcessCPUPercent: 0.5,
		},
		TrishnaServices: []runtime.ServiceHealth{{
			Name:    "youtube",
			Running: true,
			Detail:  "1 channel(s)",
			LastOK:  &now,
		}},
		Shawnb: monitor.Status{
			Running:           true,
			Detail:            "discord connected",
			Model:             "gemma4:e2b",
			LastOK:            &now,
			Uptime:            2 * time.Hour,
			Goroutines:        18,
			ProcessRSS:        32 * 1024 * 1024,
			ProcessCPUPercent: 0.3,
		},
		Ollama: ollama.Status{
			Available:       true,
			Version:         "0.30.8",
			ConfiguredModel: "gemma4:e2b",
			LoadedModels: []ollama.LoadedModel{{
				Name:          "gemma4:e2b",
				SizeVRAM:      1744809491,
				ParameterSize: "5.1B",
				Quantization:  "Q4_K_M",
				ContextLength: 4096,
			}},
		},
		Host: platform.HostSnapshot{
			Available:   true,
			Hostname:    "mac-mini",
			Model:       "Mac14,3",
			Platform:    "darwin",
			PlatformVer: "15.5",
			Arch:        "arm64",
			Uptime:      24 * time.Hour,
			CPUCores:    8,
			CPUPercent:  12.5,
			MemTotal:    16 * 1024 * 1024 * 1024,
			MemUsed:     8 * 1024 * 1024 * 1024,
			MemPercent:  50,
			GPUName:     "Apple M2",
			Load1:       1.2,
			Load5:       1.0,
			Load15:      0.8,
			Disks: []platform.DiskUsage{{
				Path:        "/",
				Total:       512 * 1024 * 1024 * 1024,
				Free:        200 * 1024 * 1024 * 1024,
				UsedPercent: 60,
			}},
		},
	}
	content := BuildMessage(report)

	if !strings.Contains(content, "**Trishna Bot Status**") {
		t.Fatalf("missing trishna header: %q", content)
	}
	if !strings.Contains(content, "**shawnb Bot Status**") {
		t.Fatalf("missing shawn header: %q", content)
	}
	if !strings.Contains(content, "**Mac Server Status**") {
		t.Fatalf("missing mac server header: %q", content)
	}
	if !strings.Contains(content, "Status:      Online") {
		t.Fatalf("missing bot status: %q", content)
	}
	if !strings.Contains(content, "Model:       gemma4:e2b") {
		t.Fatalf("missing shawn model: %q", content)
	}
	if !strings.Contains(content, "Process CPU: 0.3%") {
		t.Fatalf("missing shawn process cpu: %q", content)
	}
	if !strings.Contains(content, "Process RAM: 32.0 MB") {
		t.Fatalf("missing shawn process ram: %q", content)
	}
	if !strings.Contains(content, "Ollama:      running") {
		t.Fatalf("missing ollama status: %q", content)
	}
	if !strings.Contains(content, "Host:        mac-mini") {
		t.Fatalf("missing host name: %q", content)
	}

	htmlContent := BuildHTMLMessage(report)
	if !strings.Contains(htmlContent, "<b>Trishna Bot Status</b>") {
		t.Fatalf("missing html trishna header: %q", htmlContent)
	}
	if !strings.Contains(htmlContent, "<pre>Status:      Online") {
		t.Fatalf("missing html pre block: %q", htmlContent)
	}
	if strings.Contains(htmlContent, "**Trishna Bot Status**") || strings.Contains(htmlContent, "```") {
		t.Fatalf("html contains discord markdown: %q", htmlContent)
	}
}

func TestServicesAlignUnderValueColumn(t *testing.T) {
	content := formatTrishnaSection(runtime.BotSnapshot{Ready: true}, []runtime.ServiceHealth{
		{Name: "discord", Running: true, Detail: "connected"},
		{Name: "telegram", Running: true, Detail: "telegram mtproto via mtproxy"},
		{Name: "youtube", Running: true, Detail: "1 channel(s)"},
	})

	lines := strings.Split(content, "\n")
	var services []string
	for _, line := range lines {
		if strings.Contains(line, "discord · running") ||
			strings.Contains(line, "telegram · running") ||
			strings.Contains(line, "youtube · running") {
			services = append(services, line)
		}
	}
	if len(services) != 3 {
		t.Fatalf("service lines = %q", services)
	}

	firstColumn := strings.Index(services[0], "discord")
	for _, line := range services[1:] {
		column := strings.Index(line, strings.TrimSpace(line[:strings.Index(line, " · ")]))
		if column != firstColumn {
			t.Fatalf("service line %q starts at column %d, want %d", line, column, firstColumn)
		}
	}
}

func TestParseAllowlist(t *testing.T) {
	ids, err := ParseAllowlist(" 123 , 456 ")
	if err != nil {
		t.Fatalf("parse allowlist: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("allowlist len = %d", len(ids))
	}
}
