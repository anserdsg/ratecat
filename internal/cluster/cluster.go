package cluster

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/cmd"
	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/anserdsg/ratecat/v1/internal/mgr"
	"github.com/gogo/protobuf/types"

	"github.com/anserdsg/ratecat/v1/internal/cluster/probe"
	"github.com/shaj13/raft"
	"github.com/shaj13/raft/transport"
	"github.com/shaj13/raft/transport/raftgrpc"
	"go.etcd.io/etcd/server/v3/wal"
	"google.golang.org/grpc"

	cpb "github.com/anserdsg/ratecat/v1/internal/cluster/pb"
)

func init() {
	wal.SegmentSizeBytes = 4 * 1024 * 1024 // set WAL pre-allocated size to 4MB
}

type ClusterServer struct {
	ctx           context.Context
	cfg           *config.Config
	raftServer    *grpc.Server
	node          *raft.Node
	nodeProber    probe.NodeProber
	nodeStartOpts []raft.StartOption
	resMgr        *mgr.ResourceManager
	cmdDisp       *cmd.CmdDispatcher
}

func NewServer(
	ctx context.Context,
	cfg *config.Config,
	st *mgr.ResourceManager,
	disp *cmd.CmdDispatcher,
	mt *metric.Metrics,
	np probe.NodeProber,
) *ClusterServer {
	if !cfg.Cluster.Enabled {
		return &ClusterServer{ctx: ctx, cfg: cfg}
	}

	raftgrpc.Register(
		raftgrpc.WithDialOptions(grpc.WithInsecure()),
	)

	ep := cfg.Cluster.Url
	members := []raft.RawMember{
		{ID: uint64(ep.NodeID()), Address: ep.Host},
	}
	logging.Debugw("Set cluster url", "id", ep.NodeID(), "name", ep.NodeName(), "address", ep.String())

	if len(cfg.Cluster.PeerUrls) > 0 {
		for _, peer := range cfg.Cluster.PeerUrls {
			logging.Debugw("Set cluster peer", "id", peer.NodeID(), "name", peer.NodeName(), "address", peer.String())
			members = append(members, raft.RawMember{ID: uint64(peer.NodeID()), Address: peer.Host})
		}
	}
	opts := []raft.Option{
		raft.WithContext(ctx),
		raft.WithStateDIR(cfg.Cluster.StateDir),
		raft.WithLogger(logging.RaftLogger(false)),
		raft.WithTickInterval(100 * time.Millisecond),
		raft.WithElectionTick(10),
	}

	node := raft.NewNode(st, transport.GRPC, opts...)

	raftServer := grpc.NewServer()
	raftgrpc.RegisterHandler(raftServer, node.Handler())

	grpcServer := newGRPCServer(ctx, cfg, node, mt, st, disp)
	cpb.RegisterClusterServer(raftServer, grpcServer)

	if np == nil {
		np = probe.NewNullNodeProber()
	}
	np.SetRaftNode(node)

	ct := &ClusterServer{
		ctx:        ctx,
		cfg:        cfg,
		raftServer: raftServer,
		node:       node,
		resMgr:     st,
		cmdDisp:    disp,
		nodeProber: np,
	}

	ct.nodeStartOpts = append(ct.nodeStartOpts,
		raft.WithAddress(cfg.Cluster.Url.Host),
		raft.WithFallback(
			raft.WithInitCluster(),
			raft.WithRestart(),
		),
		raft.WithMembers(members...),
	)
	return ct
}

func (ct *ClusterServer) StartAsync() {
	if !ct.cfg.Cluster.Enabled {
		return
	}
	logging.Infow("Start cluster", "address", ct.cfg.Cluster.Url.String())

	ct.startGRPCServerAsync()
	ct.startNodeAsync()
	ct.resMgr.StartAsync()
}

func (ct *ClusterServer) WaitReady() bool {
	if !ct.cfg.Cluster.Enabled {
		return true
	}
	return ct.waitReady(false)
}

func (ct *ClusterServer) WaitAllNodeReady() bool {
	if !ct.cfg.Cluster.Enabled {
		return true
	}
	return ct.waitReady(true)
}

func (ct *ClusterServer) Stop(ctx context.Context) error {
	ct.resMgr.Stop()
	ct.raftServer.GracefulStop()
	return ct.node.Shutdown(ctx)
}

func (ct *ClusterServer) startGRPCServerAsync() {
	go func() {
		addr := ct.cfg.Cluster.Url
		listener, err := net.Listen("tcp", addr.Host)
		if err != nil {
			logging.Errorw("Failed to listen address of gRPC server", "address", addr.String())
			return
		}
		err = ct.raftServer.Serve(listener)
		if err != nil {
			logging.Errorw("Failed to listen address of gRPC server", "address", addr.String())
			return
		}
	}()
}

func (ct *ClusterServer) startNodeAsync() {
	go func() {
		err := ct.node.Start(ct.nodeStartOpts...)
		if err != nil && !errors.Is(err, raft.ErrNodeStopped) {
			logging.Errorw("Failed to start cluster node", "error", err)
		}
	}()
}

func (ct *ClusterServer) waitReady(checkAllMember bool) bool {
	ready := make(chan bool, 1)
	ctx, cancel := context.WithTimeout(ct.ctx, 10*time.Second)
	defer cancel()

	go ct.checkNodeReady(ctx, checkAllMember, ready)
	for {
		select {
		case v, ok := <-ready:
			logging.Debugw("Cluster.WaitReady", "v", v, "ok", ok)
			if ok {
				ct.resMgr.Ready()
				return v
			}
		case <-ctx.Done():
			logging.Debug("Cluster.WaitReady timeout")
			return false
		}
	}
}

func (ct *ClusterServer) checkNodeReady(ctx context.Context, checkAllMember bool, ready chan bool) {
	memberCount := len(ct.cfg.Cluster.PeerUrls) + 1

	for {
		members := ct.node.Members()
		if len(members) < memberCount {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if !checkAllMember {
			// establish grpc channel to itself and check node info
			conn, err := grpc.DialContext(ctx, fmt.Sprintf("dns:///%s", ct.cfg.Cluster.Url.Host), grpc.WithInsecure())
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			client := cpb.NewClusterClient(conn)
			nodeInfo, err := client.GetNodeInfo(ctx, &types.Empty{})
			if err != nil || !nodeInfo.Active {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// break main loop
			break
		}

		allActive := true
		hasLeader := false
		for _, m := range members {
			if m.ID() == ct.node.Leader() {
				hasLeader = true
			}
			if ct.node.Whoami() == m.ID() {
				continue
			}

			if !m.IsActive() {
				allActive = false
				break
			}

			nodeInfo, err := ct.nodeProber.GetPeerNodeInfo(uint32(m.ID()))
			if err != nil || !nodeInfo.Active {
				allActive = false
				break
			}
		}

		if !allActive || !hasLeader {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		break
	}

	ready <- true
}
