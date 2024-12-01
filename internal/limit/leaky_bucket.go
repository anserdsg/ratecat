package limit

import (
	"fmt"
	"sync"
	"time"
)

type LeakyBucket struct {
	sync.Mutex
	rate        float64
	capability  uint32
	drops       float64
	drainPeriod time.Duration
	lastDrained time.Time
	nextDrain   time.Time
}

func NewLeakyBucket(rate float64, capability uint32) *LeakyBucket {
	now := time.Now()
	drainPeriod := time.Second / time.Duration(rate)

	return &LeakyBucket{
		rate:        rate,
		capability:  capability,
		drops:       0,
		drainPeriod: drainPeriod,
		lastDrained: now,
		nextDrain:   now.Add(drainPeriod),
	}
}

func (b *LeakyBucket) Rate() float64 { return b.rate }
func (b *LeakyBucket) Burst() uint32 { return 0 }
func (b *LeakyBucket) String() string {
	return fmt.Sprintf("leakybucket_rate=%.1f_cap=%d", b.rate, b.capability)
}

func (b *LeakyBucket) Allow(t time.Time, n uint32) bool {
	b.Lock()
	defer b.Unlock()

	b.drain(t)

	if b.drops+float64(n) <= float64(b.capability) {
		b.drops += float64(n)
		return true
	}

	return false
}

func (b *LeakyBucket) Take(t time.Time, n uint32) (time.Duration, error) {
	b.Lock()
	defer b.Unlock()

	b.drain(t)

	delay := time.Duration(b.drops * float64(b.drainPeriod))

	if b.drops+float64(n) > float64(b.capability) {
		// b.drops = float64(b.capability)
		return delay, ErrOverflow
	} else {
		b.drops += float64(n)
	}

	if delay < 0 {
		delay = 0
	}
	return delay, nil
}

func (b *LeakyBucket) drain(t time.Time) {
	if b.drops > 0 && t.After(b.nextDrain) {
		// fmt.Printf("drops=%.2f\n", b.drops)
		// fDrainPeriod := float64(b.drainPeriod)
		// drips := float64(t.Sub(b.lastDrained)) / fDrainPeriod
		drips := float64(t.Sub(b.lastDrained) / b.drainPeriod)
		// if b.drops >= drips {
		// 	b.drops -= drips
		// } else {
		// 	b.drops = 0
		// }
		b.drops -= drips
		if b.drops < -float64(b.capability) {
			b.drops = -float64(b.capability)
		}
		// b.lastDrained = t
		b.lastDrained = b.lastDrained.Add(time.Duration(int64(drips) * int64(b.drainPeriod)))
		b.nextDrain = b.lastDrained.Add(b.drainPeriod)
	}
}
