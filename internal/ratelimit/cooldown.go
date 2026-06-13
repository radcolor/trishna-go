package ratelimit

import (
	"sync"
	"time"
)

type Cooldown struct {
	interval time.Duration
	mu       sync.Mutex
	last     map[string]time.Time
}

func NewCooldown(interval time.Duration) *Cooldown {
	return &Cooldown{
		interval: interval,
		last:     make(map[string]time.Time),
	}
}

func (c *Cooldown) Allow(key string) bool {
	if c == nil || c.interval <= 0 {
		return true
	}

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	if prev, ok := c.last[key]; ok && now.Sub(prev) < c.interval {
		return false
	}
	c.last[key] = now
	return true
}
