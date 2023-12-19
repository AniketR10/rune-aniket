package rpc

import (
	"context"
	"errors"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/component"
	termrpc "unstable.build/go-tui/term/rpc"
)

const loadingCopy = "LOADING"

const (
	handleBackpressureThres = 128
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
	cc     HandlerClient

	// interrupt functions
	interruptDraw   func()
	interruptHandle func()

	draw struct {
		state   breakerState
		pending DrawRequest
		ready   struct {
			DrawRequest
			DrawResponse
		}
	}

	handle struct {
		quit bool
		err  error
		ch   chan *HandleRequest
	}

	logger *log.Logger

	man termrpc.Manual
}

func withClientBreaker(
	c HandlerClient,
	interruptDraw, interruptHandle func(), logger *log.Logger,
) *clientBreaker {
	ret := new(clientBreaker)
	ret.cc = c
	ret.quitCh = make(chan struct{})
	ret.handle.ch = make(chan *HandleRequest, handleBackpressureThres)
	ret.interruptDraw = interruptDraw
	ret.interruptHandle = interruptHandle
	ret.logger = logger

	go ret.pipelineHandleEvents()

	return ret
}

func (a *clientBreaker) sendHandle(ctx context.Context, req *HandleRequest) {
	defer a.interruptHandle()

	res, err := a.cc.Handle(ctx, req)

	if res.GetQuit() {
		a.mu.Lock()
		a.handle.quit = true
		a.mu.Unlock()
	}

	draw := res.GetDraw()
	if err == nil && draw == nil {
		err = errors.New("invalid HandleResponse: missing Draw property")
	}

	if err != nil {
		comp := component.NewStringWithConfig(smtgWrongCopy,
			component.StringConfig{Alignment: component.SpanAlignmentCentered})
		width, height := int(req.GetDraw().GetWidth()), int(req.GetDraw().GetHeight())
		comp.Resize(width, height)
		draw := NewDrawResponse(comp, width, height)
		if a.logger != nil {
			a.logger.Errorf("error returned on Draw request: %v", err)
		}
		a.mu.Lock()
		a.handle.err = err
		a.mu.Unlock()

		a.transitionToReady(draw, err)
		return
	}

	a.transitionToReady(draw, err)
}

func (a *clientBreaker) pipelineHandleEvents() {
	for {
		select {
		case <-a.quitCh:
			return
		case req := <-a.handle.ch:
			a.sendHandle(context.Background(), req)
		}
	}
}

func (a *clientBreaker) readyCopy() *DrawResponse {
	res := new(DrawResponse)
	*res = a.draw.ready.DrawResponse
	return res
}

func (a *clientBreaker) loadingContent(
	in *DrawRequest,
) *DrawResponse {
	if a.draw.ready.GetCursor() != nil &&
		a.draw.ready.GetWidth() == in.Width &&
		a.draw.ready.GetHeight() == in.Height {
		return a.readyCopy()
	}

	loading := component.NewStringWithConfig(loadingCopy,
		component.StringConfig{Alignment: component.SpanAlignmentCentered})
	loading.Resize(int(in.Width), int(in.Height))
	return NewDrawResponse(loading, int(in.Width), int(in.Height))
}

func (a *clientBreaker) transitionToPending(
	ctx context.Context, in *DrawRequest, opts ...grpc.CallOption,
) {
	a.draw.state = pending
	a.draw.pending = *in
}

func (a *clientBreaker) transitionToReady(res *DrawResponse, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.draw.ready.DrawRequest = a.draw.pending
	a.draw.ready.DrawResponse = *res
	a.draw.state = ready
}

func (a *clientBreaker) processDraw(
	ctx context.Context, in *DrawRequest,
) (*DrawResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch a.draw.state {
	case initial, pending:
		resp := a.loadingContent(in)
		a.transitionToPending(ctx, in)
		return resp, nil

	case ready:
		if a.draw.ready.Width == in.Width &&
			a.draw.ready.Height == in.Height {
			a.draw.state = initial
			return a.readyCopy(), nil
		}

		a.transitionToPending(ctx, in)
		resp := a.loadingContent(in)
		return resp, nil

	case closed:
		return nil, errClientClosed

	default:
		panic(fmt.Sprintf("unknown state: %d", a.draw.state))
	}
}

func (a *clientBreaker) Handle(
	ctx context.Context, in *HandleRequest, opts ...grpc.CallOption,
) (*HandleResponse, error) {
	res := new(HandleResponse)

	a.mu.Lock()
	quitNext := a.handle.quit
	errNext := a.handle.err
	a.handle.err = nil
	a.mu.Unlock()

	select {
	case <-a.quitCh:
		return nil, errClientClosed
	default:
	}

	if quitNext {
		res.Quit = quitNext
		res.Draw = NewDrawResponse(component.Nop(), 0, 0)
		return res, nil
	}

	res.Handled = true

	drawRes, err := a.processDraw(ctx, in.GetDraw())
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	state := a.draw.state
	a.mu.Unlock()

	// interrupt handle event. If state == initial, then it means that
	// we can simply return the last draw result.
	if in.GetEvent().GetType() == termrpc.Event_TypeNone && state == initial {
		res.Draw = drawRes
		return res, nil
	}

	// if underlying rpc.HandlerClient is not processing events
	// in a timely fashion, we start returning errors as a safety valve.
	// This avoids deadlocking with browser lock. See Client documentation.
	select {
	case a.handle.ch <- in:
		if errNext != nil {
			return nil, errNext
		}
		res.Draw = drawRes
		return res, nil
	case <-a.quitCh:
		return nil, errClientClosed
	default:
		return nil, errors.New("remote handler is not processing events in a timely fashion")
	}
}

func (a *clientBreaker) Man(
	ctx context.Context, in *ManRequest, opts ...grpc.CallOption,
) (*ManResponse, error) {
	a.mu.Lock()
	if a.draw.state == closed {
		return nil, errClientClosed
	}
	a.mu.Unlock()
	return a.cc.Man(ctx, in, opts...)
}

func (a *clientBreaker) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.draw.state = closed
	close(a.quitCh)
	return nil
}
