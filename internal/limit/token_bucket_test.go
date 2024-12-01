package limit

import (
	"testing"
)

func TestTokenBucket_Test(t *testing.T) {
	t.Parallel()
	testcases := buildTestCases(t, []*testCase{
		{
			ctor:       func() rateLimiter { return NewTokenBucket(1000, 10) },
			n:          1,
			goRoutines: 1,
		},
		{
			ctor:       func() rateLimiter { return NewTokenBucket(1000, 5) },
			n:          3,
			goRoutines: 1,
		},
		{
			ctor:       func() rateLimiter { return NewTokenBucket(5000, 10) },
			n:          1,
			goRoutines: 1,
		},
		{
			ctor:       func() rateLimiter { return NewTokenBucket(1000, 10) },
			n:          1,
			goRoutines: 5,
		},
	})

	runTestCaseApply(t, testcases)
}

func TestTokenBucket_Take(t *testing.T) {
	t.Parallel()
	testcases := buildTestCases(t, []*testCase{
		{
			ctor:       func() rateLimiter { return NewTokenBucket(1000, 10) },
			n:          1,
			goRoutines: 1,
		},
		{
			ctor:       func() rateLimiter { return NewTokenBucket(5000, 10) },
			n:          1,
			goRoutines: 1,
		},
		{
			ctor:       func() rateLimiter { return NewTokenBucket(1000, 10) },
			n:          1,
			goRoutines: 5,
		},
	})

	runTestCaseTake(t, testcases)
}
