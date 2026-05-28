package auth_test

import (
	"testing"
	"time"

	"github.com/darthsoup/goblinftp/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestThrottleNotThrottledInitially(t *testing.T) {
	th := auth.NewThrottle()
	assert.False(t, th.IsThrottled("user@example.com", 3))
}

func TestThrottleBlocksAfterMaxAttempts(t *testing.T) {
	th := auth.NewThrottle()
	key := "bad@example.com"
	cooldown := 5 * time.Second

	for i := 0; i < 3; i++ {
		th.Record(key, cooldown)
	}
	assert.True(t, th.IsThrottled(key, 3))
}

func TestThrottleResetClearsAttempts(t *testing.T) {
	th := auth.NewThrottle()
	key := "user@example.com"
	cooldown := 5 * time.Second

	for i := 0; i < 3; i++ {
		th.Record(key, cooldown)
	}
	th.Reset(key)
	assert.False(t, th.IsThrottled(key, 3))
}

func TestThrottleCooldownExpires(t *testing.T) {
	th := auth.NewThrottle()
	key := "temp@example.com"
	cooldown := 50 * time.Millisecond

	for i := 0; i < 3; i++ {
		th.Record(key, cooldown)
	}
	assert.True(t, th.IsThrottled(key, 3))

	time.Sleep(100 * time.Millisecond)
	assert.False(t, th.IsThrottled(key, 3))
}

func TestThrottleIndependentKeys(t *testing.T) {
	th := auth.NewThrottle()
	cooldown := 5 * time.Second

	for i := 0; i < 3; i++ {
		th.Record("bad@example.com", cooldown)
	}
	assert.True(t, th.IsThrottled("bad@example.com", 3))
	assert.False(t, th.IsThrottled("good@example.com", 3))
}
