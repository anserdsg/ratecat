package metric

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/config"
)

type MetricBackend interface {
	ReportAPICmdRPS(rps float64) error
	ReportResourceRPS(name string, rps float64) error
}

type Metrics struct {
	ctx      context.Context
	cfg      *config.Config
	apiCmd   RPS
	resoures sync.Map
	backend  MetricBackend
}

func NewMetrics(ctx context.Context, cfg *config.Config, backend MetricBackend) *Metrics {
	return &Metrics{ctx: ctx, cfg: cfg, backend: backend}
}

func (m *Metrics) AddAPICmdCount() {
	m.AddAPICmdCountN(1)
}

func (m *Metrics) AddAPICmdCountN(n uint32) {
	m.apiCmd.Add(n)
}

func (m *Metrics) ReportAPICmdRPS(interval time.Duration) error {
	rps := m.apiCmd.Calc(interval)
	if m.backend == nil {
		return errors.New("Metrics backend doesn't set")
	}
	return m.backend.ReportAPICmdRPS(rps)
}

func (m *Metrics) GetAPICmdAvgRPS(d time.Duration) float64 {
	return m.apiCmd.AvgRPS(d)
}

func (m *Metrics) AddResourceCount(name string) {
	m.AddResourceCountN(name, 1)
}

func (m *Metrics) AddResourceCountN(name string, n uint32) {
	v, _ := m.resoures.LoadOrStore(name, &RPS{})
	if v == nil {
		return
	}
	rps, _ := v.(*RPS)
	rps.Add(n)
}

func (m *Metrics) ReportResourceRPS(interval time.Duration) error {
	if m.backend == nil {
		return errors.New("Metrics backend doesn't set")
	}

	var firstErr error
	m.resoures.Range(func(k, v any) bool {
		name, _ := k.(string)
		item, _ := v.(*RPS)
		rps := item.Calc(interval)
		err := m.backend.ReportResourceRPS(name, rps)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return true
	})

	return firstErr
}
