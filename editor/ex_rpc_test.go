package editor

import (
	"fmt"
	"net"
	_ "net/http/pprof"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
)

const testingShutdownWait = 500 * time.Millisecond

type groupEventHandler struct {
	h  *handler.TestHandler
	wg *sync.WaitGroup
}

func (h *groupEventHandler) Handle(ev term.Event) (handled bool) {
	defer h.wg.Done()
	h.h.Handle(ev)
	return
}

// binds together a client with the remote browser, so we can verify
type testClient struct {
	*browser.Client
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
func (h *safeHandler) Draw(w term.Writer) {
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

func nop() {}

func newTestRPCBrowser(t *testing.T,
	destructor *func(),
) browserConstructor {
	return func(ed Editor, opts ...browser.Option) (browserInternal, error) {
		b := newTestBrowserHandler()
		err := b.Init(ed, opts...)
		if err != nil {
			return nil, err
		}

		lis, err := net.Listen("tcp", ":0")
		require.NoError(t, err)

		broker := proto.NewDialBroker()

		var clientMutex sync.Mutex
		var serverMutex sync.Mutex
		grpcServer := grpc.NewServer()
		server := browser.NewServer(broker, b, &serverMutex, nop, nop)
		proto.RegisterWindowManagerServer(grpcServer, server)
		proto.RegisterMessengerServer(grpcServer, server)
		proto.RegisterKeyMapperServer(grpcServer, server)
		proto.RegisterResourceOpenerServer(grpcServer, server)
		proto.RegisterEventSubscriberServer(grpcServer, server)

		go grpcServer.Serve(lis)

		conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
		require.NoError(t, err)

		client := testClient{
			Client:  browser.NewClient(broker, conn, &clientMutex),
			Handler: &safeHandler{Handler: b, mu: &serverMutex},
		}
		*destructor = func() {
			client.Close()
			server.Close()
			grpcServer.Stop()
		}
		return client, nil
	}
}

func TestIntegrationRPCBrowserDraw(t *testing.T) {
	var destructor func()
	constructor := newTestRPCBrowser(t, &destructor)
	testBrowserHandlerDraw(t, constructor)
	destructor()
}

func TestRPCBrowserCloseLeak(t *testing.T) {
	var destructor func()
	browser, err := newTestRPCBrowser(t, &destructor)(&testEditor{}, browser.WithFilepath(""))
	require.NoError(t, err)
	defer destructor()

	win, err := browser.SplitVerticalLeft(handler.NewTestHandler())
	require.NoError(t, err)

	require.NoError(t, win.Close())

	// NOTE: to reason about window/handler resource leaks
	// uncomment next line and analyze running goroutines
	// goleak.VerifyNone(t)
}

// NOTE: run go test -race in order for this test to be useful.
func TestClientSynchronizeHandlers(t *testing.T) {
	var destructor func()
	browser, err := newTestRPCBrowser(t, &destructor)(&testEditor{}, browser.WithFilepath(""))
	require.NoError(t, err)
	defer destructor()
	var wg sync.WaitGroup

	h := handler.TestHandler{}

	subs := []term.Event{
		term.Event{Type: term.EventKey, Key: term.KeyCtrlA},
		term.Event{Type: term.EventKey, Key: term.KeyCtrlJ},
		term.Event{Type: term.EventKey, Key: term.KeyCtrlH},
		term.Event{Type: term.EventKey, Key: term.KeyCtrlB},
	}

	for _, ev := range subs {
		h := &groupEventHandler{wg: &wg, h: &h}
		err = browser.Subscribe(ev, h)
		require.NoError(t, err)
	}

	for _, ev := range subs {
		wg.Add(1)
		_, handled := browser.Handle(ev)
		assert.True(t, handled)
	}

	wg.Wait()
}

func TestMain(m *testing.M) {
	exitCode := m.Run()
	if exitCode == 0 {
		ignoreOpenCensus := goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")
		// this is to give time to server to close resources
		time.Sleep(testingShutdownWait)
		err := goleak.Find(ignoreOpenCensus)
		if err != nil {
			fmt.Fprintf(os.Stderr, "goleak: Leaks on successful test run: %v\n", err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}
