package proto

import (
	context "context"
	fmt "fmt"
	"net"
	"sync"
	"time"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/go-tui/util"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// satisfies to MuxBroker
type dbBroker struct {
	mu        sync.Mutex
	id        uint32
	svc       document.Service
	listeners []net.Listener
	logger    *log.Logger
}

type nextIDDoc struct {
	ID uint32
}

type listenerDoc struct {
	Address string
}

// NewDatastoreBroker provides brokerage by employing a datastore to share
// connection information.
func NewDatastoreBroker(svc document.Service, logger *log.Logger) MuxBroker {
	ret := new(dbBroker)
	ret.svc = svc
	ret.logger = logger
	return ret
}

func makeListenerDocumentKey(i uint32) string {
	return fmt.Sprintf("LISTENER: %d", i)
}

func makeNextIDDocument(i uint32) (string, nextIDDoc) {
	return fmt.Sprintf("ID: %d", i), nextIDDoc{i}
}

func makeListenerDocument(i uint32, listener net.Listener) (string, listenerDoc) {
	return makeListenerDocumentKey(i), listenerDoc{Address: listener.Addr().String()}
}

func (t *dbBroker) NextId() uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()

	for {
		t.id++
		key, doc := makeNextIDDocument(t.id)
		err := t.svc.Create(context.Background(), key, doc)
		if err != nil && err != document.ErrAlreadyExists {
			panic(err)
		} else if err == nil {
			break
		}
	}
	return t.id
}

func (t *dbBroker) Accept(id uint32) (net.Listener, error) {
	listener, err := util.TempUnixListener()
	if err != nil {
		return nil, err
	}
	key, doc := makeListenerDocument(id, listener)
	err = t.svc.Create(context.Background(), key, doc)
	if err != nil {
		return nil, err
	}
	t.listeners = append(t.listeners, listener)
	return listener, nil
}

func (t *dbBroker) AcceptAndServe(
	ID uint32, srv func(opts []grpc.ServerOption) MuxServer,
) {
	t.mu.Lock()
	defer t.mu.Unlock()

	lis, err := t.Accept(ID)
	if err != nil {
		panic(err)
	}
	server := srv([]grpc.ServerOption{})
	go server.Serve(lis)
}

func (t *dbBroker) Dial(ID uint32) (conn MuxConn, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var lis listenerDoc
	err = t.svc.Get(context.Background(), makeListenerDocumentKey(ID), &lis)
	if err != nil {
		return
	}

	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithDialer(
		func(_ string, _ time.Duration) (net.Conn, error) {
			addr, err := net.ResolveUnixAddr("unix", lis.Address)
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

	if t.logger != nil && t.logger.IsLevelEnabled(log.TraceLevel) {
		conn = loggingConn{t.logger, conn}
	}
	return
}

func (t *dbBroker) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, lis := range t.listeners {
		_ = lis.Close()
	}
	t.listeners = nil
	return nil
}
