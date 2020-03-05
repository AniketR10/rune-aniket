package handler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type slowHandler struct {
	mu    sync.Mutex
	h     TestHandler
	delay time.Duration
}

func (h *slowHandler) setDelay(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.delay = d
}

func (h *slowHandler) Resize(width, height int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	time.Sleep(h.delay)
	h.h.Resize(width, height)
}

func (h *slowHandler) Draw(w tui.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()

	time.Sleep(h.delay)
	h.h.Draw(w)
}

func (h *slowHandler) Handle(ev term.Event) (exit, handled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	time.Sleep(h.delay)
	return h.h.Handle(ev)
}

func (h *slowHandler) Cursor() (pos term.Coordinates, show bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	time.Sleep(h.delay)
	return h.h.Cursor()
}

func (h *slowHandler) Man() tui.Manual {
	h.mu.Lock()
	defer h.mu.Unlock()

	time.Sleep(h.delay)
	return h.h.Man()
}

func newSlowHandler() *slowHandler {
	ret := new(slowHandler)
	ret.h = *NewTestHandler()
	ret.h.Manual.Summary = testHandlerManualDesc
	ret.h.Manual.Keys = testHandlerKeys
	return ret
}

func testHandlerTimeout(t *testing.T,
	rpc func(c proto.HandlerClient) (interface{}, error)) {
	mock := newSlowHandler()
	stubClient := &mockHandlerClient{remote: mock}
	c := withClientTimeout(stubClient, 50*time.Millisecond)

	mock.setDelay(0)

	res, err := rpc(c)
	require.NoError(t, err)
	assert.NotNil(t, res)

	mock.setDelay(1 * time.Second)

	res, err = rpc(c)
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestHandlerManTimeout(t *testing.T) {
	testHandlerTimeout(t, func(c proto.HandlerClient) (interface{}, error) {
		return c.Man(context.Background(), new(proto.ManRequest))
	})
}

func TestHandlerHandleTimeout(t *testing.T) {
	testHandlerTimeout(t, func(c proto.HandlerClient) (interface{}, error) {
		req := new(proto.HandleRequest)
		req.Event = new(proto.Event)
		return c.Handle(context.Background(), req)
	})
}

func TestHandlerDrawTimeout(t *testing.T) {
	testHandlerTimeout(t, func(c proto.HandlerClient) (interface{}, error) {
		return c.Draw(context.Background(), new(proto.DrawRequest))
	})
}
