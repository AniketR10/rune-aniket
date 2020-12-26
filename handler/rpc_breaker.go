package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
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
	closed
)

var errClientClosed = errors.New("client already closed")

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
//
// Note that Handle events are always "handled", meaning
// that clients of this clientBreaker should not use that field
// to know whether underlying client is handling a particular event or not.
type clientBreaker struct {
	quitCh chan struct{}
	mu     sync.Mutex
	cc     proto.HandlerClient

	// interrupt functions
	interruptDraw   func()
	interruptHandle func()

	draw struct {
		cancel  func()
		state   breakerState
		pending proto.DrawRequest
		ready   struct {
			proto.DrawRequest
			proto.DrawResponse
		}
	}

	handle struct {
		quit bool
		err  error
		ch   chan *proto.HandleRequest
	}

	logger *log.Logger

	man proto.Manual
}

func withClientBreaker(
	c proto.HandlerClient,
	interruptDraw, interruptHandle func(), logger *log.Logger,
) *clientBreaker {
	ret := new(clientBreaker)
	ret.cc = c
	ret.quitCh = make(chan struct{})
	ret.handle.ch = make(chan *proto.HandleRequest, handleBackpressureThres)
	ret.interruptDraw = interruptDraw
	ret.interruptHandle = interruptHandle
	ret.logger = logger

	go ret.pipelineHandleEvents()

	return ret
}

func (a *clientBreaker) pipelineHandleEvents() {
	for {
		select {
		case <-a.quitCh:
			return
		case req := <-a.handle.ch:
			res, err := a.cc.Handle(context.Background(), req)
			if err != nil || res.Quit {
				a.mu.Lock()
				a.handle.quit = true
				a.handle.err = err
				a.mu.Unlock()

				a.interruptHandle()
				return
			}

			a.interruptDraw()
		}
	}
}

func (a *clientBreaker) readyCopy() *proto.DrawResponse {
	res := new(proto.DrawResponse)
	*res = a.draw.ready.DrawResponse
	return res
}

func (a *clientBreaker) loadingContent(
	in *proto.DrawRequest,
) *proto.DrawResponse {
	if a.draw.ready.Width == in.Width &&
		a.draw.ready.Height == in.Height {
		return a.readyCopy()
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
	a.draw.pending = *in
	ctx, a.draw.cancel = context.WithCancel(ctx)
	go a.sendDrawRequest(ctx, in, opts...)
}

func (a *clientBreaker) transitionToReady(res *proto.DrawResponse, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch a.draw.state {
	case pending:
	case closed:
		return
	default:
		panic(fmt.Sprintf("corrupted state machine: "+
			"state should be pending if there's an "+
			"inflight request: err=%s, state=%d",
			err, a.draw.state))
	}

	// should be never nil because we check if context.Done() channel
	// before transitioning to ready.
	a.draw.cancel()
	a.draw.ready.DrawRequest = a.draw.pending
	a.draw.ready.DrawResponse = *res
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
		if a.logger != nil {
			a.logger.Errorf("error returned on Draw request: %v", err)
		}
	}

	select {
	case <-ctx.Done():
		return
	default:
	}

	a.transitionToReady(res, err)
	a.interruptDraw()
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
		if a.draw.ready.Width == in.Width &&
			a.draw.ready.Height == in.Height {
			a.draw.state = initial
			return a.readyCopy(), nil
		}

		resp := a.loadingContent(in)
		a.transitionToPending(ctx, in)
		return resp, nil

	case closed:
		return nil, errClientClosed

	default:
		panic(fmt.Sprintf("unknown state: %d", a.draw.state))
	}
}

func (a *clientBreaker) Handle(
	ctx context.Context, in *proto.HandleRequest, opts ...grpc.CallOption,
) (*proto.HandleResponse, error) {
	res := new(proto.HandleResponse)

	a.mu.Lock()
	quitNext := a.handle.quit
	errNext := a.handle.err
	a.mu.Unlock()

	if errNext != nil {
		return nil, errNext
	}

	select {
	case <-a.quitCh:
		return nil, errClientClosed
	default:
	}

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
	case <-a.quitCh:
		return nil, errClientClosed
	default:
		return nil, errors.New("remote handler is not processing events in a timely fashion")
	}
}

func (a *clientBreaker) Man(
	ctx context.Context, in *proto.ManRequest, opts ...grpc.CallOption,
) (*proto.ManResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.draw.state == closed {
		return nil, errClientClosed
	}
	return a.cc.Man(ctx, in, opts...)
}

func (a *clientBreaker) OnUnmount(
	ctx context.Context, in *proto.OnUnmountRequest, opts ...grpc.CallOption,
) (*proto.OnUnmountResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.draw.state == closed {
		return nil, errClientClosed
	}

	go func() {
		_, err := a.cc.OnUnmount(ctx, in, opts...)
		if err != nil {
			a.mu.Lock()
			defer a.mu.Unlock()

			// next Handle returns error
			a.handle.err = err
		}
	}()

	return new(proto.OnUnmountResponse), nil
}

func (a *clientBreaker) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.draw.state = closed
	close(a.quitCh)
	return nil
}
