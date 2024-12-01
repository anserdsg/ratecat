package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/logging"
)

func init() {
	if err := config.Init(); err != nil {
		fmt.Print("Failed to initiliaze configuration", "error", err)
		os.Exit(1)
	}

	logging.Init()
}

func main() {
	cfg := config.GetDefault()
	ctx := context.Background()

	logging.Infow("Ratecat configuration",
		"env", cfg.Env,
		"log_level", cfg.LogLevel,
		"api_listen_addr", cfg.ApiListenUrl.String(),
		"api_advertise_url", cfg.ApiAdvertiseUrl.String(),
		"multi_core", cfg.MultiCore,
		"worker_pool", cfg.WorkerPool,
		"cluster.enabled", cfg.Cluster.Enabled,
	)
	if cfg.Cluster.Enabled {
		peers := make([]string, 0, len(cfg.Cluster.PeerUrls))
		for _, peer := range cfg.Cluster.PeerUrls {
			peers = append(peers, peer.String())
		}
		logging.Infow("Cluster configuration",
			"url", cfg.Cluster.Url.String(),
			"peers", peers,
			"state_dir", cfg.Cluster.StateDir,
		)
	}

	services, err := initServices(ctx)
	apiServer := services.API
	clusterServer := services.Cluster

	if err != nil {
		logging.Fatalw("Failed to initialize service", "error", err)
	}

	if cfg.Cluster.Enabled {
		clusterServer.StartAsync()
	}

	apiServer.StartAsync()

	if cfg.Cluster.Enabled {
		ready := clusterServer.WaitReady()
		if !ready {
			logging.Fatalw("Failed to start cluster server")
			os.Exit(1)
		}
		logging.Infow("Cluster Server has started", "address", cfg.Cluster.Url.String())
	}

	ready := apiServer.WaitReady()
	if !ready {
		logging.Fatalw("Failed to start API server")
		os.Exit(1)
	}
	logging.Infow("API Server has started", "address", cfg.ApiListenUrl.String())

	quiteSig := make(chan os.Signal, 1)
	signal.Notify(quiteSig, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)

	<-quiteSig

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	if err := apiServer.Stop(shutdownCtx); err != nil {
		logging.Fatalw("Server shutdown timed out", "error", err.Error())
	}

	if cfg.Cluster.Enabled {
		if err := clusterServer.Stop(shutdownCtx); err != nil {
			logging.Fatalw("Cluster shutdown timed out", "error", err.Error())
		}
	}

	logging.Info("All Services has been shutdowned")
}
