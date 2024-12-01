package client

import (
	"time"

	"go.uber.org/zap"
)

type clientOptions struct {
	dialTimeout   time.Duration
	checkInterval time.Duration
	logger        *zap.SugaredLogger
}

type ClientOption interface {
	apply(*clientOptions)
}

type fto struct {
	f func(*clientOptions)
}

func (inst *fto) apply(to *clientOptions) {
	inst.f(to)
}

func newFTO(f func(*clientOptions)) *fto {
	return &fto{f: f}
}

func WithDialTimeout(t time.Duration) ClientOption {
	return newFTO(func(to *clientOptions) {
		to.dialTimeout = t
	})
}

func WithCheckInterval(t time.Duration) ClientOption {
	return newFTO(func(to *clientOptions) {
		to.checkInterval = t
	})
}

func WithLogger(logger *zap.Logger) ClientOption {
	return newFTO(func(to *clientOptions) {
		if logger != nil {
			to.logger = logger.Sugar()
		}
	})
}
