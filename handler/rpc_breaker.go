package handler

import (
	"context"
	"errors"
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
	ready
	pending
)

type clientBreaker struct {
	quitNext bool
	clientCh chan error
	quitCh   chan struct{}
	mu       sync.Mutex
	cc       proto.HandlerClient

	draw struct {
		state   breakerState
		prevReq *proto.DrawRequest
		prevRes *proto.DrawResponse
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

func (a *clientBreaker) dispatchDraw(
	ctx context.Context, in *proto.DrawRequest, opts ...grpc.CallOption,
) {
	res, err := a.cc.Draw(ctx, in, opts...)
	if err != nil {
		comp := component.String(smtgWrongCopy)
		comp.Resize(int(in.Width), int(in.Height))
		res = proto.NewDrawResponse(comp, int(in.Width), int(in.Height))
	}

	a.mu.Lock()
	a.draw.prevReq = in
	a.draw.prevRes = res
	// instead of setting to ready, we decrement such that only when
	// there are no outstanding draw requests, we move back to ready.
	a.draw.state--
	a.mu.Unlock()

	select {
	case <-a.quitCh:
	case a.clientCh <- err:
	}
}

func (a *clientBreaker) Draw(
	ctx context.Context, in *proto.DrawRequest, opts ...grpc.CallOption,
) (*proto.DrawResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch a.draw.state {
	case ready:
		a.draw.state = initial
		if a.draw.prevRes != nil && a.draw.prevReq.Width == in.Width &&
			a.draw.prevReq.Height == in.Height {
			return a.draw.prevRes, nil
		}

		fallthrough

	case initial:
		// so fallthrough will ++ and so become pending
		a.draw.state = ready

		fallthrough

	default:
		a.draw.state++

		go a.dispatchDraw(ctx, in, opts...)

		if a.draw.prevRes != nil && a.draw.prevReq.Width == in.Width &&
			a.draw.prevReq.Height == in.Height {
			return a.draw.prevRes, nil
		}
		// NOTE we could improve responsiveness by cancelling
		// context of previous request and re-issuing a request
		// if draw width, height is different that issued.
		loading := component.String(loadingCopy)
		loading.Resize(int(in.Width), int(in.Height))
		return proto.NewDrawResponse(loading, int(in.Width), int(in.Height)), nil
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
