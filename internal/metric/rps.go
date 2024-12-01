package metric

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/pbutil"
	pb "github.com/anserdsg/ratecat/v1/proto"
)

var maxHistoryDuration time.Duration = time.Hour

func MaxHistoryDuration() time.Duration {
	return maxHistoryDuration
}

func SetMaxHistoryDuration(d time.Duration) {
	maxHistoryDuration = d
}

type rpsEntry struct {
	rps float64
	t   time.Time
}

type rpsHistory struct {
	mu      sync.RWMutex
	entries []*rpsEntry // TODO: use circular stack
}

func (h *rpsHistory) reset() {
	h.entries = make([]*rpsEntry, 0, 1)
}

func (h *rpsHistory) pushNow(rps float64) {
	h.push(rps, time.Now())
}

func (h *rpsHistory) push(rps float64, now time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.entries == nil {
		h.reset()
	}

	h.entries = append(h.entries, &rpsEntry{rps: rps, t: now})
	if now.Sub(h.entries[0].t) > maxHistoryDuration {
		h.entries = h.entries[1:]
	}
}

func (h *rpsHistory) avgRPS(d time.Duration) float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entrySize := len(h.entries)
	if entrySize == 0 {
		return 0.0
	}

	since := time.Now().Add(-d)
	rpsCount := 0
	var rpsSum float64 = 0
	for i := entrySize - 1; i >= 0; i-- {
		entry := h.entries[i]
		if since.Compare(entry.t) <= 0 {
			rpsSum += entry.rps
			rpsCount++
		}
	}
	return rpsSum / float64(rpsCount)
}

func (h *rpsHistory) assginToPB(rpsPB *pb.ReqPerSec) {
	if rpsPB == nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	rpsPB.Histories = make([]*pb.ReqPerSecEntry, 0, len(h.entries))
	for _, entry := range h.entries {
		rpsPB.Histories = append(rpsPB.Histories, &pb.ReqPerSecEntry{
			Rps:  entry.rps,
			Time: pbutil.TimeToTimestamp(entry.t),
		})
	}
}

func (h *rpsHistory) fromPB(rpsPB *pb.ReqPerSec) {
	if rpsPB == nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	h.reset()
	now := time.Now()
	for _, entryPB := range rpsPB.Histories {
		entryTime := pbutil.TimestampToTime(entryPB.Time)
		if now.Sub(entryTime) < maxHistoryDuration {
			h.entries = append(h.entries, &rpsEntry{rps: entryPB.Rps, t: entryTime})
		}
	}
}

type RPS struct {
	reqCount atomic.Uint32
	hist     rpsHistory
}

func (r *RPS) Add(n uint32) {
	r.reqCount.Add(n)
}

func (r *RPS) Calc(interval time.Duration) float64 {
	reqs := r.reqCount.Swap(0)
	rps := float64(reqs) / interval.Seconds()
	r.hist.pushNow(rps)

	return rps
}

func (r *RPS) AvgRPS(d time.Duration) float64 {
	return r.hist.avgRPS(d)
}

func (r *RPS) Reset() {
	r.reqCount.Store(0)
	r.hist.reset()
}

func (r *RPS) ToPB() *pb.ReqPerSec {
	rpsPB := &pb.ReqPerSec{
		ReqCount:  r.reqCount.Load(),
		Histories: make([]*pb.ReqPerSecEntry, 0, len(r.hist.entries)),
	}
	r.hist.assginToPB(rpsPB)

	return rpsPB
}

func (r *RPS) FromPB(rpsPB *pb.ReqPerSec) {
	r.reqCount.Store(rpsPB.ReqCount)
	r.hist.fromPB(rpsPB)
}
