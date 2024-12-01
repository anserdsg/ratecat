package probe

import (
	pb "github.com/anserdsg/ratecat/v1/proto"
)

type NodeEventType int

const (
	NewMember            NodeEventType = 1
	MemberActive         NodeEventType = 2
	MemberInActive       NodeEventType = 3
	MemberRemoved        NodeEventType = 4
	MemberTypeChanged    NodeEventType = 5
	MemberAddressChanged NodeEventType = 6
)

type NodeEvent struct {
	Event    NodeEventType
	NodeInfo *pb.NodeInfo
}
