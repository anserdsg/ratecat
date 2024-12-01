package testutil

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/cluster"
	"github.com/anserdsg/ratecat/v1/internal/cluster/probe"
	"github.com/anserdsg/ratecat/v1/internal/cmd"
	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/anserdsg/ratecat/v1/internal/mgr"
	"github.com/anserdsg/ratecat/v1/internal/server/api"
	treq "github.com/stretchr/testify/require"
)

type ClusterInfo struct {
	NodeID        uint32
	NodeName      string
	Metrics       *metric.Metrics
	CmdDisp       *cmd.CmdDispatcher
	ResMgr        *mgr.ResourceManager
	NodeProber    probe.NodeProber
	APIServer     *api.Server
	APIServerHost string
	ClusterServer *cluster.ClusterServer
	StateDir      string

	id      int
	peerIDs []int
	stopped bool
}

func (ci *ClusterInfo) StartAsync(ctx context.Context) error {
	if ci.stopped {
		fmt.Printf("=== Stopped, InitClusterServer ===\n\n")
		newCluster, err := InitClusterServer(ctx, ci.id, ci.peerIDs)
		if err != nil {
			return err
		}
		*ci = *newCluster
		ci.stopped = false
	}
	ci.ClusterServer.StartAsync()
	ci.APIServer.StartAsync()

	return nil
}

func (ci *ClusterInfo) Stop(ctx context.Context) error {
	if ci.stopped {
		return nil
	}
	err1 := ci.APIServer.Stop(ctx)
	err2 := ci.ClusterServer.Stop(ctx)
	ci.stopped = true
	return errors.Join(err1, err2)
}

type ClusterInfos map[uint32]*ClusterInfo

func (ci ClusterInfos) WaitClusterReady() bool {
	for _, info := range ci {
		ready := info.APIServer.WaitReady()
		if !ready {
			return false
		}
	}

	for _, info := range ci {
		ready := info.ClusterServer.WaitAllNodeReady()
		if !ready {
			return false
		}
	}

	return true
}

func (ci ClusterInfos) StartAync(ctx context.Context, nodeID uint32) error {
	node, ok := ci[nodeID]
	if !ok || node == nil {
		return fmt.Errorf("Failed to get node %d", nodeID)
	}
	return node.StartAsync(ctx)
}

func (ci ClusterInfos) Stop(ctx context.Context, nodeID uint32) error {
	node, ok := ci[nodeID]
	if !ok || node == nil {
		return fmt.Errorf("Failed to get node %d", nodeID)
	}
	return node.Stop(ctx)
}

func (ci ClusterInfos) GetNode(nodeID uint32) *ClusterInfo {
	return ci[nodeID]
}

func (ci ClusterInfos) RemoveClusterNode(nodeID uint32) {
	delete(ci, nodeID)
}

func (ci ClusterInfos) GetLeaderNode() *ClusterInfo {
	for i := 0; i < 10; i++ {
		for _, info := range ci {
			if info.ResMgr.IsNodeLeader() {
				return info
			}
		}
		time.Sleep(time.Second)
	}
	return nil
}

func (ci ClusterInfos) GetNonLeaderNode() *ClusterInfo {
	for i := 0; i < 10; i++ {
		for _, info := range ci {
			if !info.ResMgr.IsNodeLeader() {
				return info
			}
		}
	}
	time.Sleep(time.Second)
	return nil
}

func InitClusterServers(t *testing.T, ctx context.Context, nodeCount int) ClusterInfos {
	var require *treq.Assertions
	if t != nil {
		require = treq.New(t)
	}

	clusterInfos := make(ClusterInfos, nodeCount)
	clusterIds := make([]int, nodeCount)
	for i := 0; i < nodeCount; i++ {
		clusterIds[i] = i + 1
	}

	for i, id := range clusterIds {
		peers := make([]int, nodeCount)
		copy(peers, clusterIds)
		peers = append(peers[:i], peers[i+1:]...)
		clusterInfo, err := InitClusterServer(ctx, id, peers)
		if require != nil {
			require.NoError(err)
		}
		clusterInfos[clusterInfo.NodeID] = clusterInfo
	}

	return clusterInfos
}

func InitClusterServer(ctx context.Context, id int, peerIDs []int) (*ClusterInfo, error) {
	cfg := config.CloneDefault()
	_ = cfg.ApiListenUrl.Decode(fmt.Sprintf("tcp://0.0.0.0:%d", 14000+id))
	_ = cfg.ApiAdvertiseUrl.Decode(fmt.Sprintf("tcp://127.0.0.1:%d", 14000+id))
	cfg.Cluster.Enabled = true
	cfg.Cluster.StateDir = fmt.Sprintf("/tmp/test_rc%d", id)
	_ = cfg.Cluster.Url.Decode(fmt.Sprintf("tcp://node%d@127.0.0.1:%d", id, 15000+id))

	for _, peerID := range peerIDs {
		peerURL, _ := config.ParseURL(fmt.Sprintf("tcp://node%d@127.0.0.1:%d", peerID, 15000+peerID))
		cfg.Cluster.PeerUrls = append(cfg.Cluster.PeerUrls, peerURL)
	}

	metrics := metric.NewMetrics(ctx, cfg, metric.NullMetric)
	nodeProber := probe.NewNodeProber(ctx, cfg, metrics)
	resMgr := mgr.NewResourceManager(ctx, cfg, nodeProber)
	disp := cmd.NewCmdDispatcher(ctx, cfg, resMgr, metrics)
	cluster := cluster.NewServer(ctx, cfg, resMgr, disp, metrics, nodeProber)

	api, err := api.NewServer(ctx, cfg, resMgr, disp, nil, metrics)
	if err != nil {
		return nil, err
	}

	return &ClusterInfo{
		NodeID:        resMgr.CurNodeID(),
		NodeName:      resMgr.CurNodeName(),
		Metrics:       metrics,
		CmdDisp:       disp,
		ResMgr:        resMgr,
		NodeProber:    nodeProber,
		APIServer:     api,
		APIServerHost: cfg.ApiAdvertiseUrl.Host,
		ClusterServer: cluster,
		StateDir:      cfg.Cluster.StateDir,

		id:      id,
		peerIDs: peerIDs,
	}, nil
}

func SetupBenchmarkEnv() {
	cfg := config.GetDefault()
	cfg.Env = "benchmark"
	cfg.LogLevel = "error"
	logging.Init()
}
