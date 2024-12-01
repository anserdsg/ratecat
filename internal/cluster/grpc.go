package cluster

import (
	"context"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/cmd"
	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/anserdsg/ratecat/v1/internal/mgr"
	"github.com/gogo/protobuf/types"
	"github.com/google/uuid"
	"github.com/shaj13/raft"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cpb "github.com/anserdsg/ratecat/v1/internal/cluster/pb"
	pb "github.com/anserdsg/ratecat/v1/proto"
)

type gRPCServer struct {
	cpb.UnimplementedClusterServer
	ctx     context.Context
	cfg     *config.Config
	node    *raft.Node
	metric  *metric.Metrics
	resMgr  *mgr.ResourceManager
	cmdDisp *cmd.CmdDispatcher
}

func newGRPCServer(ctx context.Context, cfg *config.Config, node *raft.Node, mt *metric.Metrics, st *mgr.ResourceManager, disp *cmd.CmdDispatcher) *gRPCServer {
	return &gRPCServer{ctx: ctx, cfg: cfg, node: node, metric: mt, resMgr: st, cmdDisp: disp}
}

func (gs *gRPCServer) GetNodeInfo(_ context.Context, _ *types.Empty) (*pb.NodeInfo, error) {
	resp, err := gs.resMgr.GetCurNodeInfo()
	if err != nil {
		return resp, status.Error(codes.NotFound, err.Error())
	}
	return resp, nil
}

func (gs *gRPCServer) IsEntryApplied(ctx context.Context, req *cpb.IsEntryAppliedReq) (*types.BoolValue, error) {
	ch := gs.resMgr.GetEntryApplyChannel()
	gctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	reqEntryID, err := uuid.FromBytes(req.Id)
	if err != nil {
		return &types.BoolValue{Value: false}, status.Error(codes.InvalidArgument, err.Error())
	}

	for {
		select {
		case <-gctx.Done():
			goto done
		case entryID := <-ch:
			if reqEntryID == entryID {
				return &types.BoolValue{Value: true}, nil
			}
			gs.resMgr.InsertEntryApplySet(entryID)
			if gctx.Err() != nil {
				goto done
			}
		default:
			if gs.resMgr.RemoveFromEntryApplySet(reqEntryID) {
				return &types.BoolValue{Value: true}, nil
			}
		}
	}

done:
	return &types.BoolValue{Value: false}, nil
}

func (gs *gRPCServer) GetAPICmdAvgRPS(_ context.Context, req *cpb.AvgRpsReq) (*types.DoubleValue, error) {
	return &types.DoubleValue{
		Value: gs.metric.GetAPICmdAvgRPS(time.Duration(req.LastSecs) * time.Second),
	}, nil
}

func (gs *gRPCServer) RegisterResource(ctx context.Context, req *pb.RegResCmd) (*pb.RegResCmdResp, error) {
	resp, err := gs.cmdDisp.RegisterResourcePbCmd(req)
	return resp, err.StatusErr()
}

func (gs *gRPCServer) Allow(ctx context.Context, req *pb.AllowCmd) (*pb.AllowCmdResp, error) {
	resp, err := gs.cmdDisp.AllowPbCmd(req)
	return resp, err.StatusErr()
}
