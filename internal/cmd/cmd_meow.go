package cmd

import (
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/pbutil"
	"github.com/gogo/protobuf/proto"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

func (c *CmdDispatcher) Meow(body []byte) (proto.Message, *pb.CmdError) {
	resp := &pb.MeowCmdResp{NodeId: c.resMgr.CurNodeID()}

	if !c.cfg.Cluster.Enabled {
		nodeInfo := &pb.NodeInfo{
			Id:          0,
			ApiEndpoint: c.cfg.ApiAdvertiseUrl.String(),
			Leader:      true,
			Active:      true,
			ActiveSince: pbutil.TimeToTimestamp(c.initTime),
		}
		resp.Nodes = append(resp.Nodes, nodeInfo)

		return resp, nil
	}

	members := c.resMgr.GetNodeMembers()
	for _, m := range members {
		var nodeInfo *pb.NodeInfo
		var err error

		id := uint32(m.ID())
		if c.resMgr.AtSameNode(id) {
			nodeInfo, err = c.resMgr.GetCurNodeInfo()
		} else {
			nodeInfo, err = c.resMgr.GetPeerNodeInfo(id)
		}
		if err != nil {
			logging.Warnw("Failed to get node info", "id", id, "error", err)
			continue
		}
		resp.Nodes = append(resp.Nodes, nodeInfo)
	}

	return resp, nil
}
