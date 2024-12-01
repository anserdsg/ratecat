package probe

import (
	"errors"

	"github.com/shaj13/raft"

	cpb "github.com/anserdsg/ratecat/v1/internal/cluster/pb"
	pb "github.com/anserdsg/ratecat/v1/proto"
)

type NodeProber interface {
	Init() bool
	IsReady() bool
	CheckMemberStatus() []*NodeEvent
	UpdateNodeLoading()
	WaitAllPeerEntryApplied(entryID []byte) error
	SetRaftNode(raftNode *raft.Node)
	GetRaftNode() *raft.Node
	CurNodeID() uint32
	NodeLeaderID() uint32
	IsNodeLeader() bool
	GetNodeMember(nodeID uint32) raft.Member
	GetNodeMembers() []raft.Member
	GetLightestNodeID() uint32
	GetHeavinessNodeID() uint32
	GetCurNodeInfo() (*pb.NodeInfo, error)
	GetPeerNodeInfo(nodeID uint32) (*pb.NodeInfo, error)
	GetNodeClient(nodeID uint32) (cpb.ClusterClient, error)
	GetLeaderNodeClient() (cpb.ClusterClient, error)
}

type nullProber struct{}

var errUnimpl = errors.New("unimplemented")

func (n *nullProber) Init() bool                                             { return true }
func (n *nullProber) IsReady() bool                                          { return true }
func (n *nullProber) CheckMemberStatus() []*NodeEvent                        { return []*NodeEvent{} }
func (n *nullProber) UpdateNodeLoading()                                     {}
func (n *nullProber) WaitAllPeerEntryApplied(entryID []byte) error           { return nil }
func (n *nullProber) SetRaftNode(raftNode *raft.Node)                        {}
func (n *nullProber) GetRaftNode() *raft.Node                                { return nil }
func (n *nullProber) CurNodeID() uint32                                      { return 0 }
func (n *nullProber) NodeLeaderID() uint32                                   { return 0 }
func (n *nullProber) IsNodeLeader() bool                                     { return true }
func (n *nullProber) GetNodeMember(nodeID uint32) raft.Member                { return nil }
func (n *nullProber) GetNodeMembers() []raft.Member                          { return []raft.Member{} }
func (n *nullProber) GetLightestNodeID() uint32                              { return 0 }
func (n *nullProber) GetHeavinessNodeID() uint32                             { return 0 }
func (n *nullProber) GetCurNodeInfo() (*pb.NodeInfo, error)                  { return nil, errUnimpl }
func (n *nullProber) GetPeerNodeInfo(nodeID uint32) (*pb.NodeInfo, error)    { return nil, errUnimpl }
func (n *nullProber) GetNodeClient(nodeID uint32) (cpb.ClusterClient, error) { return nil, errUnimpl }
func (n *nullProber) GetLeaderNodeClient() (cpb.ClusterClient, error)        { return nil, errUnimpl }

func NewNullNodeProber() NodeProber {
	return &nullProber{}
}
