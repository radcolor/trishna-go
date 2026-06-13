package ratelimit

import (
	"testing"
	"time"
)

func TestCooldown(t *testing.T) {
	c := NewCooldown(time.Hour)
	if !c.Allow("user:1") {
		t.Fatal("expected first allow")
	}
	if c.Allow("user:1") {
		t.Fatal("expected second deny")
	}
	if !c.Allow("user:2") {
		t.Fatal("expected different key allow")
	}
}
