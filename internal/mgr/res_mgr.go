package mgr

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/cluster/probe"
	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/limit"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/dolthub/swiss"
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set"

	cpb "github.com/anserdsg/ratecat/v1/internal/cluster/pb"
)

const defResourceSize uint32 = 64

type ResourceManager struct {
	ctx           context.Context
	cfg           *config.Config
	mu            sync.Mutex
	store         *swiss.Map[string, *Resource]
	nodeStore     map[uint32]*swiss.Map[string, *Resource]
	nodeProber    probe.NodeProber
	ctxCancelFunc context.CancelFunc

	ready          atomic.Bool
	amu            sync.Mutex
	entryApplySet  *set.Set[uuid.UUID]
	entryApplyChan chan uuid.UUID
}

func NewResourceManager(ctx context.Context, cfg *config.Config, np probe.NodeProber) *ResourceManager {
	rctx, cancel := context.WithCancel(ctx)
	rs := &ResourceManager{
		ctx:            rctx,
		ctxCancelFunc:  cancel,
		cfg:            cfg,
		store:          swiss.NewMap[string, *Resource](defResourceSize),
		nodeStore:      make(map[uint32]*swiss.Map[string, *Resource]),
		nodeProber:     np,
		entryApplySet:  set.New[uuid.UUID](16),
		entryApplyChan: make(chan uuid.UUID, 1024),
	}
	if np == nil {
		rs.nodeProber = probe.NewNullNodeProber()
	}

	return rs
}

func (rs *ResourceManager) StartAsync() {
	logging.Debugw("ResourceManager.StartAsync", "check_interval", rs.cfg.Cluster.CheckInterval)
	go func() {
		// nodeStatusTicker := time.NewTicker(rs.cfg.Cluster.CheckInterval / 10)
		checkTicker := time.NewTicker(rs.cfg.Cluster.CheckInterval)
		defer checkTicker.Stop()
		for {
			select {
			// case <-nodeStatusTicker.C:
			// 	if !rs.IsReady() || !rs.nodeProber.IsNodeLeader() {
			// 		continue
			// 	}
			// 	events := rs.nodeProber.CheckMemberStatus()
			// 	rs.checkResources(events)

			// case <-checkTicker.C:
			// 	if !rs.IsReady() || !rs.nodeProber.IsNodeLeader() {
			// 		continue
			// 	}
			// 	rs.nodeProber.UpdateNodeLoading()

			case <-rs.ctx.Done():
				checkTicker.Stop()
				logging.Debugw("ResourceManager.StartAsync() receives context done", "node_id", rs.CurNodeID(), "node_name", rs.CurNodeName())
				return
			}
		}
	}()
}

func (rs *ResourceManager) Stop() {
	rs.ready.Store(false)
	rs.ctxCancelFunc()
}

func (rs *ResourceManager) IsReady() bool {
	return rs.ready.Load()
}

