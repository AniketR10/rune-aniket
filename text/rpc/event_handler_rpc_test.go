package rpc

import (
	"context"
	"net"
	"sync"
	"testing"

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
) (*eventHandlerClient, func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	server := newEventHandlerServer(h, serverQuitCallback)
	RegisterEditorEventHandlerServer(grpcServer, server)

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	client := newEventHandlerClient(context.Background(), "", conn)

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
			func(context.Context) {})
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
}
