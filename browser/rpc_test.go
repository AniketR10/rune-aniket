package browser

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type brokerage struct {
	net.Listener
	*grpc.Server
}

// satisfies to MuxBroker
type dialBroker struct {
	mu    sync.Mutex
	id    uint32
	conns map[uint32]brokerage
}

func newDialBroker() *dialBroker {
	ret := new(dialBroker)
	ret.conns = make(map[uint32]brokerage)
	return ret
}

func (t *dialBroker) NextId() uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.id++
	return t.id
}

func (t *dialBroker) AcceptAndServe(
	ID uint32, srv func(opts []grpc.ServerOption) *grpc.Server,
) {
	t.mu.Lock()
	defer t.mu.Unlock()

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	server := srv([]grpc.ServerOption{})
	go server.Serve(lis)

	t.conns[ID] = brokerage{Listener: lis, Server: server}
}

func (t *dialBroker) Dial(ID uint32) (conn *grpc.ClientConn, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	br, ok := t.conns[ID]
	if !ok {
		return nil, fmt.Errorf("connection with id %d not found", ID)
	}
	return grpc.Dial(br.Listener.Addr().String(), grpc.WithInsecure())
}

func (t *dialBroker) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, br := range t.conns {
		br.Server.Stop()
	}
	t.conns = nil
	return nil
}

// binds together a client with the remote browser, so we can verify
type testClient struct {
	*Client
	tui.Handler
}

func TestRPCBrowserDraw(t *testing.T) {
	var closeFn func()

	testBrowserHandlerDraw(t, func(ed editor.Editor, opts ...Option) (browserInternal, error) {
		b := newTestBrowserHandler()
		err := b.Init(ed, opts...)
		if err != nil {
			return nil, err
		}

		lis, err := net.Listen("tcp", ":0")
		require.NoError(t, err)

		broker := newDialBroker()

		grpcServer := grpc.NewServer()
		server := NewServer(broker, b, new(sync.Mutex))
		proto.RegisterWindowManagerServer(grpcServer, server)
		proto.RegisterMessengerServer(grpcServer, server)
		proto.RegisterKeyMapperServer(grpcServer, server)
		proto.RegisterFileOpenerServer(grpcServer, server)

		go grpcServer.Serve(lis)

		conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
		require.NoError(t, err)

		client := testClient{
			Client:  NewClient(broker, conn),
			Handler: b,
		}
		closeFn = func() {
			client.Close()
			server.Close()
			grpcServer.Stop()
			broker.Close()
		}
		return client, nil
	})

	closeFn()
}
