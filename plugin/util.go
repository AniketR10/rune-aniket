package plugin

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"unstable.build/go-tui/proto"
)

// MergeResourceMap merges m1 with mn.
// If permissions are overlapping, the last of passed prevails.
func MergeResourceMap(
	m1 map[Permission]ResourceServer, mn ...map[Permission]ResourceServer,
) map[Permission]ResourceServer {
	ret := make(map[Permission]ResourceServer)
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

func acceptAndServe(
	broker proto.MuxBroker, ID uint32,
	srv func(opts []grpc.ServerOption) proto.MuxServer,
) error {
	lis, err := broker.Accept(ID)
	if err != nil {
		return err
	}
	server := srv([]grpc.ServerOption{})
	go server.Serve(lis)
	return nil
}

func stdTimeToProto(ts time.Time) *timestamppb.Timestamp {
	seconds := ts.Unix()
	nanos := ts.Nanosecond()
	return &timestamppb.Timestamp{Seconds: seconds, Nanos: int32(nanos)}
}

func protoTimeToStd(ts *timestamppb.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
}
