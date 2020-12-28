package proto

import (
	fmt "fmt"
	"net"
	"sync"

	grpc "google.golang.org/grpc"
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

// NewDialBroker is an integration test MuxBroker which under the hood
// dials and serves real grpc connections over an insecure transport.
func NewDialBroker() MuxBroker {
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

func (t *dialBroker) Dial(ID uint32) (conn MuxConn, err error) {
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
