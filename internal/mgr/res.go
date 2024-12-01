package mgr

import (
	"time"

	"github.com/anserdsg/ratecat/v1/internal/limit"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/anserdsg/ratecat/v1/internal/pbutil"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

type Resource struct {
	NodeID      uint32
	Limiter     limit.Limiter
	LastUpdated time.Time
	RPS         metric.RPS
}

func newResource(nodeID uint32, limiter limit.Limiter) *Resource {
	return &Resource{NodeID: nodeID, Limiter: limiter, LastUpdated: time.Now()}
}

func newResourceFromPB(resPB *pb.Resource) (*Resource, error) {
	limiter, err := limit.NewLimiterFromResPB(resPB)
	if err != nil {
		logging.Errorw("Failed to create resource limiter from Resource protobuf message", "error", err)
		return nil, err
	}
	res := &Resource{NodeID: resPB.NodeId, Limiter: limiter, LastUpdated: pbutil.TimestampToTime(resPB.LastUpdated)}
	res.RPS.FromPB(resPB.Rps)

	return res, nil
}

func (res *Resource) ToPB(name string) (*pb.Resource, error) {
	resPB := &pb.Resource{
		Name:        name,
		NodeId:      res.NodeID,
		Rps:         res.RPS.ToPB(),
		LastUpdated: pbutil.TimeToTimestamp(res.LastUpdated),
	}
	if res.Limiter == nil {
		return resPB, nil
	}

	if err := limit.AssignLimiterToResPB(resPB, res.Limiter); err != nil {
		return resPB, err
	}

	return resPB, nil
}
