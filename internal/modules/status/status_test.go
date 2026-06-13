package status

import (
	"strings"
	"testing"
	"time"

	"github.com/radcolor/trishna-go/internal/platform"
	"github.com/radcolor/trishna-go/internal/runtime"
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
	content := BuildMessage(
		runtime.BotSnapshot{
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
		platform.HostSnapshot{
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
		nil,
		[]runtime.ServiceHealth{{
			Name:    "youtube",
			Running: true,
			Detail:  "1 channel(s)",
			LastOK:  &now,
		}},
	)

	if !strings.Contains(content, "**Trishna**") {
		t.Fatalf("missing trishna header: %q", content)
	}
	if !strings.Contains(content, "**Mac Mini**") {
		t.Fatalf("missing mac mini header: %q", content)
	}
	if !strings.Contains(content, "Status:      Online") {
		t.Fatalf("missing bot status: %q", content)
	}
	if !strings.Contains(content, "Host:        mac-mini") {
		t.Fatalf("missing host name: %q", content)
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
