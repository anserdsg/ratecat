package mgr

import (
	"github.com/google/uuid"

	cpb "github.com/anserdsg/ratecat/v1/internal/cluster/pb"
	"github.com/anserdsg/ratecat/v1/internal/pbutil"
	pb "github.com/anserdsg/ratecat/v1/proto"
)

func newEntryPB(name string, action cpb.EntryAction, srcNodeID uint32, nodeID uint32) *cpb.Entry {
	return &cpb.Entry{
		Id:        generateEntryID(),
		Action:    action,
		SrcNodeId: srcNodeID,
		Resource:  newEmptyResourcePB(name, nodeID),
	}
}

func newEntryPBWithRes(name string, action cpb.EntryAction, srcNodeID uint32, res *Resource) (*cpb.Entry, error) {
	entryPB := &cpb.Entry{
		Id:        generateEntryID(),
		Action:    action,
		SrcNodeId: srcNodeID,
	}
	if res != nil {
		resPB, err := res.ToPB(name)
		if err != nil {
			return nil, err
		}
		entryPB.Resource = resPB
	} else {
		entryPB.Resource = newEmptyResourcePB(name, 0)
	}

	return entryPB, nil
}

func newEmptyResourcePB(name string, nodeID uint32) *pb.Resource {
	return &pb.Resource{
		Name:        name,
		NodeId:      nodeID,
		LimiterType: pb.LimiterType_Null,
		LastUpdated: pbutil.NowTimestamp(),
	}
}

func generateEntryID() []byte {
	var id uuid.UUID
	var err error
	for {
		id, err = uuid.NewRandom()
		if err == nil {
			break
		}
	}

	return id[:]
}
