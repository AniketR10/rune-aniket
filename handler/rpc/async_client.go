package rpc

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

var _ tui.Handler = (*AsyncClient)(nil)

const (
	smtgWrongCopy = `

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
	asyncDrawRPCTimeout  = 5 * time.Second
	startingErrorTimeout = 1 * time.Second
	maxErrorTimeout      = 10 * time.Second
)

// AsyncClient satisfies Handler by talking to a remote handler over GRPC.
// It builds upon Client by adding asynchronous Draw.
type AsyncClient struct {
	Client
	interrupter    term.Interrupter
	ch             chan request
	contextPayload string
	loading        tui.Component
	timeout        time.Duration
	circuitBreak   bool
	waitClose      chan struct{}

	mu    sync.Mutex
	state asyncState

	req struct {
		height, width int
		cancel        func()
		ctx           context.Context
	}
	resp struct {
		*DrawResponse
		ctx           context.Context
		height, width int
	}
}

// NewClient allocates storage for a new Client and initializes it.
func NewAsyncClient(interrupter term.Interrupter, pbClient HandlerClient) *AsyncClient {
	ret := new(AsyncClient)
	ret.Init(interrupter, pbClient)
	return ret
}

// Init initialies this Client with pbClient and the given interrupt func.
func (c *AsyncClient) Init(interrupter term.Interrupter, pbClient HandlerClient) {
	c.Client.Init(pbClient)

	// buffered: ensure that we can drop a Draw request if nothing is consuming
	// the next draw request, knowing that at least there is one more request
	// to be dispatched.
	c.ch = make(chan request, 1)
	c.waitClose = make(chan struct{})
	c.interrupter = interrupter
	c.contextPayload = makeContextPayload(c)
	c.resp.ctx = context.Background()
	c.req.cancel = func() {}
	c.timeout = startingErrorTimeout
	c.loading = component.NewStringWithConfig("LOADING",
		component.StringConfig{Alignment: component.SpanAlignmentCentered})

	go c.processEvents()
}

// Resize satisfies tui.Handler
func (c *AsyncClient) Resize(width, height int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Client.Resize(width, height)
}

// Draw satisfies tui.Handler
func (c *AsyncClient) Draw(w term.Writer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx := w.Context()
	iterationID, reqIsTick := tui.IterationFromContext(ctx)

	switch c.state {
	case stateAsyncIdle:
		readySameDimensions := c.width == c.resp.width && c.height == c.resp.height
		respIterationID, respIsTick := tui.IterationFromContext(c.resp.ctx)
		olderIterationID := respIsTick && iterationID <= respIterationID
		reqIsSameIterationID := (reqIsTick && olderIterationID)
		selfInterrupt := !reqIsTick && !respIsTick && c.contextPayloadIsSelf(ctx)

		if log.IsLevelEnabled(log.TraceLevel) {
			c.log(log.TraceLevel, "Draw(initial): iterationID: %d, readySameDimensions: %t,"+
				"reqIsSameIterationID: %t, selfInterrupt: %t, otherClientInterrupt: %t",
				iterationID, readySameDimensions, reqIsSameIterationID, selfInterrupt,
				c.contextPayloadIsOtherAsyncClient(ctx))
		}

		if readySameDimensions && (reqIsSameIterationID || selfInterrupt ||
			c.contextPayloadIsOtherAsyncClient(ctx)) {
			c.drawReady(w)
		} else if c.scheduleDrawRequest(ctx, reqIsTick) {
			c.drawPending(w)
		} else {
			c.drawError(w)
		}
	case stateAsyncPending:
		pendingRespSameDimensions := c.width == c.req.width && c.height == c.req.height
		pendingResponseIterationID, pendingResponseIsTick := tui.IterationFromContext(c.req.ctx)
		olderIterationID := pendingResponseIsTick && iterationID <= pendingResponseIterationID
		pendingResponseIsSameIterationID := (reqIsTick && olderIterationID)

		if log.IsLevelEnabled(log.TraceLevel) {
			c.log(log.TraceLevel, "Draw(pending): iterationID: %d, pendingRespSameDimensions: %t,"+
				"pendingResponseIsSameIterationID: %t, otherClientInterrupt: %t",
				iterationID, pendingRespSameDimensions, pendingResponseIsSameIterationID,
				c.contextPayloadIsOtherAsyncClient(ctx))
		}

		if pendingRespSameDimensions && (pendingResponseIsSameIterationID ||
			c.contextPayloadIsOtherAsyncClient(ctx)) {
			c.drawPending(w)
		} else if c.scheduleDrawRequest(ctx, reqIsTick) {
			c.drawPending(w)
		} else {
			c.drawError(w)
		}
	default:
		panic("unknown state")
	}
}

// Handle satisfies tui.Handle.
func (c *AsyncClient) Handle(ev term.Event) (bool, bool) {
	c.mu.Lock()
	circuitBreak := c.circuitBreak
	c.mu.Unlock()

	if circuitBreak {
		return false, false
	}
	return c.Client.Handle(ev)
}

// Handle satisfies tui.Handle.
func (c *AsyncClient) Man() tui.Manual {
	c.mu.Lock()
	circuitBreak := c.circuitBreak
	c.mu.Unlock()

	if circuitBreak {
		return tui.Manual{}
	}
	return c.Client.Man()
}

// Cursor satisfies tui.Handler
func (c *AsyncClient) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	c.mu.Lock()
	// synchronize access to cached cursor
	defer c.mu.Unlock()

	if c.circuitBreak {
		return
	}

	return c.Client.Cursor()
}

type asyncState uint8

const (
	stateAsyncIdle asyncState = iota
	stateAsyncPending
)

func (c *AsyncClient) log(level log.Level, msg string, args ...interface{}) {
	log.WithFields(log.Fields{
		logging.KeyClass: "handler.AsyncClient",
		"ptr":            fmt.Sprintf("%p", c),
	}).Logf(level, msg, args...)
}

func (c *AsyncClient) processEvents() {
	defer close(c.waitClose)
	for {
		select {
		case req := <-c.ch:
			err := c.sendDrawReq(req)
			if err != nil {
				c.collectError("send draw request", err)
			}
		case <-c.ctx.Done():
			c.log(log.DebugLevel, "done processing events")
			return
		}
	}
}

type request struct {
	ctx    context.Context
	width  int
	height int
}

func (r request) String() string {
	payload, _ := term.PayloadFromContext(r.ctx)
	return fmt.Sprintf("request{ctx: %s}", string(payload))
}

func (c *AsyncClient) scheduleDrawRequest(ctx context.Context, reqIsTick bool) bool {
	if c.circuitBreak {
		return false
	}
	// make sure that all requests that we schedule interrupts for have
	// either an iteration ID or a payload that we can recognize.
	if !reqIsTick {
		ctx = c.contextWithSelfPayload(ctx)
	}

	cancel := c.req.cancel
	r := request{width: c.width, height: c.height, ctx: ctx}

	c.log(log.TraceLevel, "scheduling new draw in state=%v: %s", c.state, r)

	c.mu.Unlock()
	defer c.mu.Lock()

	// cancel previous request so the single goroutine processing requests
	// is ready to take the request below in the select statement
	cancel()

	select {
	case <-c.ctx.Done():
		return false
	default:
	}

	// split in two selects, so an empty chanel with no consumer doesn't
	// get selected over ctx.Done, which would signal that request
	// is scheduled when it's actually not.
	select {
	case c.ch <- r:
		return true
	default:
		// there's already one request pending in the buffered chan
		// so consider this as a succesful schedule
		return true
	}
}

func (c *AsyncClient) sendDrawReq(r request) error {

	ctx, cancel := context.WithTimeout(c.ctx, asyncDrawRPCTimeout)
	defer cancel()

	c.mu.Lock()
	c.state = stateAsyncPending
	c.req.cancel = cancel
	c.req.width = r.width
	c.req.height = r.height
	// NOTE: this context is used for figuring out draw context
	// not this RPC's cancelation context, so we use r.ctx.
	c.req.ctx = r.ctx
	c.mu.Unlock()

	c.log(log.TraceLevel, "draw request %s", r)
	rpcReq := DrawRequest{Width: int32(r.width), Height: int32(r.height)}
	resp, err := c.client.Draw(ctx, &rpcReq)
	c.log(log.TraceLevel, "draw response to %s: err=%v: scheduling interrupt..", r, err)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()

		if status.Code(err) == codes.Canceled {
			return nil
		}

		c.state = stateAsyncIdle
		c.setErrorResponse(r.ctx)

		// avoid thundering herd issues when error notifications high FPS
		// swamp a slow handler with more requests.
		c.timeout *= 2
		c.timeout = time.Duration(math.Max(float64(c.timeout), float64(maxErrorTimeout)))
		c.circuitBreak = true
		go func(d time.Duration) {
			timer := time.NewTimer(d)
			select {
			case <-timer.C:
			case <-c.ctx.Done():
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			c.circuitBreak = false
		}(c.timeout)
		c.interrupter.Interrupt(r.ctx)
		return err
	}

	// call Interrupt after we have unlocked mu
	defer c.interrupter.Interrupt(r.ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	// even if the result is stale: i.e. another request has been received
	// by main goroutine, there's only 1 goroutine processing sendRawReq, so storing
	// this response until the next one is processed is ok.

	c.resp.DrawResponse = resp
	// NOTE: same here as note above.
	c.resp.ctx = r.ctx
	c.resp.height = int(r.height)
	c.resp.width = int(r.width)

	cursor := resp.GetCursor()
	c.cursor.show = cursor.GetShow()
	c.cursor.style = term.CursorStyle(cursor.GetStyle())
	c.cursor.Coordinates.X = int(cursor.GetPosition().GetX())
	c.cursor.Coordinates.Y = int(cursor.GetPosition().GetY())
	c.state = stateAsyncIdle // reset error timeout after a successful response
	c.timeout = startingErrorTimeout

	return nil
}

// Close frees all resources associated with this client.
func (c *AsyncClient) Close() error {
	err := c.Client.Close()
	<-c.waitClose
	return err
}

func (c *AsyncClient) setErrorResponse(ctx context.Context) {
	comp := component.NewStringWithConfig(smtgWrongCopy,
		component.StringConfig{Alignment: component.SpanAlignmentCentered})
	comp.Resize(c.width, c.height)
	draw := NewDrawResponse(ctx, comp, c.width, c.height)
	c.resp.DrawResponse = draw
	c.resp.width = c.width
	c.resp.height = c.height
	c.resp.ctx = ctx
	c.cursor.show = false
}

func (c *AsyncClient) drawError(w term.Writer) {
	comp := component.NewStringWithConfig(smtgWrongCopy,
		component.StringConfig{Alignment: component.SpanAlignmentCentered})
	comp.Resize(c.width, c.height)
	comp.Draw(w)
}

func (c *AsyncClient) drawPending(w term.Writer) {
	if c.width == c.resp.width && c.height == c.resp.height {
		c.drawReady(w)
	} else {
		c.loading.Resize(c.width, c.height)
		c.loading.Draw(w)
	}
}

func (c *AsyncClient) drawReady(w term.Writer) {
	doDraw(w, c.resp.DrawResponse)
}

func (c *AsyncClient) contextPayloadIsSelf(ctx context.Context) bool {
	payload, ok := term.PayloadFromContext(ctx)
	return ok && c.contextPayload == string(payload)
}

func (c *AsyncClient) contextPayloadIsOtherAsyncClient(ctx context.Context) bool {
	payload, ok := term.PayloadFromContext(ctx)
	return ok && strings.HasPrefix(string(payload), "AsyncClient")
}

func (c *AsyncClient) contextWithSelfPayload(ctx context.Context) context.Context {
	return term.ContextWithPayload(ctx, []byte(c.contextPayload))
}

func makeContextPayload(c *AsyncClient) string {
	return fmt.Sprintf("AsyncClient:%p", c)
}
