package probe

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/anserdsg/ratecat/v1/internal/pbutil"
	"github.com/gogo/protobuf/types"
	"github.com/google/uuid"
	"github.com/shaj13/raft"

	cpb "github.com/anserdsg/ratecat/v1/internal/cluster/pb"
	pb "github.com/anserdsg/ratecat/v1/proto"
)

const (
	probeInteral        time.Duration = time.Second
	avgRPSDuration      time.Duration = time.Minute
	curNodeWeightFactor float64       = 1.3
)

type nodeProber struct {
	ctx         context.Context
	cfg         *config.Config
	metrics     *metric.Metrics
	peerClients *PeerClientMgr
	curNodeID   uint32
	raftNode    *raft.Node

	mu            sync.RWMutex
	ready         atomic.Bool
	loadDiff      float64
	lightNodeID   uint32
	heavyNodeID   uint32
	nodeInfoCache map[uint32]*pb.NodeInfo
}

func NewNodeProber(ctx context.Context, cfg *config.Config, mt *metric.Metrics) NodeProber {
	curNodeID := cfg.Cluster.Url.NodeID()
	return &nodeProber{
		ctx:           ctx,
		cfg:           cfg,
		metrics:       mt,
		peerClients:   newPeerClientMgr(ctx, cfg),
		curNodeID:     curNodeID,
		lightNodeID:   curNodeID,
		heavyNodeID:   curNodeID,
		nodeInfoCache: make(map[uint32]*pb.NodeInfo),
	}
}

func (np *nodeProber) Init() bool {
	members := np.raftNode.Members()
	if len(members) < len(np.cfg.Cluster.PeerUrls)+1 {
		return false
	}

	for _, m := range members {
		nodeID := uint32(m.ID())
		if nodeID == np.curNodeID {
			np.nodeInfoCache[nodeID] = &pb.NodeInfo{
				Id:           nodeID,
				Name:         np.cfg.ClusterNodeName(),
				ApiEndpoint:  np.cfg.ApiAdvertiseUrl.String(),
				GrpcEndpoint: fmt.Sprintf("dns:///%s", np.cfg.Cluster.Url.Host),
				Type:         m.Type().String(),
				Leader:       nodeID == uint32(np.raftNode.Leader()),
				Active:       m.IsActive(),
				ActiveSince:  pbutil.TimeToTimestamp(m.ActiveSince()),
			}
			continue
		}
		peerNodeInfo, err := np.GetPeerNodeInfo(nodeID)
		if err != nil {
			logging.Error(err.Error())
			continue
		}
		np.nodeInfoCache[nodeID] = peerNodeInfo
	}

	np.ready.Store(true)
	return true
}

func (np *nodeProber) IsReady() bool {
	return np.ready.Load()
}

func (np *nodeProber) CheckMemberStatus() []*NodeEvent {
	if !np.IsReady() {
		np.Init()
		return []*NodeEvent{}
	}

	np.mu.Lock()
	defer np.mu.Unlock()

	events := make([]*NodeEvent, 0)
	members := np.raftNode.Members()
	for _, m := range members {
		if m == nil {
			continue
		}

		// NewMember event
		nodeID := uint32(m.ID())
		cache, ok := np.nodeInfoCache[nodeID]
		if !ok {
			err := np.peerClients.addClient(nodeID, m.Address())
			if err != nil {
				logging.Warnw("NodeProber.CheckMemberStatus: failed to add peer client", "error", err)
				continue
			}

			nodeInfo, err := np.GetPeerNodeInfo(nodeID)
			if err != nil {
				logging.Warnw("NodeProber.CheckMemberStatus: failed to get peer node info", "error", err)
				continue
			}
			np.nodeInfoCache[nodeID] = nodeInfo
			events = append(events, &NodeEvent{Event: NewMember, NodeInfo: nodeInfo})
			logging.Debugw("Event:NewMember", "addr", m.Address(), "node_id", nodeID, "node_name", cache.Name)
			continue
		}

		// MemberActive/MemberInActive events
		mIsActive := m.IsActive()
		if cache.Active && !mIsActive {
			cache.Active = mIsActive
			events = append(events, &NodeEvent{Event: MemberInActive, NodeInfo: cache})
			logging.Debugw("Event:MemberInActive", "addr", m.Address(), "node_id", nodeID, "node_name",
				cache.Name, "cur_name", np.cfg.ClusterNodeName(),
				"leader", np.NodeLeaderName(), "leader_id", np.NodeLeaderID(), "type", m.Type().String())
		} else if !cache.Active && mIsActive {
			cache.Active = mIsActive
			events = append(events, &NodeEvent{Event: MemberActive, NodeInfo: cache})
			logging.Debugw("Event:MemberActive", "addr", m.Address(), "node_id", nodeID, "node_name", cache.Name)
		}

		memAddr := "dns:///" + m.Address()
		if cache.GrpcEndpoint != memAddr {
			cache.GrpcEndpoint = memAddr
			events = append(events, &NodeEvent{Event: MemberAddressChanged, NodeInfo: cache})
			logging.Debugw("Event:MemberAddressChanged", "cache_addr", cache.GrpcEndpoint, "addr", memAddr, "node_id", nodeID, "node_name", cache.Name)
		}

		// MemberTypeChanged event
		if cache.Type != m.Type().String() {
			cache.Type = m.Type().String()
			events = append(events, &NodeEvent{Event: MemberTypeChanged, NodeInfo: cache})
		}
	}

	// MemberRemoved event
	for nodeID, nodeInfo := range np.nodeInfoCache {
		exist := false
		for _, m := range members {
			if m == nil {
				continue
			}
			if m.ID() == uint64(nodeID) {
				exist = true
				break
			}
		}
		if !exist {
			var clonedInfo pb.NodeInfo = *nodeInfo
			events = append(events, &NodeEvent{Event: MemberRemoved, NodeInfo: &clonedInfo})
			delete(np.nodeInfoCache, nodeID)
			np.peerClients.removeClient(nodeID)
			logging.Debugw("Event:MemberRemoved", "addr", nodeInfo.GrpcEndpoint, "node_id", nodeID, "node_name", nodeInfo.Name)
		}
	}

	return events
}

