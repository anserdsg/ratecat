package rate

import (
	"time"

	"github.com/anserdsg/ratecat/v1/internal/limit"
)

type TokenBucket struct {
	impl *limit.TokenBucket
}

func NewTokenBucket(rate float64, burst uint32) *TokenBucket {
	return &TokenBucket{impl: limit.NewTokenBucket(rate, burst)}
}

func (b *TokenBucket) Allow() bool {
	return b.impl.Allow(time.Now(), 1)
}

func (b *TokenBucket) AllowN(t time.Time, n uint32) bool {
	return b.impl.Allow(t, n)
}
