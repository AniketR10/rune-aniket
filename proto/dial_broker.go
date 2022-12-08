package proto

import (
	fmt "fmt"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	"unstable.build/go-tui/util"
)

type brokerage struct {
	net.Listener
}

// DEPRECEATED: remove once migration to channel API is over
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

func (t *dialBroker) NewChannel(tags ...string) (net.Listener, error) {
	return util.TempUnixListenerTags(tags...)
}

func (t *dialBroker) DialChannel(address string) (conn MuxConn, err error) {
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithDialer(
		func(_ string, _ time.Duration) (net.Conn, error) {
			addr, err := net.ResolveUnixAddr("unix", address)
			if err != nil {
				return nil, err
			}
			return net.Dial(addr.Network(), addr.String())
		},
	)}
	conn, err = grpc.Dial("", opts...)
	if err != nil {
		return
	}

	if log.IsLevelEnabled(log.TraceLevel) {
		conn = loggingConn{conn}
	}
	return
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
