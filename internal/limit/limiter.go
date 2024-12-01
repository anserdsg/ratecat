package limit

import (
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

type LimiterType int

var (
	ErrInvalidLimiter     = errors.New("invalid limiter")
	ErrUnknownLimiterType = errors.New("unknown limiter type")
	ErrOverflow           = errors.New("limiter overflow")
)

type Limiter interface {
	Allow(t time.Time, n uint32) bool
	Take(t time.Time, n uint32) (time.Duration, error)
	Type() string
	PBType() pb.LimiterType
	String() string
	ToLimiterPB() proto.Message
	FromResPB(v *pb.Resource) error
}

func NewLimiterFromResPB(resPB *pb.Resource) (Limiter, error) {
	if resPB.Limiter == nil {
		return nil, ErrInvalidLimiter
	}

	switch resPB.LimiterType {
	case pb.LimiterType_TokenBucket:
		limiter := NewTokenBucketEmptyLimiter()
		if err := limiter.FromResPB(resPB); err != nil {
			return nil, err
		}

		return limiter, nil
	default:
		return nil, ErrUnknownLimiterType
	}
}

func AssignLimiterToResPB(resPB *pb.Resource, limiter Limiter) error {
	pbType := limiter.PBType()
	resPB.LimiterType = pbType

	switch pbType {
	case pb.LimiterType_TokenBucket:
		bt, ok := limiter.ToLimiterPB().(*pb.TokenBucketLimiter)
		if !ok {
			return ErrInvalidLimiter
		}
		resPB.Limiter = &pb.Resource_TokenBucket{TokenBucket: bt}
	default:
		return ErrUnknownLimiterType
	}

	return nil
}
