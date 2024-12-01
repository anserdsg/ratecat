package limit

import (
	"testing"
)

func TestLeakyBucket_Test(t *testing.T) {
	t.Parallel()
	testcases := buildTestCases(t, []*testCase{
		{
			ctor:       func() rateLimiter { return NewLeakyBucket(1000, 10) },
			n:          1,
			goRoutines: 1,
		},
		{
			ctor:       func() rateLimiter { return NewLeakyBucket(5000, 10) },
			n:          1,
			goRoutines: 1,
		},
		{
			ctor:       func() rateLimiter { return NewLeakyBucket(1000, 10) },
			n:          1,
			goRoutines: 5,
		},
	})

	runTestCaseApply(t, testcases)
}

func TestLeakyBucket_Take(t *testing.T) {
	t.Parallel()
	testcases := buildTestCases(t, []*testCase{
		{
			ctor:       func() rateLimiter { return NewLeakyBucket(1000, 10) },
			n:          1,
			goRoutines: 1,
		},
		{
			ctor:       func() rateLimiter { return NewLeakyBucket(5000, 10) },
			n:          1,
			goRoutines: 1,
		},
		{
			ctor:       func() rateLimiter { return NewLeakyBucket(1000, 10) },
			n:          1,
			goRoutines: 5,
		},
	})

	runTestCaseTake(t, testcases)
}
