package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu   sync.Mutex
	hits map[int64][]time.Time
}

func New() *Limiter {
	return &Limiter{hits: map[int64][]time.Time{}}
}

// AllowRPM 简单滑动窗口（每分钟请求数）
func (l *Limiter) AllowRPM(keyID int64, rpm int) bool {
	if rpm <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	windowStart := now.Add(-time.Minute)
	arr := l.hits[keyID]
	kept := arr[:0]
	for _, t := range arr {
		if t.After(windowStart) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rpm {
		l.hits[keyID] = kept
		return false
	}
	kept = append(kept, now)
	l.hits[keyID] = kept
	return true
}
