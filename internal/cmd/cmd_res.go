package cmd

import (
	"github.com/anserdsg/ratecat/v1/internal/limit"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/gogo/protobuf/proto"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

func (c *CmdDispatcher) RegisterResource(body []byte) (proto.Message, *pb.CmdError) {
	cmd := &pb.RegResCmd{}
	if err := proto.Unmarshal(body, cmd); err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, err)
	}

	// if !c.resMgr.IsNodeLeader() {
	// 	logging.Debugw("CMD RegRes Forward", "cmd", cmd)
	// 	resp, err := c.resMgr.ForwardRegResCmd(cmd)
	// 	if err != nil {
	// 		return nil, pb.WrapCmdError(pb.ErrorCode_ForwardFail, err)
	// 	}
	// 	return resp, nil
	// }

	logging.Debugw("CMD RegRes", "cmd", cmd)
	return c.RegisterResourcePbCmd(cmd)
}

func (c *CmdDispatcher) RegisterResourcePbCmd(cmd *pb.RegResCmd) (*pb.RegResCmdResp, *pb.CmdError) {
	if !cmd.Override {
		if res, ok := c.resMgr.GetResource(cmd.Resource); ok {
			return &pb.RegResCmdResp{NodeId: res.NodeID}, nil
		}
	}

	var limiter limit.Limiter
	switch cmd.Type {
	case pb.LimiterType_TokenBucket:
		params := cmd.GetTokenBucket()
		if params == nil {
			return nil, nil
		}
		limiter = limit.NewTokenBucketLimiter(params.Rate, params.Burst)
	default:
		return nil, pb.NewCmdInvalidLimiterError("Unknown limiter type")
	}

	res, err := c.resMgr.PutResource(cmd.Resource, limiter)
	if err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, err)
	}

	return &pb.RegResCmdResp{NodeId: res.NodeID}, nil
}

func (c *CmdDispatcher) GetResource(body []byte) (proto.Message, *pb.CmdError) {
	cmd := &pb.GetResCmd{}
	if err := proto.Unmarshal(body, cmd); err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, err)
	}

	logging.Debugw("CMD GetRes", "cmd", cmd)
	return c.GetResourcePbCmd(cmd)
}

func (c *CmdDispatcher) GetResourcePbCmd(cmd *pb.GetResCmd) (*pb.Resource, *pb.CmdError) {
	res, ok := c.resMgr.GetResource(cmd.Resource)
	if !ok || res == nil {
		return nil, pb.NewCmdNotFoundError("Failed to get resource: %s", cmd.Resource)
	}

	respMsg, err := res.ToPB(cmd.Resource)
	if err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_DecodeFail, err)
	}

	return respMsg, nil
}
