package limit_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/limit"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

func BenchmarkTokenBucket_Allow(b *testing.B) {
	limiter := limit.NewTokenBucket(1000, 100)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = limiter.Allow(time.Now(), 1)
	}
}

func BenchmarkTokenBucket_ConcurrentAllow(b *testing.B) {
	limiter := limit.NewTokenBucket(1000, 100)
	var counter atomic.Int64

	var wg sync.WaitGroup
	f := func(n int64) {
		defer wg.Done()
		for {
			_ = limiter.Allow(time.Now(), 1)
			c := counter.Add(1)
			if c >= n {
				return
			}
		}
	}
	b.ResetTimer()

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go f(int64(b.N))
	}
	wg.Wait()
}

func BenchmarkTokenBucketLimiter_ToLimiterPB(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = limit.NewTokenBucketLimiter(1000, 100).ToLimiterPB()
	}
}

func BenchmarkTokenBucketLimiter_FromResPB(b *testing.B) {
	limiter := limit.NewTokenBucketLimiter(1000, 100)
	resPB := &pb.Resource{
		Name:    "test",
		NodeId:  1,
		Limiter: &pb.Resource_TokenBucket{TokenBucket: limiter.ToLimiterPB().(*pb.TokenBucketLimiter)},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = limit.NewTokenBucketEmptyLimiter().FromResPB(resPB)
	}
}
