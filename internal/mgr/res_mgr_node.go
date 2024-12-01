package mgr

import (
	"github.com/google/uuid"
	"github.com/shaj13/raft"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

func (rs *ResourceManager) CurNodeID() uint32 {
	return rs.nodeProber.CurNodeID()
}

func (rs *ResourceManager) CurNodeName() string {
	if !rs.cfg.Cluster.Enabled {
		return "default"
	}

	return rs.cfg.ClusterNodeName()
}

func (rs *ResourceManager) AtSameNode(nodeID uint32) bool {
	return nodeID == rs.nodeProber.CurNodeID()
}

func (rs *ResourceManager) NodeLeaderID() uint32 {
	return rs.nodeProber.NodeLeaderID()
}

func (rs *ResourceManager) IsNodeLeader() bool {
	return rs.nodeProber.IsNodeLeader()
}

func (rs *ResourceManager) GetNodeMembers() []raft.Member {
	return rs.nodeProber.GetNodeMembers()
}

func (rs *ResourceManager) GetCurNodeInfo() (*pb.NodeInfo, error) {
	return rs.nodeProber.GetCurNodeInfo()
}

func (rs *ResourceManager) GetPeerNodeInfo(nodeID uint32) (*pb.NodeInfo, error) {
	return rs.nodeProber.GetPeerNodeInfo(nodeID)
}

func (rs *ResourceManager) GetEntryApplyChannel() chan uuid.UUID {
	return rs.entryApplyChan
}

func (rs *ResourceManager) InsertEntryApplySet(entryID uuid.UUID) {
	rs.amu.Lock()
	defer rs.amu.Unlock()
	_ = rs.entryApplySet.Insert(entryID)
}

func (rs *ResourceManager) RemoveFromEntryApplySet(entryID uuid.UUID) bool {
	rs.amu.Lock()
	defer rs.amu.Unlock()
	return rs.entryApplySet.Remove(entryID)
}

func (rs *ResourceManager) ForwardAllowCmd(cmd *pb.AllowCmd, nodeID uint32) (*pb.AllowCmdResp, error) {
	client, err := rs.nodeProber.GetNodeClient(nodeID)
	if err != nil {
		return nil, err
	}
	return client.Allow(rs.ctx, cmd)
}

func (rs *ResourceManager) ForwardRegResCmd(cmd *pb.RegResCmd) (*pb.RegResCmdResp, error) {
	client, err := rs.nodeProber.GetLeaderNodeClient()
	if err != nil {
		return nil, err
	}
	return client.RegisterResource(rs.ctx, cmd)
}

func (rs *ResourceManager) UpdateNodeLoading() {
	if rs.nodeProber != nil {
		rs.nodeProber.UpdateNodeLoading()
	}
}
