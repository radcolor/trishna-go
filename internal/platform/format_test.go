package platform

import (
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	if got := FormatBytes(1024); got != "1.0 KB" {
		t.Fatalf("format bytes = %q", got)
	}
}

func TestFormatDuration(t *testing.T) {
	if got := FormatDuration(26*time.Hour); got != "1d 2h" {
		t.Fatalf("format duration = %q", got)
	}
}