type nodeLoad struct {
	nodeID       uint32
	apiCmdAvgRPS float64
}

func (np *nodeProber) UpdateNodeLoading() {
	if !np.IsReady() {
		return
	}

	np.mu.Lock()
	defer np.mu.Unlock()

	nodeLoads := make([]*nodeLoad, 0, len(np.cfg.Cluster.PeerUrls)+1)
	// set current node avg. rps
	nodeLoads = append(nodeLoads, &nodeLoad{nodeID: np.curNodeID, apiCmdAvgRPS: np.metrics.GetAPICmdAvgRPS(avgRPSDuration)})

	// probe and set peers avg. rps
	_ = np.peerClients.iter(func(peerNodeID uint32, client cpb.ClusterClient) bool {
		if np.raftNode != nil {
			mem, ok := np.raftNode.GetMemebr(uint64(peerNodeID))
			if !ok || mem == nil || !mem.IsActive() {
				return true
			}
		}

		ctx, cancel := context.WithTimeout(np.ctx, time.Second)
		defer cancel()
		resp, err := client.GetAPICmdAvgRPS(ctx, &cpb.AvgRpsReq{LastSecs: uint32(avgRPSDuration / time.Second)})
		if err != nil || resp == nil {
			return true
		}
		nodeLoads = append(nodeLoads, &nodeLoad{nodeID: peerNodeID, apiCmdAvgRPS: resp.Value})
		return true
	})

	np.calcNodeLoading(nodeLoads)
}

func (np *nodeProber) calcNodeLoading(nodeLoads []*nodeLoad) {
	slices.SortFunc(nodeLoads, func(a, b *nodeLoad) int {
		if a.apiCmdAvgRPS < b.apiCmdAvgRPS {
			return -1
		} else if a.apiCmdAvgRPS > b.apiCmdAvgRPS {
			return 1
		}
		return 0
	})

	minLoad := nodeLoads[0].apiCmdAvgRPS
	maxLoad := nodeLoads[len(nodeLoads)-1].apiCmdAvgRPS
	sameLoadCount := 0
	for _, load := range nodeLoads {
		if load.apiCmdAvgRPS != minLoad {
			break
		}
		sameLoadCount++
	}
	logging.Debugw("nodeLoads", "len", len(nodeLoads), "sameLoadCount", sameLoadCount)
	np.lightNodeID = nodeLoads[rand.Intn(sameLoadCount)].nodeID //nolint:gosec
	np.heavyNodeID = nodeLoads[len(nodeLoads)-1].nodeID
	np.loadDiff = maxLoad - minLoad
	// var minLoad float64 = math.MaxFloat64
	// var maxLoad float64 = -1

	// for nodeID, load := range np.nodeLoading {
	// 	if nodeID == np.curNodeID {
	// 		load *= curNodeWeightFactor
	// 	}
	// 	if load < minLoad {
	// 		minLoad = load
	// 		np.lightNodeID = nodeID
	// 	} else if load > maxLoad {
	// 		maxLoad = load
	// 		np.heavyNodeID = nodeID
	// 	}
	// }
}

func (np *nodeProber) WaitAllPeerEntryApplied(entryID []byte) error {
	var clientErr error
	allApplied := true

	iterErr := np.peerClients.iter(func(peerNodeID uint32, client cpb.ClusterClient) bool {
		tctx, cancel := context.WithTimeout(np.ctx, 5*time.Second)
		defer cancel()

		ret, err := client.IsEntryApplied(tctx, &cpb.IsEntryAppliedReq{Id: entryID})
		if err != nil {
			clientErr = err
			return false
		}

		if !ret.Value {
			allApplied = false
			return false
		}

		return true
	})

	if !allApplied {
		id, _ := uuid.FromBytes(entryID)
		return fmt.Errorf("The entry %s is not be appplied from peers", id.String())
	}

	if clientErr != nil {
		return clientErr
	}

	if iterErr != nil {
		return iterErr
	}

	id, _ := uuid.FromBytes(entryID)
	logging.Debugw("WaitAllPeerEntryApplied success", "entry_id", id.String())
	return nil
}

