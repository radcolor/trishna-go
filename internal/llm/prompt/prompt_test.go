package prompt

import (
	"strings"
	"testing"
)

func TestWrapUserContent(t *testing.T) {
	got := WrapUserContent("hello")
	if !strings.Contains(got, "BEGIN_USER_MESSAGE") || !strings.Contains(got, "hello") {
		t.Fatalf("wrap = %q", got)
	}
}

func TestTruncateInput(t *testing.T) {
	long := strings.Repeat("a", MaxUserInputRunes+10)
	got := TruncateInput(long)
	if utf8Count(got) > MaxUserInputRunes+3 {
		t.Fatalf("len = %d", utf8Count(got))
	}
}

func TestSanitizeDiscordOutput(t *testing.T) {
	got := SanitizeDiscordOutput("ping @everyone and @here")
	if strings.Contains(got, "@everyone") || strings.Contains(got, "@here") {
		t.Fatalf("got = %q", got)
	}
}

func utf8Count(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
