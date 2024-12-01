package mgr

import (
	"bytes"
	"errors"
	"io"

	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"

	cpb "github.com/anserdsg/ratecat/v1/internal/cluster/pb"
)

func (rs *ResourceManager) Apply(data []byte) {
	entryPB := &cpb.Entry{}

	err := proto.Unmarshal(data, entryPB)
	if err != nil {
		logging.Errorw("Failed to apply raft entry", "error", err)
		return
	}

	entryID, err := uuid.FromBytes(entryPB.Id)
	if err != nil {
		logging.Errorw("Failed to apply raft entry", "error", err)
		return
	}

	logging.Debugw("Apply resource",
		"action", entryPB.Action.String(),
		"entry_id", entryID.String(),
		"cur_node_name", rs.CurNodeName(),
		"cur_node_id", rs.CurNodeID(),
		"src_node_id", entryPB.SrcNodeId,
		"res_name", entryPB.Resource.Name,
		"res_node_id", entryPB.Resource.NodeId,
	)

	rs.mu.Lock()
	defer rs.mu.Unlock()

	switch entryPB.Action {
	case cpb.EntryAction_PutResource:
		res, err := newResourceFromPB(entryPB.Resource)
		if err != nil {
			logging.Errorw("Failed to apply new resource", "error", err)
			return
		}
		rs.putResource(entryPB.Resource.Name, res)

	case cpb.EntryAction_RemoveResource:
		rs.removeResource(entryPB.Resource.Name)

	case cpb.EntryAction_UpdateResourceNodeId:
		rs.updateResourceNodeId(entryPB.Resource.Name, entryPB.Resource.NodeId)

	case cpb.EntryAction_UpdateResourceLimiter:
		// TODO
	}

	// only push entry id into channel when ratecat is ready(not in WAL init)
	// if rs.IsReady() {
	// 	rs.entryApplyChan <- entryID
	// }

	logging.Debugw("Apply resource FINISHED",
		"action", entryPB.Action.String(),
		"entry_id", entryID.String(),
		"cur_node_name", rs.CurNodeName(),
		"cur_node_id", rs.CurNodeID(),
		"src_node_id", entryPB.SrcNodeId,
		"res_name", entryPB.Resource.Name,
		"res_node_id", entryPB.Resource.NodeId,
	)
}

func (rs *ResourceManager) Snapshot() (io.ReadCloser, error) {
	logging.Debug("Take snapshot")
	snaphotPB, err := rs.createSnapshotPB()
	if err != nil {
		logging.Errorw("Failed to create snapshot", "error", err)
		return nil, err
	}

	buf, err := proto.Marshal(snaphotPB)
	if err != nil {
		logging.Errorw("Failed to marshal snapshot protobuf message", "error", err)
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(buf)), nil
}

func (rs *ResourceManager) Restore(r io.ReadCloser) error {
	logging.Debug("Restore from snapshot")

	buf, err := io.ReadAll(r)
	if err != nil {
		logging.Errorw("Failed to read snapshot protobuf message", "error", err)
		return err
	}

	snaphotPB := &cpb.EntrySnapshot{}
	err = proto.Unmarshal(buf, snaphotPB)
	if err != nil {
		logging.Errorw("Failed to unmarshal snapshot protobuf message", "error", err)
		return err
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, entryPB := range snaphotPB.Entries {
		if entryPB == nil || entryPB.Resource == nil {
			logging.Error("Failed to restore resource, resource in entry is empty")
			return errors.New("Failed to restore resource, resource in entry is empty")
		}

		res, err := newResourceFromPB(entryPB.Resource)
		if err != nil {
			logging.Errorw("Failed to restore resource", "name", entryPB.Resource.Name, "error", err)
			return err
		}

		rs.putResource(entryPB.Resource.Name, res)
	}

	return nil
}

func (rs *ResourceManager) createSnapshotPB() (*cpb.EntrySnapshot, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	snaphotPB := &cpb.EntrySnapshot{Entries: make([]*cpb.Entry, 0, rs.store.Count())}

	var iterErr error
	var entryPB *cpb.Entry
	srcNodeID := rs.CurNodeID()
	rs.store.Iter(func(name string, res *Resource) bool {
		entryPB, iterErr = newEntryPBWithRes(name, cpb.EntryAction_PutResource, srcNodeID, res)
		if iterErr != nil {
			return true // break
		}
		snaphotPB.Entries = append(snaphotPB.Entries, entryPB)
		return false // continue
	})

	if iterErr != nil {
		return nil, iterErr
	}

	return snaphotPB, nil
}

//nolint:unused
func (rs *ResourceManager) updateResourceByEntryPB(entryPB *cpb.Entry) bool {
	res, ok := rs.store.Get(entryPB.Resource.Name)
	if !ok {
		return false
	}
	if res.Limiter.PBType() != entryPB.Resource.LimiterType {
		return false
	}

	// skip updating and report the process of updating is done if one of conditions are met:
	// 1. the source node of entry message is the current node, it set by Put method already
	// 2. the resource is handled by the current node.
	if rs.AtSameNode(entryPB.SrcNodeId) || rs.AtSameNode(entryPB.Resource.NodeId) {
		logging.Debugw("Skip update resource", "cur_node_id", rs.CurNodeID(), "src_node_id", entryPB.SrcNodeId, "node_id", entryPB.Resource.NodeId)
		return true
	}

	err := res.Limiter.FromResPB(entryPB.Resource)
	if err != nil {
		logging.Errorw("Failed to update existing limiter from resource protobuf message", "error", err)
		return false
	}
	return true
}