func (np *nodeProber) SetRaftNode(raftNode *raft.Node) {
	np.raftNode = raftNode
	np.peerClients.setRaftNode(raftNode)
}

func (np *nodeProber) GetRaftNode() *raft.Node {
	return np.raftNode
}

func (np *nodeProber) CurNodeID() uint32 {
	if np.raftNode == nil {
		return 0
	}
	return np.curNodeID
}

func (np *nodeProber) NodeLeaderID() uint32 {
	if np.raftNode == nil {
		return 0
	}
	return uint32(np.raftNode.Leader())
}

func (np *nodeProber) NodeLeaderName() string {
	if np.raftNode == nil {
		return "default"
	}
	nodeID := uint32(np.raftNode.Leader())
	if info, ok := np.nodeInfoCache[nodeID]; ok {
		return info.Name
	}

	return "unknown"
}

func (np *nodeProber) IsNodeLeader() bool {
	if np.raftNode == nil {
		logging.Debugw("IsNodeLeader,  np.raftNode == nil, return true")
		return true
	}

	// logging.Debugw("IsNodeLeader", "np.raftNode.Leader()", np.raftNode.Leader(), "uint64(np.curNodeID)", uint64(np.curNodeID))
	return np.raftNode.Leader() == uint64(np.curNodeID)
}

func (np *nodeProber) GetNodeMember(nodeID uint32) raft.Member {
	if np.raftNode == nil {
		return nil
	}

	mem, ok := np.raftNode.GetMemebr(uint64(nodeID))
	if !ok {
		return nil
	}

	return mem
}

func (np *nodeProber) GetNodeMembers() []raft.Member {
	if np.raftNode == nil {
		return []raft.Member{}
	}

	return np.raftNode.Members()
}

func (np *nodeProber) GetLightestNodeID() uint32 {
	np.mu.RLock()
	defer np.mu.RUnlock()
	return np.lightNodeID
}

func (np *nodeProber) GetHeavinessNodeID() uint32 {
	np.mu.RLock()
	defer np.mu.RUnlock()
	return np.heavyNodeID
}

func (np *nodeProber) GetCurNodeInfo() (*pb.NodeInfo, error) {
	if np.raftNode == nil {
		return nil, errors.New("the raft node is nil")
	}

	np.mu.RLock()
	defer np.mu.RUnlock()

	if info, ok := np.nodeInfoCache[np.curNodeID]; ok {
		return info, nil
	}

	m, ok := np.raftNode.GetMemebr(uint64(np.curNodeID))
	if !ok {
		return nil, fmt.Errorf("Failed to get raft node member %d", np.curNodeID)
	}

	since := m.ActiveSince()
	info := &pb.NodeInfo{
		Id:           np.curNodeID,
		Name:         np.cfg.ClusterNodeName(),
		ApiEndpoint:  np.cfg.ApiAdvertiseUrl.String(),
		GrpcEndpoint: fmt.Sprintf("dns:///%s", np.cfg.Cluster.Url.Host),
		Type:         m.Type().String(),
		Leader:       np.curNodeID == uint32(np.raftNode.Leader()),
		Active:       m.IsActive(),
		ActiveSince:  pbutil.TimeToTimestamp(since),
	}

	np.nodeInfoCache[np.curNodeID] = info

	return info, nil
}

func (np *nodeProber) GetPeerNodeInfo(nodeID uint32) (*pb.NodeInfo, error) {
	if np.raftNode == nil {
		return nil, errors.New("the raft node is nil")
	}

	np.mu.RLock()
	defer np.mu.RUnlock()

	if info, ok := np.nodeInfoCache[nodeID]; ok {
		return info, nil
	}

	client := np.peerClients.getClient(nodeID)
	if client == nil {
		return nil, fmt.Errorf("Failed to get node gRPC client for node %d", nodeID)
	}

	ctx, cancel := context.WithTimeout(np.ctx, 500*time.Millisecond)
	defer cancel()
	info, err := client.GetNodeInfo(ctx, &types.Empty{})
	if err != nil {
		return nil, err
	}

	np.nodeInfoCache[nodeID] = info

	return info, err
}

func (np *nodeProber) GetNodeClient(nodeID uint32) (cpb.ClusterClient, error) {
	client := np.peerClients.getClient(nodeID)
	if client == nil {
		return nil, fmt.Errorf("Failed to get leader node gRPC client for node %d", nodeID)
	}

	return client, nil
}

func (np *nodeProber) GetLeaderNodeClient() (cpb.ClusterClient, error) {
	nodeID := np.NodeLeaderID()
	return np.GetNodeClient(nodeID)
}