func (rs *ResourceManager) Ready() {
	for i := 0; i < 20; i++ {
		if rs.nodeProber.Init() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	rs.ready.Store(true)
}

func (rs *ResourceManager) PutResource(name string, limiter limit.Limiter) (*Resource, error) {
	// standalone mode
	if !rs.cfg.Cluster.Enabled {
		rs.mu.Lock()
		res := newResource(rs.CurNodeID(), limiter)
		rs.store.Put(name, res)
		rs.mu.Unlock()
		return res, nil
	}

	nodeID := rs.nodeProber.GetLightestNodeID()
	res := newResource(nodeID, limiter)

	entryPB, err := newEntryPBWithRes(name, cpb.EntryAction_PutResource, rs.CurNodeID(), res)
	if err != nil {
		return res, err
	}

	err = rs.replicateEntryPB(entryPB)
	return res, err
}

func (rs *ResourceManager) PutResourceToNode(name string, limiter limit.Limiter, nodeID uint32) (*Resource, error) {
	// standalone mode
	if !rs.cfg.Cluster.Enabled {
		rs.mu.Lock()
		res := newResource(rs.CurNodeID(), limiter)
		rs.store.Put(name, res)
		rs.mu.Unlock()
		return res, nil
	}

	res := newResource(nodeID, limiter)
	entryPB, err := newEntryPBWithRes(name, cpb.EntryAction_PutResource, rs.CurNodeID(), res)
	if err != nil {
		return res, err
	}

	err = rs.replicateEntryPB(entryPB)
	return res, err
}

func (rs *ResourceManager) putResource(name string, res *Resource) {
	rs.store.Put(name, res)
	if _, ok := rs.nodeStore[res.NodeID]; !ok {
		rs.nodeStore[res.NodeID] = swiss.NewMap[string, *Resource](defResourceSize)
	}
	rs.nodeStore[res.NodeID].Put(name, res)
}

func (rs *ResourceManager) RemoveResource(name string) error {
	// standalone mode
	if !rs.cfg.Cluster.Enabled {
		rs.mu.Lock()
		rs.store.Delete(name)
		rs.mu.Unlock()
		return nil
	}

	entryPB, err := newEntryPBWithRes(name, cpb.EntryAction_RemoveResource, rs.CurNodeID(), nil)
	if err != nil {
		return err
	}

	return rs.replicateEntryPB(entryPB)
}

func (rs *ResourceManager) removeResource(name string) {
	res, ok := rs.store.Get(name)
	if ok {
		resMap := rs.nodeStore[res.NodeID]
		if resMap != nil {
			resMap.Delete(name)
		}
		rs.store.Delete(name)
	}
}

func (rs *ResourceManager) UpdateResourceNodeId(name string, nodeID uint32) error {
	// standalone mode
	if !rs.cfg.Cluster.Enabled {
		rs.mu.Lock()
		rs.updateResourceNodeId(name, nodeID)
		rs.mu.Unlock()
		return nil
	}

	entryPB := newEntryPB(name, cpb.EntryAction_UpdateResourceNodeId, rs.CurNodeID(), nodeID)

	return rs.replicateEntryPB(entryPB)
}

func (rs *ResourceManager) updateResourceNodeId(name string, dstNodeID uint32) {
	res, ok := rs.store.Get(name)
	if !ok || res == nil {
		logging.Infow("update res node id doesn't exist", "name", name)
		return
	}

	if res.NodeID == dstNodeID {
		return
	}

	res.NodeID = dstNodeID

	srcResMap, ok := rs.nodeStore[res.NodeID]
	if !ok || srcResMap == nil {
		return
	}
	if _, ok = rs.nodeStore[dstNodeID]; !ok {
		rs.nodeStore[dstNodeID] = swiss.NewMap[string, *Resource](defResourceSize)
	}
	dstResMap := rs.nodeStore[dstNodeID]

	dstResMap.Put(name, res)
	srcResMap.Delete(name)
}

func (rs *ResourceManager) GetResource(name string) (*Resource, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// if rs.cfg.Cluster.Enabled {
	// 	ctx, cancel := context.WithTimeout(rs.ctx, time.Second)
	// 	defer cancel()

	// 	node := rs.nodeProber.GetRaftNode()
	// 	if err := node.LinearizableRead(ctx); err != nil {
	// 		return nil, false
	// 	}
	// }

	return rs.store.Get(name)
}

func (rs *ResourceManager) HasResource(name string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	return rs.store.Has(name)
}

func (rs *ResourceManager) RebalanceResources() {
	// TODO
}

func (rs *ResourceManager) checkResources(events []*probe.NodeEvent) {
	for _, evt := range events {
		//nolint:exhaustive
		switch evt.Event {
		case probe.NewMember, probe.MemberActive:
			rs.RebalanceResources()

			// case probe.MemberInActive, probe.MemberRemoved:
			// 	rs.moveNodeResources(evt.NodeInfo.Id)
		}
	}
}

func (rs *ResourceManager) moveNodeResources(srcNodeID uint32) {
	var dstNodeID uint32

	timeout := true
	for i := 0; i < 10; i++ {
		rs.nodeProber.UpdateNodeLoading()
		dstNodeID = rs.nodeProber.GetLightestNodeID()
		if srcNodeID == dstNodeID {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		timeout = false
		break
	}
	if timeout {
		logging.Warnw("Move node resource timed out", "node_id", srcNodeID)
		return
	}

	rs.mu.Lock()
	srcResMap, ok := rs.nodeStore[srcNodeID]
	// do nothing if no resource in source store
	if !ok || srcResMap == nil {
		rs.mu.Unlock()
		return
	}

	entryPBs := make([]*cpb.Entry, 0, srcResMap.Count())
	srcResMap.Iter(func(name string, res *Resource) bool {
		logging.Debugw("Move node resource", "count", srcResMap.Count(), "srcNodeID", srcNodeID, "dstNodeID", dstNodeID, "resource", name)
		entryPB := newEntryPB(name, cpb.EntryAction_UpdateResourceNodeId, srcNodeID, dstNodeID)
		entryPBs = append(entryPBs, entryPB)
		return false
	})
	rs.mu.Unlock()

	for _, entryPB := range entryPBs {
		_ = rs.replicateEntryPB(entryPB)
	}
}

// func (rs *ResourceManager) updateResourceLimiter(name string, nodeID uint32) {
// 	res, ok := rs.store.Get(name)
// 	if ok && res != nil {
// 		res.NodeID = nodeID
// 	}
// }

func (rs *ResourceManager) replicateEntryPB(entryPB *cpb.Entry) error {
	buf, err := proto.Marshal(entryPB)
	if err != nil {
		return err
	}

	entryID, _ := uuid.FromBytes(entryPB.Id)
	// logging.Debugw("Send replicate command",
	// 	"entry_id", entryID.String(),
	// 	"resource_name", entryPB.Resource.Name,
	// 	"assigned_node_id", entryPB.Resource.NodeId,
	// 	"cur_node_name", rs.CurNodeName(),
	// 	"cur_node_id", rs.CurNodeID(),
	// 	"is_leader", rs.IsNodeLeader(),
	// )

	raftNode := rs.nodeProber.GetRaftNode()
	if raftNode == nil {
		return errors.New("Raft node didn't set in node prober")
	}

	ctx, cancel := context.WithTimeout(rs.ctx, 10*time.Second)
	defer cancel()

	err = raftNode.Replicate(ctx, buf)
	if err != nil {
		logging.Errorw("Failed to send replicate command", "entryID", entryID, "error", err)
		return fmt.Errorf("Failed to send replicate command, error: %w", err)
	}

	// wait for current node to receive and apply the resource entry.
	// rs.waitEntryApplied(entryID)

	// logging.Debugw("Wait all peers apply entries",
	// 	"entry_id", entryID.String(),
	// 	"resource_name", entryPB.Resource.Name,
	// 	"assigned_node_id", entryPB.Resource.NodeId,
	// 	"cur_node_name", rs.CurNodeName(),
	// 	"cur_node_id", rs.CurNodeID(),
	// 	"is_leader", rs.IsNodeLeader(),
	// )
	// // wait for all peer nodes to receive and apply the resource entry.
	// err = rs.nodeProber.WaitAllPeerEntryApplied(entryPB.Id)
	// if err != nil {
	// 	return fmt.Errorf("Failed to wait all peers apply entries, error: %w", err)
	// }

	return nil
}

func (rs *ResourceManager) waitEntryApplied(entryID uuid.UUID) {
	gctx, cancel := context.WithTimeout(rs.ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-gctx.Done():
			logging.Warnw("Wait entry applied timed out", "entry_id", entryID)
			return
		case id := <-rs.entryApplyChan:
			if id == entryID {
				return
			}
			logging.Warnw("Wait entry applied got  id", "expect", entryID, "got", id)
		}
	}
}
