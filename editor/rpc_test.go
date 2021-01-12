package editor

import (
	"errors"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/proto"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func setupIntTest(
	t *testing.T, broker proto.MuxBroker, s *Server,
) (client *Client, closeFn func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	proto.RegisterEditorServer(grpcServer, s)

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	client = NewClient(broker, conn, new(sync.Mutex))
	closeFn = func() {
		client.Close()
		grpcServer.Stop()
	}
	return
}

func TestClientServerIntegration(t *testing.T) {
	t.Run("client through server calls underlying editor Edit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		b := proto.NewDialBroker()
		ed := NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{}, func() {}, func() {})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		expectEdit(t, ed, "zion", "hero")
		buf := cell.NewBuffer()
		buf.WriteString("hero")

		_, err := client.Edit("zion", buf)
		require.NoError(t, err)
	})

	t.Run("underlying editor errors bubbles up to client", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		b := proto.NewDialBroker()
		ed := NewMockEditor(ctrl)
		s := NewServer(b, ed, nopLocker{}, func() {}, func() {})

		client, closeFn := setupIntTest(t, b, s)
		defer closeFn()

		ed.EXPECT().Edit(gomock.Eq("babylon"), gomock.Any()).
			Return(nil, errors.New("The Upsetter")).
			Times(1)

		_, err := client.Edit("babylon", cell.NewBuffer())
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "The Upsetter"))
	})
}
