package api

import (
	"context"
	"fmt"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/cluster"
	"github.com/anserdsg/ratecat/v1/internal/cmd"
	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/anserdsg/ratecat/v1/internal/mgr"
	"github.com/panjf2000/gnet/v2"
)

type Server struct {
	ctx           context.Context
	cfg           *config.Config
	resourceStore *mgr.ResourceManager
	cluster       *cluster.ClusterServer
	evtHandler    EventHandler
	metric        *metric.Metrics
	ready         chan bool
}

func NewServer(
	ctx context.Context,
	cfg *config.Config,
	rs *mgr.ResourceManager,
	disp *cmd.CmdDispatcher,
	ct *cluster.ClusterServer,
	mt *metric.Metrics,
) (*Server, error) {
	server := &Server{
		ctx:           ctx,
		cfg:           cfg,
		resourceStore: rs,
		cluster:       ct,
		metric:        mt,
		ready:         make(chan bool, 1),
	}

	switch cfg.ApiListenUrl.Scheme {
	case "tcp":
		server.evtHandler = newTCPEventHandler(ctx, cfg, disp, mt, server.ready)
	default:
		return nil, fmt.Errorf("Invalid listen address: %s, unknown protocol: %s", cfg.ApiListenUrl.String(), cfg.ApiListenUrl.Scheme)
	}

	return server, nil
}

func (rs *Server) Start() error {
	return gnet.Run(rs.evtHandler, rs.cfg.ApiListenUrl.String(),
		gnet.WithLogLevel(logging.WarnLevel),
		gnet.WithReuseAddr(true),
		gnet.WithReusePort(true),
		gnet.WithMulticore(rs.cfg.MultiCore),
		gnet.WithTicker(true),
	)
}

func (rs *Server) StartAsync() {
	go func() {
		logging.Debugw("starting server async", "addr", rs.cfg.ApiListenUrl.String())
		if err := rs.Start(); err != nil {
			logging.Fatalw("Failed to start server", "error", err)
			rs.ready <- false
		}
	}()
}

func (rs *Server) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = rs.ctx
	}
	return gnet.Stop(ctx, rs.cfg.ApiListenUrl.String())
}

func (rs *Server) WaitReady() bool {
	ctx, cancel := context.WithTimeout(rs.ctx, 5*time.Second)
	defer cancel()
	for {
		select {
		case v, ok := <-rs.ready:
			if ok {
				return v
			}
		case <-ctx.Done():
			return false
		}
	}
}
