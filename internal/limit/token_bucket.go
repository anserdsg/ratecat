package limit

import (
	"fmt"
	"sync"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/pbutil"
	"github.com/gogo/protobuf/proto"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

type tokenBucketLimiter struct {
	limiter *TokenBucket
}

func NewTokenBucketEmptyLimiter() *tokenBucketLimiter {
	return &tokenBucketLimiter{limiter: &TokenBucket{}}
}

func NewTokenBucketLimiter(rate float64, burst uint32) *tokenBucketLimiter {
	return &tokenBucketLimiter{limiter: NewTokenBucket(rate, burst)}
}

func (l *tokenBucketLimiter) Type() string {
	return pb.LimiterType_TokenBucket.String()
}

func (l *tokenBucketLimiter) PBType() pb.LimiterType {
	return pb.LimiterType_TokenBucket
}

func (l *tokenBucketLimiter) String() string {
	return fmt.Sprintf("limiter type=%s, rate=%f, burst=%d", l.Type(), l.limiter.rate, l.limiter.burst)
}

func (l *tokenBucketLimiter) ToLimiterPB() proto.Message {
	return &pb.TokenBucketLimiter{
		Rate:       l.limiter.rate,
		Burst:      l.limiter.burst,
		TokenCount: l.limiter.tokenCount,
		FillPeriod: int64(l.limiter.fillPeriod),
		LastFilled: pbutil.TimeToTimestamp(l.limiter.lastFilled),
		NextFill:   pbutil.TimeToTimestamp(l.limiter.nextFill),
	}
}

func (l *tokenBucketLimiter) FromResPB(v *pb.Resource) error {
	res, ok := v.Limiter.(*pb.Resource_TokenBucket)
	if !ok {
		return ErrInvalidLimiter
	}
	msg := res.TokenBucket

	l.limiter.rate = msg.Rate
	l.limiter.burst = msg.Burst
	l.limiter.tokenCount = msg.TokenCount
	l.limiter.fillPeriod = time.Duration(msg.FillPeriod)
	l.limiter.lastFilled = pbutil.TimestampToTime(msg.LastFilled)
	l.limiter.nextFill = pbutil.TimestampToTime(msg.NextFill)

	return nil
}

func (l *tokenBucketLimiter) Allow(t time.Time, n uint32) bool {
	return l.limiter.Allow(t, n)
}

func (l *tokenBucketLimiter) Take(t time.Time, n uint32) (time.Duration, error) {
	return l.limiter.Take(t, n)
}

type TokenBucket struct {
	sync.Mutex
	rate       float64
	burst      uint32
	tokenCount float64
	fillPeriod time.Duration
	lastFilled time.Time
	nextFill   time.Time
}

// NewTokenBucket takes an initial burst for the rate limiter and a fillPeriod
// which determines the time between adding single units of burst to the limiter
func NewTokenBucket(rate float64, burst uint32) *TokenBucket {
	t := time.Now()
	fillPeriod := time.Second / time.Duration(rate)
	return &TokenBucket{
		rate:       rate,
		fillPeriod: fillPeriod,
		burst:      burst,
		tokenCount: float64(burst),
		lastFilled: t,
		nextFill:   t.Add(fillPeriod),
	}
}

func (b *TokenBucket) Rate() float64 { return b.rate }
func (b *TokenBucket) Burst() uint32 { return b.burst }
func (b *TokenBucket) String() string {
	return fmt.Sprintf("tokenbucket_rate=%.1f_burst=%d", b.rate, b.burst)
}

func (b *TokenBucket) Allow(t time.Time, n uint32) bool {
	b.Lock()
	defer b.Unlock()

	b.fill(t)

	if b.tokenCount < float64(n) {
		return false
	}
	b.tokenCount -= float64(n)
	return true
}

func (b *TokenBucket) Take(t time.Time, n uint32) (time.Duration, error) {
	b.Lock()
	defer b.Unlock()

	b.fill(t)

	b.tokenCount -= float64(n)
	if b.tokenCount < 0 {
		if b.tokenCount < -float64(b.burst) {
			b.tokenCount = -float64(b.burst)
		}
		return time.Duration(-b.tokenCount * float64(b.fillPeriod)), nil
	}

	return 0, nil
}

func (b *TokenBucket) fill(t time.Time) {
	if t.After(b.nextFill) {
		burst := float64(b.burst)
		newTokens := float64(t.Sub(b.lastFilled)) / float64(b.fillPeriod)
		if b.tokenCount+newTokens <= burst {
			b.tokenCount += newTokens
		} else {
			b.tokenCount = burst
		}
		b.lastFilled = t
		b.nextFill = t.Add(b.fillPeriod)
	}
}
