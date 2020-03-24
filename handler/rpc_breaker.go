package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/proto"
	"google.golang.org/grpc"
)

const loadingCopy = "LOADING"

const smtgWrongCopy = `

          ___
         /___/\_               
        _\   \/_/\__           
      __\       \/_/\          
      \   __    __ \ \         
     __\  \_\   \_\ \ \   __   
    /_/\\   __   __  \ \_/_/\  
    \_\/_\__\/\__\/\__\/_\_\/  
       \_\/_/\       /_\_\/    
          \_\/       \_\/      
    

Uh, Houston, we've had a problem
`

const (
	handleBackpressureThres = 10
)

type breakerState uint8

const (
	initial = iota
	pending
	ready
)

// clientBreaker Draw logic follows the following state diagram:
//
// Initial - DrawReqSame ->> Pending
// Initial -- DrawRes -->> Error
// Pending - DrawRes ->> Ready
// Pending - DrawReqSame ->> Cancel
// Pending - DrawReqDiff ->> Cancel
// Ready - DrawReqSame ->> Initial
// Ready - DrawReqDiff ->> Pending
// Ready -- DrawRes -->> Error
// Cancel ->> Pending
//
// http://chartmage.com/index.html
type clientBreaker struct {
	quitNext bool
	clientCh chan error
	quitCh   chan struct{}
	mu       sync.Mutex
	cc       proto.HandlerClient

	draw struct {
		cancel  func()
		state   breakerState
		prevReq proto.DrawRequest
		prevRes proto.DrawResponse
	}

	handle struct {
		ch chan *proto.HandleRequest
	}

	man proto.Manual
}

func withClientBreaker(c proto.HandlerClient) (*clientBreaker, chan error) {
	ret := new(clientBreaker)
	ret.cc = c
	ret.clientCh = make(chan error)
	ret.quitCh = make(chan struct{})
	ret.handle.ch = make(chan *proto.HandleRequest, handleBackpressureThres)

	go ret.pipelineHandleEvents()

	return ret, ret.clientCh
}

func (a *clientBreaker) pipelineHandleEvents() {
	for {
		select {
		case <-a.quitCh:
			return
		case req := <-a.handle.ch:
			res, err := a.cc.Handle(context.Background(), req)
			if err == nil && res.Quit {
				a.mu.Lock()
				a.quitNext = true
				a.mu.Unlock()

				return
			}

			select {
			case a.clientCh <- err:
			case <-a.quitCh:
				return
			}
		}
	}
}

func (a *clientBreaker) prevResCopy() *proto.DrawResponse {
	res := new(proto.DrawResponse)
	*res = a.draw.prevRes
	return res
}

func (a *clientBreaker) loadingContent(
	in *proto.DrawRequest,
) *proto.DrawResponse {
	if a.draw.prevReq.Width == in.Width &&
		a.draw.prevReq.Height == in.Height {
		return a.prevResCopy()
	}

	loading := component.String(loadingCopy)
	loading.Resize(int(in.Width), int(in.Height))
	return proto.NewDrawResponse(loading, int(in.Width), int(in.Height))
}

func (a *clientBreaker) transitionToCancel() {
	if a.draw.cancel == nil {
		panic("corrupted state machine: tried to cancel same request twice")
	}

	a.draw.cancel()
	a.draw.cancel = nil
}

func (a *clientBreaker) transitionToPending(
	ctx context.Context, in *proto.DrawRequest, opts ...grpc.CallOption,
) {
	a.draw.state = pending
	a.draw.prevReq = *in
	ctx, a.draw.cancel = context.WithCancel(ctx)
	go a.sendDrawRequest(ctx, in, opts...)
}

func (a *clientBreaker) transitionToReady(res *proto.DrawResponse, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.draw.state != pending {
		panic(fmt.Sprintf("corrupted state machine: "+
			"state should be pending if there's an "+
			"inflight request: err=%s, state=%d",
			err, a.draw.state))
	}

	// should be never nil; if canceled err should be context.Canceled
	a.draw.cancel()
	a.draw.prevRes = *res
	a.draw.state = ready
}

func (a *clientBreaker) sendDrawRequest(
	ctx context.Context, in *proto.DrawRequest, opts ...grpc.CallOption,
) {
	res, err := a.cc.Draw(ctx, in, opts...)
	if err != nil {
		comp := component.String(smtgWrongCopy)
		comp.Resize(int(in.Width), int(in.Height))
		res = proto.NewDrawResponse(comp, int(in.Width), int(in.Height))
	}

	select {
	case <-ctx.Done():
		return
	default:
	}

	a.transitionToReady(res, err)

	// TODO there should be another state here: interrupt not yet issued
	// but received another draw request. should unlocking be defered to
	// after clientCh channel is drained?

	select {
	case <-a.quitCh:
	case a.clientCh <- err:
	}
}

func (a *clientBreaker) Draw(
	ctx context.Context, in *proto.DrawRequest,
	opts ...grpc.CallOption,
) (*proto.DrawResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch a.draw.state {
	case initial:
		resp := a.loadingContent(in)
		a.transitionToPending(ctx, in)
		return resp, nil

	case pending:
		resp := a.loadingContent(in)
		a.transitionToCancel()
		a.transitionToPending(ctx, in)
		return resp, nil

	case ready:
		if a.draw.prevReq.Width == in.Width &&
			a.draw.prevReq.Height == in.Height {
			a.draw.state = initial
			return a.prevResCopy(), nil
		}

		resp := a.loadingContent(in)
		a.transitionToPending(ctx, in)
		return resp, nil

	default:
		panic(fmt.Sprintf("unknown state: %d", a.draw.state))
	}
}

func (a *clientBreaker) Handle(
	ctx context.Context, in *proto.HandleRequest, opts ...grpc.CallOption,
) (*proto.HandleResponse, error) {
	res := new(proto.HandleResponse)

	a.mu.Lock()
	quitNext := a.quitNext
	a.mu.Unlock()

	if quitNext {
		res.Quit = quitNext
		return res, nil
	}

	res.Handled = true

	// if underlying proto.HandlerClient is not processing events
	// in a timely fashion, we start returning errors as a safety valve.
	// This avoids deadlocking with browser lock. See Client documentation.
	select {
	case a.handle.ch <- in:
		return res, nil
	default:
		return nil, errors.New("remote handler is not processing events in a timely fashion")
	}
}

func (a *clientBreaker) Man(
	ctx context.Context, in *proto.ManRequest, opts ...grpc.CallOption,
) (*proto.ManResponse, error) {
	return a.cc.Man(ctx, in, opts...)
}

func (a *clientBreaker) Close() error {
	close(a.quitCh)
	return nil
}
