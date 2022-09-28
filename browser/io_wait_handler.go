package browser

import (
	context "context"
	"sync"

	"google.golang.org/grpc"
	handlerpb "unstable.build/go-tui/handler/rpc"
)

// wraps a handlerpb.HandlerClient to provide unlocking a resource mutex while waiting
// for a I/O based Handler to respond.
type ioUnlockHandler struct {
	lock sync.Locker
	h    handlerpb.HandlerClient
}

func newIOWaitUnlockHandlerClient(
	h handlerpb.HandlerClient, lock sync.Locker,
) handlerpb.HandlerClient {
	ret := new(ioUnlockHandler)
	ret.init(h, lock)
	return ret
}

func (h *ioUnlockHandler) init(hc handlerpb.HandlerClient, lock sync.Locker) {
	h.h = hc
	h.lock = lock
}

func (h *ioUnlockHandler) Handle(
	ctx context.Context, in *handlerpb.HandleRequest, opts ...grpc.CallOption,
) (*handlerpb.HandleResponse, error) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Handle(ctx, in, opts...)
}

func (h *ioUnlockHandler) Man(
	ctx context.Context, in *handlerpb.ManRequest, opts ...grpc.CallOption,
) (*handlerpb.ManResponse, error) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Man(ctx, in, opts...)
}

func (h *ioUnlockHandler) Close(
	ctx context.Context, in *handlerpb.CloseRequest, opts ...grpc.CallOption,
) (*handlerpb.CloseResponse, error) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Close(ctx, in, opts...)
}
