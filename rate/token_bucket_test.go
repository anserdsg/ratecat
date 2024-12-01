package rate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTokenBucket_Allow(t *testing.T) {
	require := require.New(t)

	var rate float64 = 1000
	var burst uint32 = 10
	b := NewTokenBucket(rate, burst)
	require.NotNil(b)
	for i := 0; i < int(burst); i++ {
		ok := b.Allow()
		require.True(ok)
	}
	ok := b.AllowN(time.Now(), 10)
	require.False(ok)

	time.Sleep(100 * time.Millisecond)

	var allowCount int64 = 0

	start := time.Now()
	loopCount := 20000000
	for i := 0; i < loopCount; i++ {
		ok := b.Allow()
		if ok {
			allowCount++
		}
	}

	elapsed := time.Since(start)
	rps := float64((allowCount * int64(time.Second)) / elapsed.Nanoseconds())
	// check if the delta of rps is within 1%
	require.InDelta(float64(rate), rps, float64(rate)*0.01)
}
