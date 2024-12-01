package metric

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRPS_Calc(t *testing.T) {
	require := require.New(t)

	var rps RPS
	for i := 0; i < 10; i++ {
		rps.Add(1)
	}
	require.Equal(float64(10.0), rps.Calc(time.Second))
	require.Equal(float64(0.0), rps.Calc(time.Second))
	for i := 0; i < 10; i++ {
		rps.Add(1)
	}
	require.Equal(float64(5.0), rps.Calc(2*time.Second))
	require.Equal(float64(0.0), rps.Calc(time.Second))
}

func TestRPSHistory_AvgRPS(t *testing.T) {
	require := require.New(t)

	var hist rpsHistory
	it := time.Now().Add(-time.Hour)
	until := time.Hour / time.Second
	for i := 0; i < int(until); i++ {
		hist.push(float64(i), it)
		it = it.Add(time.Second)
	}

	durationTests := []time.Duration{
		time.Minute,
		10 * time.Minute,
		20 * time.Minute,
		25 * time.Minute,
		30 * time.Minute,
		59 * time.Minute,
		60 * time.Minute,
	}
	for _, dt := range durationTests {
		require.Equal(avgRPS(until, dt), hist.avgRPS(dt))
	}
	durationTests = []time.Duration{
		61 * time.Minute,
		100 * time.Minute,
		200 * time.Minute,
	}
	for _, dt := range durationTests {
		require.Equal(avgRPS(until, dt), hist.avgRPS(dt))
	}
}

func avgRPS(until time.Duration, due time.Duration) float64 {
	var sum float64
	var from time.Duration
	secs := due / time.Second
	if secs > until {
		secs = until
		from = until - secs
	} else {
		from = until - secs + 1
	}
	for i := from; i < until; i++ {
		sum += float64(i)
	}
	return sum / float64(until-from)
}
