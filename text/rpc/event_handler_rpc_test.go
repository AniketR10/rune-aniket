package rpc

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	textapi "unstable.build/go-tui/api/text"
	texttest "unstable.build/go-tui/text/test"
)

func newClientServerIntegration(
	t *testing.T, h *texttest.MockEventHandler,
	serverQuitCallback func(context.Context),
	clientQuitCallback func(),
) (*eventHandlerClient, func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	server := newEventHandlerServer(h, serverQuitCallback)
	RegisterEditorEventHandlerServer(grpcServer, server)

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	client := newEventHandlerClient(conn, clientQuitCallback)

	closeFn := func() {
		client.Close()
		grpcServer.Stop()
		conn.Close()
	}

	return client, closeFn
}

func consumeError(t *testing.T, wg *sync.WaitGroup, client *eventHandlerClient) {
	select {
	case err := <-client.errors():
		wg.Done()
		assert.NoError(t, err)
	case <-client.quitChan:
	}
}

func TestEventHandlerRPC(t *testing.T) {
	content := "myContent"
	ev := textapi.Event{
		Type:     textapi.EventTypeFlush,
		URI:      uri,
		Resource: Token{URI: uri},
		Content:  content,
	}

	t.Run("asynchronously dispatches events to remote event handler", func(t *testing.T) {
		var wg sync.WaitGroup
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := texttest.NewMockEventHandler(ctrl)

		client, closeFn := newClientServerIntegration(t, h,
			func(context.Context) {}, func() {})
		defer closeFn()

		go consumeError(t, &wg, client)

		h.EXPECT().Handle(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ev textapi.Event) bool {
				assert.Equal(t, ev, _ev)
				wg.Done()
				return false
			}).Times(1)

		wg.Add(1)
		exit := client.Handle(context.Background(), ev)
		assert.False(t, exit)

		wg.Wait()
	})

	t.Run("client/server call exit callback if remote handler returns true to a Handle call", func(t *testing.T) {
		var wg sync.WaitGroup

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := texttest.NewMockEventHandler(ctrl)

		client, closeFn := newClientServerIntegration(t, h,
			func(context.Context) { wg.Done() }, wg.Done)
		defer closeFn()

		go consumeError(t, &wg, client)

		h.EXPECT().Handle(gomock.Any(), gomock.Eq(ev)).Return(true).Times(1)

		wg.Add(2)
		exit := client.Handle(context.Background(), ev)
		assert.False(t, exit)

		wg.Wait()
	})

	t.Run("dispatches errors through error channel", func(t *testing.T) {
		var wg sync.WaitGroup
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		h := texttest.NewMockEventHandler(ctrl)

		client, closeFn := newClientServerIntegration(t, h,
			func(context.Context) {}, func() {})
		defer closeFn()

		go func() {
			defer wg.Done()
			err := <-client.errors()
			assert.Error(t, err)
		}()

		// should cause failure
		client.conn.(io.Closer).Close()
		time.Sleep(gracefulShutdownWait)

		wg.Add(1)
		exit := client.Handle(context.Background(), ev)
		assert.False(t, exit)

		wg.Wait()
	})
}
