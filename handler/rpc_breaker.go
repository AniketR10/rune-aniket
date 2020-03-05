package handler

import (
	"context"
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
	// if underlying proto.HandlerClient is not processing events
	// in a timely fashion, we start applying backpressure when
	// we have 10 events queued.
	handleBackpressureThres = 10
)

type breakerState uint8

const (
	initial = iota
	waiting
	ready
)

type clientBreaker struct {
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
			_, err := a.cc.Handle(context.Background(), req)
			if err != nil {
				select {
				case a.clientCh <- err:
				case <-a.quitCh:
					return
				}
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
	a.draw.state = ready
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
		a.draw.state = waiting
		go a.dispatchDraw(ctx, in, opts...)

		fallthrough

	case waiting:
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

	default:
		panic("circuing breaker client unkown draw.state")
	}
}

func (a *clientBreaker) Handle(
	ctx context.Context, in *proto.HandleRequest, opts ...grpc.CallOption,
) (*proto.HandleResponse, error) {
	a.handle.ch <- in
	return new(proto.HandleResponse), nil
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
