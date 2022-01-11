package browser

import (
	context "context"
	"sync"

	"github.com/ernestrc/go-tui/proto"
	"google.golang.org/grpc"
)

// wraps a proto.HandlerClient to provide unlocking a resource mutex while waiting
// for a I/O based Handler to respond.
type ioUnlockHandler struct {
	lock sync.Locker
	h    proto.HandlerClient
}

func newIOWaitUnlockHandlerClient(
	h proto.HandlerClient, lock sync.Locker,
) proto.HandlerClient {
	ret := new(ioUnlockHandler)
	ret.init(h, lock)
	return ret
}

func (h *ioUnlockHandler) init(hc proto.HandlerClient, lock sync.Locker) {
	h.h = hc
	h.lock = lock
}

func (h *ioUnlockHandler) Handle(
	ctx context.Context, in *proto.HandleRequest, opts ...grpc.CallOption,
) (*proto.HandleResponse, error) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Handle(ctx, in, opts...)
}

func (h *ioUnlockHandler) Man(
	ctx context.Context, in *proto.ManRequest, opts ...grpc.CallOption,
) (*proto.ManResponse, error) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Man(ctx, in, opts...)
}

func (h *ioUnlockHandler) Close(
	ctx context.Context, in *proto.CloseRequest, opts ...grpc.CallOption,
) (*proto.CloseResponse, error) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Close(ctx, in, opts...)
}
