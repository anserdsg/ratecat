//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/anserdsg/ratecat/v1/internal/cluster"
	"github.com/anserdsg/ratecat/v1/internal/cluster/probe"
	"github.com/anserdsg/ratecat/v1/internal/cmd"
	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/anserdsg/ratecat/v1/internal/mgr"
	"github.com/anserdsg/ratecat/v1/internal/server/api"
	"github.com/google/wire"
)

func provideMetrics(ctx context.Context, cfg *config.Config) *metric.Metrics {
	var backend metric.MetricBackend = metric.NullMetric

	if !cfg.Metrics.Enabled {
		backend = metric.NullMetric
	} else if config.IsDevEnv() {
		backend = metric.NewLogMetricBackend()
	} else {
		backend = metric.NullMetric
	}
	return metric.NewMetrics(ctx, cfg, backend)
}

type serviceSet struct {
	API     *api.Server
	Cluster *cluster.ClusterServer
}

func initServices(ctx context.Context) (*serviceSet, error) {
	wire.Build(
		config.Get,
		mgr.NewResourceManager,
		cmd.NewCmdDispatcher,
		provideMetrics,
		probe.NewNodeProber,
		wire.NewSet(
			cluster.NewServer,
			api.NewServer,
			wire.Struct(new(serviceSet), "API", "Cluster"),
		),
	)
	return &serviceSet{}, nil
}
