package proto

import (
	context "context"
	fmt "fmt"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type loggingConn struct {
	id interface{}
	MuxConn
}

func newLoggingConn(id interface{}, c MuxConn) MuxConn {
	ret := loggingConn{id: id, MuxConn: c}
	ret.log(log.Fields{
		logging.KeyCallType: "Dial",
	})
	return ret
}

func (c loggingConn) log(extraFields log.Fields) {
	fields := log.Fields{
		logging.KeyClass: "proto.MuxConn",
		"ID":             fmt.Sprintf("%v", c.id),
		"Address":        fmt.Sprintf("%p", c.MuxConn),
	}
	for k, v := range extraFields {
		fields[k] = v
	}
	log.WithFields(fields).Trace()
}

func (c loggingConn) Invoke(
	ctx context.Context, method string, args interface{},
	reply interface{}, opts ...grpc.CallOption,
) error {
	err := c.MuxConn.Invoke(ctx, method, args, reply, opts...)
	c.log(log.Fields{
		logging.KeyCallType: "Invoke",
		"method":            method,
		logging.KeyError:    fmt.Sprintf("%v", err),
	})
	return err
}

func (c loggingConn) NewStream(
	ctx context.Context, desc *grpc.StreamDesc,
	method string, opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	stream, err := c.MuxConn.NewStream(ctx, desc, method, opts...)
	c.log(log.Fields{
		logging.KeyCallType: "NewStream",
		"method":            method,
		"name":              desc.StreamName,
		logging.KeyError:    fmt.Sprintf("%v", err),
	})
	return stream, err
}

func (c loggingConn) GetState() connectivity.State {
	state := c.MuxConn.GetState()
	c.log(log.Fields{
		logging.KeyCallType: "GetState",
		"state":             state,
	})
	return state
}

func (c loggingConn) WaitForStateChange(
	ctx context.Context, sourceState connectivity.State,
) bool {
	ok := c.MuxConn.WaitForStateChange(ctx, sourceState)
	c.log(log.Fields{
		logging.KeyCallType: "WaitForStateChange",
		"source":            sourceState,
		"ok":                ok,
	})
	return ok
}

func (c loggingConn) Close() error {
	err := c.MuxConn.Close()
	c.log(log.Fields{
		logging.KeyCallType: "Close",
		logging.KeyError:    fmt.Sprintf("%v", err),
	})
	return err
}
