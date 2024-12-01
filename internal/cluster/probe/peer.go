package probe

import (
	"context"
	"fmt"
	"sync"

	cpb "github.com/anserdsg/ratecat/v1/internal/cluster/pb"
	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/shaj13/raft"
	"google.golang.org/grpc"
)

type PeerClient struct {
	client cpb.ClusterClient
}

func NewPeerClient(ctx context.Context, target string) (*PeerClient, error) {
	conn, err := grpc.DialContext(ctx, target, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("Failed to create gRPC channel for target: %s, error: %w", target, err)
	}

	peerClient := &PeerClient{client: cpb.NewClusterClient(conn)}

	return peerClient, nil
}

type PeerClientMgr struct {
	ctx      context.Context
	cfg      *config.Config
	clients  *sync.Map
	raftNode *raft.Node
}

func newPeerClientMgr(ctx context.Context, cfg *config.Config) *PeerClientMgr {
	mgr := &PeerClientMgr{ctx: ctx, cfg: cfg, clients: new(sync.Map), raftNode: nil}
	for _, peerUrl := range cfg.Cluster.PeerUrls {
		_ = mgr.addClient(peerUrl.NodeID(), peerUrl.Host)
	}

	return mgr
}

func (mgr *PeerClientMgr) addClient(nodeID uint32, host string) error {
	if _, ok := mgr.clients.Load(nodeID); ok {
		return nil
	}

	return mgr.replaceClient(nodeID, host)
}

func (mgr *PeerClientMgr) replaceClient(nodeID uint32, host string) error {
	target := fmt.Sprintf("dns:///%s", host)
	peerClient, err := NewPeerClient(mgr.ctx, target)
	if err != nil {
		logging.Errorw("Failed to create peer client", "error", err)
		return fmt.Errorf("Failed to create peer client, error:%w", err)
	}

	mgr.clients.Store(nodeID, peerClient)

	return nil
}

func (mgr *PeerClientMgr) removeClient(nodeID uint32) {
	mgr.clients.Delete(nodeID)
}

func (mgr *PeerClientMgr) getClient(nodeID uint32) cpb.ClusterClient {
	val, ok := mgr.clients.Load(nodeID)
	if !ok {
		return nil
	}
	return val.(*PeerClient).client
}

func (mgr *PeerClientMgr) setRaftNode(raftNode *raft.Node) {
	mgr.raftNode = raftNode
}

func (mgr *PeerClientMgr) isPeerActive(nodeID uint32) bool {
	if mgr.raftNode == nil {
		return false
	}

	mem, ok := mgr.raftNode.GetMemebr(uint64(nodeID))
	if !ok {
		return false
	}

	if !mem.IsActive() || mem.Type() != raft.VoterMember {
		return false
	}

	return true
}

func (mgr *PeerClientMgr) iter(iterFunc func(key uint32, client cpb.ClusterClient) bool) error {
	mgr.clients.Range(func(k, v any) bool {
		nodeID, ok := k.(uint32)
		if !ok {
			logging.Warnf("PeerClientMgr.Iter: failed to get node id")
			return true
		}

		if !mgr.isPeerActive(nodeID) {
			return true
		}

		peer, ok := v.(*PeerClient)
		if !ok {
			logging.Warnf("PeerClientMgr.Iter: failed to get peer client")
			return true
		}
		return iterFunc(nodeID, peer.client)
	})

	return nil
}
