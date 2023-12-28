package rpc

import (
	context "context"
	"sync"

	"google.golang.org/grpc"
	handlerpb "unstable.build/go-tui/handler/rpc"
)

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

func (h *ioUnlockHandler) Draw(
	ctx context.Context, in *handlerpb.DrawRequest, opts ...grpc.CallOption,
) (*handlerpb.DrawResponse, error) {
	// do not unlock for Draw, as impls should simply draw, not call other APIs.
	return h.h.Draw(ctx, in, opts...)
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
