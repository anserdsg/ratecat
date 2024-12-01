package pbutil

import (
	"time"

	"github.com/gogo/protobuf/types"
)

func TimeToTimestamp(t time.Time) *types.Timestamp {
	return &types.Timestamp{Seconds: t.Unix(), Nanos: int32(t.Nanosecond())}
}

func TimestampToTime(ts *types.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos())).UTC()
}

func NowTimestamp() *types.Timestamp {
	return TimeToTimestamp(time.Now())
}
