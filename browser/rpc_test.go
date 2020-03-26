package browser

import (
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
)

func nop() {}

const testingShutdownWait = 500 * time.Millisecond

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

// used to emulate term event loop synchronization
type safeHandler struct {
	mu *sync.Mutex
	tui.Handler
}

func (h *safeHandler) Resize(width, height int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Handler.Resize(width, height)
}
func (h *safeHandler) Draw(w tui.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Handler.Draw(w)
}
func (h *safeHandler) Handle(ev term.Event) (exit, handled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.Handler.Handle(ev)
}
func (h *safeHandler) Cursor() (pos term.Coordinates, show bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.Handler.Cursor()
}
func (h *safeHandler) Man() tui.Manual {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.Handler.Man()
}

func newTestRPCBrowser(t *testing.T,
	destructor *func(),
) browserConstructor {
	return func(ed editor.Editor, opts ...Option) (browserInternal, error) {
		b := newTestBrowserHandler()
		err := b.Init(ed, opts...)
		if err != nil {
			return nil, err
		}

		lis, err := net.Listen("tcp", ":0")
		require.NoError(t, err)

		broker := newDialBroker()

		var mu sync.Mutex
		grpcServer := grpc.NewServer()
		server := NewServer(broker, b, &mu, nop, nop)
		server.shutdownWait = testingShutdownWait
		proto.RegisterWindowManagerServer(grpcServer, server)
		proto.RegisterMessengerServer(grpcServer, server)
		proto.RegisterKeyMapperServer(grpcServer, server)
		proto.RegisterFileOpenerServer(grpcServer, server)
		proto.RegisterEventPublisherServer(grpcServer, server)

		go grpcServer.Serve(lis)

		conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
		require.NoError(t, err)

		client := testClient{
			Client:  NewClient(broker, conn),
			Handler: &safeHandler{Handler: b, mu: &mu},
		}
		*destructor = func() {
			client.Close()
			server.Close()
			grpcServer.Stop()
		}
		return client, nil
	}
}

func TestRPCBrowserDraw(t *testing.T) {
	var destructor func()
	constructor := newTestRPCBrowser(t, &destructor)
	testBrowserHandlerDraw(t, constructor)
	destructor()
}

func TestRPCBrowserCloseLeak(t *testing.T) {
	var destructor func()
	browser, err := newTestRPCBrowser(t, &destructor)(&testEditor{}, WithFilepath(""))
	require.NoError(t, err)
	defer destructor()

	win, err := browser.SplitVerticalLeft(handler.NewTestHandler())
	require.NoError(t, err)

	require.NoError(t, win.Close())

	// NOTE: to reason about window/handler resource leaks
	// uncomment next line and analyze running goroutines
	// goleak.VerifyNone(t)
}

func TestMain(m *testing.M) {
	exitCode := m.Run()
	if exitCode == 0 {
		// this is to give time to server to close resources
		time.Sleep(testingShutdownWait)
		err := goleak.Find()
		if err != nil {
			fmt.Fprintf(os.Stderr, "goleak: Leaks on successful test run: %v\n", err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}
