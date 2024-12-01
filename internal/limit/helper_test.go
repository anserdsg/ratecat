package limit

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testCase struct {
	name       string
	ctor       func() rateLimiter
	target     *testLimiter
	n          uint32
	goRoutines int
}

func buildTestCases(t *testing.T, casecases []*testCase) []*testCase {
	for _, tc := range casecases {
		tc.target = &testLimiter{
			t:       t,
			limiter: tc.ctor(),
		}
		tc.name = fmt.Sprintf("%s:n=%d_goroutine=%d", tc.target.limiter.String(), tc.n, tc.goRoutines)
	}
	return casecases
}

type rateLimiter interface {
	Allow(t time.Time, n uint32) bool
	Take(t time.Time, n uint32) (time.Duration, error)
	Rate() float64
	Burst() uint32
	String() string
}

type testLimiter struct {
	t       *testing.T
	limiter rateLimiter
	wg      sync.WaitGroup
	count   atomic.Int64
}

func runTestCaseApply(t *testing.T, testCases []*testCase) {
	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.target.testApply(tc.n, tc.goRoutines)
		})
	}
}

func runTestCaseTake(t *testing.T, testCases []*testCase) {
	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.target.testTake(tc.n, tc.goRoutines)
		})
	}
}

func (tl *testLimiter) testApply(n uint32, goRoutines int) {
	require := require.New(tl.t)

	require.NotNil(tl.limiter)

	limiter := tl.limiter
	rate := limiter.Rate()
	burst := limiter.Burst()

	if burst > 0 {
		for i := uint32(0); i < burst/n; i++ {
			ok := limiter.Allow(time.Now(), n)
			require.Truef(ok, "should be within burst range: i=%d, loop=%d, n=%d", i, burst/n, n)
		}

		time.Sleep((time.Second / time.Duration(rate)) + 1)
	}

	fn := func(allowNum uint32) {
		defer tl.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ok := limiter.Allow(time.Now(), allowNum)
				if ok {
					tl.count.Add(1)
				}
			}
		}
	}

	start := time.Now()
	tl.count.Store(0)
	for i := 0; i < goRoutines; i++ {
		tl.wg.Add(1)
		go fn(n)
	}
	tl.wg.Wait()

	rps := calRPS(tl.count.Load(), start, time.Now())
	// check if the delta of rps is within 5%
	require.InDelta(rate/float64(n), rps, rate*0.05)
}

func (tl *testLimiter) testTake(n uint32, goRoutines int) {
	require := require.New(tl.t)

	rate := tl.limiter.Rate()
	b := tl.limiter
	require.NotNil(b)

	fn := func(allowNum uint32) {
		defer tl.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				delay, err := tl.limiter.Take(time.Now(), allowNum)
				require.NoError(err)
				tl.count.Add(1)
				time.Sleep(delay)
			}
		}
	}

	start := time.Now()
	tl.count.Store(0)
	for i := 0; i < goRoutines; i++ {
		tl.wg.Add(1)
		go fn(n)
	}
	tl.wg.Wait()

	rps := calRPS(tl.count.Load(), start, time.Now())
	// check if the delta of rps is within 5%
	require.InDelta(rate/float64(n), rps, rate*0.05)
}

func calRPS(count int64, start time.Time, end time.Time) float64 {
	elapsed := end.Sub(start)
	return float64((count * int64(time.Second))) / float64(elapsed.Nanoseconds())
}
