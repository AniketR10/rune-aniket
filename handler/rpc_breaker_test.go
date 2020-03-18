package handler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientBreakerMan(t *testing.T) {
	mock := &mockHandlerClient{remote: testHandler()}
	b, _ := withClientBreaker(mock)
	defer b.Close()

	t.Run("dispatches Man synchronously", func(t *testing.T) {
		manRes, err := b.Man(context.Background(), new(proto.ManRequest))
		require.NoError(t, err)
		require.NotNil(t, manRes)

		man, err := manRes.GetMan().ToModel()
		require.NoError(t, err)

		assertTestManual(t, man)
	})

	t.Run("bubbles up error", func(t *testing.T) {
		mock.rpcError = errors.New("man error")

		manRes, err := b.Man(context.Background(), new(proto.ManRequest))
		assert.Error(t, err)
		assert.Nil(t, manRes)
	})
}

func consumeMockEvents(
	mock *mockHandlerClient, quitChan chan struct{},
) {
	for {
		select {
		case <-mock.handledCh:
		case <-quitChan:
			return
		}
	}
}

func slowlyConsumeMockEvents(
	wg *sync.WaitGroup,
	mock *mockHandlerClient,
	quitChan chan struct{},
) {
	for {
		select {
		case <-mock.handledCh:
			time.Sleep(1 * time.Millisecond)
			wg.Done()
		case <-quitChan:
			return
		}
	}
}

func consumeInterrupts(
	ch chan error,
	quitChan chan struct{},
) {
	for {
		select {
		case <-ch:
		case <-quitChan:
			return
		}
	}
}

func closeTestingResources(b *clientBreaker, quitChan chan struct{}) {
	b.Close()
	close(quitChan)
}

func TestClientBreakerHandle(t *testing.T) {
	req := &proto.HandleRequest{
		Event: &proto.Event{Char: '$', Type: proto.Event_TypeKey},
	}

	t.Run("dispatches events asynchronously", func(t *testing.T) {
		mock := &mockHandlerClient{remote: testHandler()}
		b, _ := withClientBreaker(mock)
		defer b.Close()

		mock.handledCh = make(chan term.Event)
		res, err := b.Handle(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, res)

		expected := term.Event{Ch: '$', Type: term.EventKey}
		assert.Equal(t, expected, <-mock.handledCh)
	})

	t.Run("dispatches handle errors to error chan", func(t *testing.T) {
		mock := &mockHandlerClient{remote: testHandler()}
		b, errCh := withClientBreaker(mock)
		defer b.Close()

		mock.handledCh = make(chan term.Event)
		mock.rpcError = errors.New("sup")

		res, err := b.Handle(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, res)

		assert.Equal(t, mock.rpcError, <-errCh)
	})

	t.Run("dispatches all events, even when upon backpressure", func(t *testing.T) {
		var wg sync.WaitGroup
		h := NewTestHandler()
		mock := &mockHandlerClient{remote: h}
		b, ch := withClientBreaker(mock)
		quitChan := make(chan struct{})
		mock.handledCh = make(chan term.Event)

		go slowlyConsumeMockEvents(&wg, mock, quitChan)
		defer closeTestingResources(b, quitChan)
		go consumeInterrupts(ch, quitChan)

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			_, err := b.Handle(context.Background(), req)
			if err != nil {
				wg.Done()
			}
		}

		wg.Wait()
	})

	t.Run("dispatches exit on next handle event", func(t *testing.T) {
		h := NewTestHandler()
		h.Exit = true
		mock := &mockHandlerClient{remote: h}
		b, ch := withClientBreaker(mock)
		quitChan := make(chan struct{})
		mock.handledCh = make(chan term.Event)

		go consumeMockEvents(mock, quitChan)
		go consumeInterrupts(ch, quitChan)
		defer closeTestingResources(b, quitChan)

		// next exit is false, because events are handled
		// asynchronously
		res, err := b.Handle(context.Background(), req)
		require.NoError(t, err)
		assert.False(t, res.GetQuit())

		time.Sleep(50 * time.Millisecond)

		// next should be now true
		res, err = b.Handle(context.Background(), req)
		require.NoError(t, err)
		assert.True(t, res.GetQuit())
	})
}

func assertDrawResponse(t *testing.T, res *proto.DrawResponse, strCopy string) {
	require.NotNil(t, res)
	str, width, height := proto.DrawResponseToTermString(res)
	expected := component.String(strCopy)
	expected.Resize(width, height)
	w := term.NewStringWriter(width, height)
	expected.Draw(w)
	w.Flush()
	assert.Equal(t, w.String(), str)
}

func TestClientBreakerDraw(t *testing.T) {
	const testHandlerCopy = "AAAAA\nAAAAA\nAAAAA\nAAAAA\nAAAAA"

	req := &proto.DrawRequest{Width: 5, Height: 5}

	t.Run("dispatches draw requests asynchonously", func(t *testing.T) {
		mock := &mockHandlerClient{remote: testHandler()}
		b, errCh := withClientBreaker(mock)
		defer b.Close()

		res, err := b.Draw(context.Background(), req)
		require.NoError(t, err)
		assertDrawResponse(t, res, loadingCopy)

		assert.Nil(t, <-errCh)
		res, err = b.Draw(context.Background(), req)
		require.NoError(t, err)
		assertDrawResponse(t, res, testHandlerCopy)

		// no need to consume interrupt because previous
		// draw was consumed when state = ready.
		res, err = b.Draw(context.Background(), req)
		require.NoError(t, err)
		assertDrawResponse(t, res, testHandlerCopy)
	})

	t.Run("subsequent draw with different size re-issues new draw", func(t *testing.T) {
		mock := &mockHandlerClient{remote: testHandler()}
		b, errCh := withClientBreaker(mock)
		defer b.Close()

		res, err := b.Draw(context.Background(), req)
		require.NoError(t, err)
		assertDrawResponse(t, res, loadingCopy)

		biggerReq := &proto.DrawRequest{Width: 6, Height: 5}

		assert.Nil(t, <-errCh)
		res, err = b.Draw(context.Background(), biggerReq)
		require.NoError(t, err)
		assertDrawResponse(t, res, loadingCopy)

		assert.Nil(t, <-errCh)
		res, err = b.Draw(context.Background(), biggerReq)
		require.NoError(t, err)
		assertDrawResponse(t, res, "AAAAAA\nAAAAAA\nAAAAAA\nAAAAAA\nAAAAAA")
	})

	t.Run("dispatches draw errors to error chan", func(t *testing.T) {
		mock := &mockHandlerClient{remote: testHandler()}
		b, errCh := withClientBreaker(mock)
		defer b.Close()

		mock.rpcError = errors.New("Uh, Houstoun, we've had a problem")

		res, err := b.Draw(context.Background(), req)
		require.NoError(t, err)
		assertDrawResponse(t, res, loadingCopy)

		assert.Equal(t, mock.rpcError, <-errCh)
		res, err = b.Draw(context.Background(), req)
		require.NoError(t, err)
		assertDrawResponse(t, res, smtgWrongCopy)
	})
}
