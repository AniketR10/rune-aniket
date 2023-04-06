package plugin

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// MergeResourceMap merges m1 with mn.
// If permissions are overlapping, the last of passed prevails.
func MergeResourceMap(
	m1 map[Permission]ResourceRegistrar, mn ...map[Permission]ResourceRegistrar,
) map[Permission]ResourceRegistrar {
	ret := make(map[Permission]ResourceRegistrar)
	for k, v := range m1 {
		ret[k] = v
	}
	for _, m := range mn {
		for k, v := range m {
			ret[k] = v
		}
	}
	return ret
}

func stdTimeToProto(ts time.Time) *timestamppb.Timestamp {
	seconds := ts.Unix()
	nanos := ts.Nanosecond()
	return &timestamppb.Timestamp{Seconds: seconds, Nanos: int32(nanos)}
}

func protoTimeToStd(ts *timestamppb.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
}
