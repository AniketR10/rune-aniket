package rpc

import (
	context "context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type loggingConn struct {
	id interface{}
	MuxConn
	tags []string
}

func newLoggingConn(id string, c MuxConn, tags ...string) MuxConn {
	ret := loggingConn{id: id, MuxConn: c, tags: tags}
	ret.log(log.Fields{
		logging.KeyCallType: "Dial",
	})
	/* uncomment to debug leaks
	dir := os.TempDir()
	filename := "DEBUGMUXCONN" + strings.Join(append(tags, strconv.Itoa(rand.Int())), "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	ret.tempfile = filepath.Join(dir, filename)
	err := os.WriteFile(ret.tempfile, []byte(filename), 0777)
	if err != nil {
		panic(err)
	}
	*/
	return ret
}

func (c loggingConn) log(extraFields log.Fields) {
	fields := log.Fields{
		logging.KeyClass: "rpc.MuxConn",
		"ID":             fmt.Sprintf("%v", c.id),
		"Address":        fmt.Sprintf("%p", c.MuxConn),
		"Tags":           fmt.Sprintf("%v", c.tags),
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
	/* uncomment to debug leaks
	os.Remove(c.tempfile)
	*/
	return err
}
