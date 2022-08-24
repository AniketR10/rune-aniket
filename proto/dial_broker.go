package proto

import (
	fmt "fmt"
	"net"
	"sync"

	grpc "google.golang.org/grpc"
)

type brokerage struct {
	net.Listener
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
	ret.id = 1
	return ret
}

func (t *dialBroker) NextId() uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.id++
	return t.id
}

func (t *dialBroker) Accept(ID uint32) (net.Listener, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.conns[ID]; ok {
		return nil, fmt.Errorf("trying to serve a connection that has been served already: %v", ID)
	}

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, fmt.Errorf("Listen: %w", err)
	}

	t.conns[ID] = brokerage{Listener: lis}
	return lis, nil
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

func (t *dialBroker) Cleanup(ID uint32) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	brokerage, ok := t.conns[ID]
	if !ok {
		return nil
	}
	err := brokerage.Listener.Close()
	delete(t.conns, ID)
	return err
}

func (t *dialBroker) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, br := range t.conns {
		br.Listener.Close()
	}
	t.conns = make(map[uint32]brokerage)
	return nil
}
