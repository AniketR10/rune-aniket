// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package browserrpc

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
