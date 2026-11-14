package telegram

import (
	"sync"
	"time"
)

type UserLimiter struct {
	mu          sync.Mutex
	lastRequest map[int64]time.Time
	cooldown    time.Duration
}

func NewUserLimiter(cooldown time.Duration) *UserLimiter {
	return &UserLimiter{
		lastRequest: make(map[int64]time.Time),
		cooldown:    cooldown,
	}
}

func (l *UserLimiter) Allow(userID int64) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	lastTime, exists := l.lastRequest[userID]
	if !exists {
		l.lastRequest[userID] = time.Now()
		return true, 0
	}

	timePassed := time.Since(lastTime)
	if timePassed < l.cooldown {
		timeLeft := l.cooldown - timePassed
		return false, timeLeft
	}

	l.lastRequest[userID] = time.Now()
	return true, 0
}
