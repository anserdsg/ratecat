package rate_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	xrate "golang.org/x/time/rate"
)

func TestRate_Allow(t *testing.T) {
	require := require.New(t)

	var rate xrate.Limit = 1000
	var burst int = 10
	b := xrate.NewLimiter(rate, burst)
	require.NotNil(b)
	for i := 0; i < burst; i++ {
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

func TestRate_Reserve(t *testing.T) {
	require := require.New(t)

	var rate xrate.Limit = 1000
	var burst int = 10
	b := xrate.NewLimiter(rate, burst)
	require.NotNil(b)

	var allowCount int64 = 0

	start := time.Now()
	loopCount := int(rate) * 5
	for i := 0; i < loopCount; i++ {
		r := b.Reserve()
		allowCount++
		delay := r.Delay()
		time.Sleep(delay)
	}

	elapsed := time.Since(start)
	rps := float64((allowCount * int64(time.Second)) / elapsed.Nanoseconds())
	// check if the delta of rps is within 1%
	require.InDelta(float64(rate), rps, float64(rate)*0.01)
}
