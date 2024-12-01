package metric

import (
	"testing"
	"time"
)

func BenchmarkRPS_Add(b *testing.B) {
	var rps RPS

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rps.Add(1)
	}
}

func BenchmarkRPS_Cal(b *testing.B) {
	var rps RPS

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rps.Add(1000)
		_ = rps.Calc(time.Second)
	}
}

func BenchmarkRPSHistory_AvgRPS(b *testing.B) {
	var hist rpsHistory

	it := time.Now().Add(-time.Hour)
	for i := 0; i < int(time.Hour/time.Second); i++ {
		hist.push(float64(i), it)
		it = it.Add(time.Second)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = hist.avgRPS(time.Hour)
	}
}
