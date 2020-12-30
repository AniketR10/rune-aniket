package browser

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// satisfies sync.Locker
type nopLocker struct{}

func (l nopLocker) Lock() {
}
func (l nopLocker) Unlock() {
}

// NOTE: this should probably me moved under Component package.

// NewMessageSpan returns a virtual message bar suitable for use with
// ResizeMessageSpan.
func NewMessageSpan(buf *cell.Buffer, bgAttr term.Attributes) handler.Virtual {
	scroll := component.NewScroll()
	scroll.InitWithBuffer(buf)
	scroll.Attributes = bgAttr

	background := term.Cell{Bg: bgAttr.Bg, Fg: bgAttr.Fg}
	content := component.WithBackground(scroll, background)
	return handler.Virtual{Virtual: component.Virtual{C: content}}
}

// ResizeMessageSpan resizes a handler.Virtual Message span returned by
// NewMessageSpan. It positions the handler.Virtual at the bottom
// of the available space with a 1 cell padding left and bottom.
func ResizeMessageSpan(logVirt *handler.Virtual, width, height int) {
	if width > 1 && height > 0 {
		logVirt.Resize(width-2, 1)
		busPos := term.Coordinates{X: 1, Y: height - 2}
		logVirt.Move(busPos)
	} else {
		logVirt.Resize(0, 0)
	}
}

func forceCloseResource(
	brokerID uint32, getResourcesFn func() map[uint32]io.Closer,
	logger *log.Logger, locker sync.Locker,
) (io.Closer, error) {
	locker.Lock()
	defer locker.Unlock()
	resources := getResourcesFn()
	res, ok := resources[brokerID]
	if !ok {
		if logger != nil {
			logger.Warnf("resource %d already closed", brokerID)
		}
		return nil, nil
	}
	delete(resources, brokerID)

	err := res.Close()
	if err != nil && logger != nil {
		logger.Errorf("resource.Close error: %v", err)
	}

	return res, err
}

const defaultFailureTimeout = 5 * time.Second

func monitorConnection(
	ctx context.Context, failureTimeout time.Duration,
	conn proto.MuxConn, onClosed func(reason string),
) {

	for {
		state := conn.GetState()
		switch state {
		case connectivity.Idle, connectivity.Connecting, connectivity.Ready:
			conn.WaitForStateChange(ctx, state)
		case connectivity.TransientFailure:
			failureCtx, cancelFn := context.WithTimeout(ctx, failureTimeout)
			didChange := conn.WaitForStateChange(failureCtx, connectivity.TransientFailure)
			cancelFn()
			if !didChange {
				onClosed("timeout waiting for transient failure to recover")
				return
			}
		case connectivity.Shutdown:
			onClosed("grpc connection state = shutdown")
			return
		default:
			panic(fmt.Sprintf("unknown connection state: %v", state))
		}
	}
}

func acceptAndServe(
	broker proto.MuxBroker, register func(uint32, *grpc.Server),
) (uint32, *grpc.Server) {
	brokerID := broker.NextId()

	var wg sync.WaitGroup
	var srv *grpc.Server
	serverFunc := func(opts []grpc.ServerOption) *grpc.Server {
		defer wg.Done()

		srv = grpc.NewServer(opts...)
		register(brokerID, srv)
		return srv
	}

	wg.Add(1)
	go broker.AcceptAndServe(brokerID, serverFunc)
	wg.Wait()

	return brokerID, srv
}
