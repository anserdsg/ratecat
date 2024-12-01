package cmd

import (
	"time"

	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/gogo/protobuf/proto"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

func (c *CmdDispatcher) Allow(body []byte) (proto.Message, *pb.CmdError) {
	cmd := &pb.AllowCmd{}
	if err := proto.Unmarshal(body, cmd); err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, err)
	}

	return c.AllowPbCmd(cmd)
}

func (c *CmdDispatcher) AllowPbCmd(cmd *pb.AllowCmd) (*pb.AllowCmdResp, *pb.CmdError) {
	if len(cmd.Resource) < 1 {
		return nil, pb.NewCmdInvalidMsgError("Empty resource name")
	}

	res, ok := c.resMgr.GetResource(cmd.Resource)
	if !ok || res == nil {
		return nil, pb.NewCmdNotFoundError("The resource %s doesn't exist", cmd.Resource)
	}

	logging.Debugw("CmdDispatcher.Allow()", "res_name", cmd.Resource, "res_node_id", res.NodeID, "cur_node_id", c.resMgr.CurNodeID())

	if cmd.Events == 0 {
		cmd.Events = 1
	}

	if !c.resMgr.AtSameNode(res.NodeID) {
		logging.Debugw("CmdDispatcher.Allow() redirected", "node_id", res.NodeID, "cur_node_id", c.resMgr.CurNodeID(), "res_name", cmd.Resource)
		respMsg, err := c.resMgr.ForwardAllowCmd(cmd, res.NodeID)
		if err != nil {
			return nil, pb.WrapCmdError(pb.ErrorCode_ForwardFail, err)
		}
		respMsg.Redirected = true
		respMsg.NodeId = res.NodeID
		return respMsg, nil
	}

	ok = res.Limiter.Allow(time.Now(), cmd.Events)
	return &pb.AllowCmdResp{NodeId: res.NodeID, Ok: ok}, nil
}
