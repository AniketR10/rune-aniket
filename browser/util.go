package browser

import (
	"context"
	"io"
	"sync"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/connectivity"
)

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
	lock sync.Locker, brokerID uint32,
	resources map[uint32]io.Closer, logger *log.Logger,
) error {
	lock.Lock()
	defer lock.Unlock()

	res, ok := resources[brokerID]
	if !ok {
		if logger != nil {
			logger.Warnf("resource %d already closed", brokerID)
		}
		return nil
	}

	err := res.Close()
	if err != nil && logger != nil {
		logger.Error(err)
	}

	delete(resources, brokerID)

	return err
}

func monitorConnection(ctx context.Context, conn proto.MuxConn, callback func()) {
	for conn.GetState() != connectivity.Shutdown {
		conn.WaitForStateChange(ctx, connectivity.Ready)
	}
	callback()
}
