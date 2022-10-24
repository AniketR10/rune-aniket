package rpc

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/ernestrc/blue/iterator"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui/workspace"
	workspacetest "unstable.build/go-tui/workspace/test"
)

func doSetupClientServerTest(
	t *testing.T, s *Server,
) (conn *grpc.ClientConn, closeFn func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	RegisterWorkspaceServer(grpcServer, s)

	go grpcServer.Serve(lis)

	conn, err = grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	closeFn = func() {
		grpcServer.Stop()
		lis.Close()
	}
	return
}

func setupClientServerTest(
	t *testing.T, s *Server,
) (*Client, func()) {
	conn, closeFn := doSetupClientServerTest(t, s)
	client := NewClient(conn)
	return client, func() {
		client.Close()
		closeFn()
	}
}

func setupClientServerUnitTest(t *testing.T) (*Client, *Server, *workspace.MockOsFile, func()) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := workspace.NewMockOsFile(ctrl)
	mockExecutor := workspacetest.NewMockWorkspace(ctrl)
	server := NewServer(mockExecutor, new(sync.Mutex))
	client, cleanup := setupClientServerTest(t, server)
	return client, server, mock, cleanup
}

func expectCommand(t *testing.T, s *Server, pid int) {
	s.wp.(*workspacetest.MockWorkspace).EXPECT().
		Command(gomock.Any(), gomock.Any()).
		Return(workspace.Pid(pid), nil)
}

func TestClientServer(t *testing.T) {
	tsuite := []struct {
		description string
		do          func(*testing.T, *workspace.MockOsFile, *Client, *Server)
	}{
		{"Command happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Command(gomock.Eq("six"), gomock.Eq("arg1")).Return(workspace.Pid(1), nil)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)
			assert.Equal(t, workspace.Pid(1), pid)
		}},
		{"Command error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Command(gomock.Any(), gomock.Any()).
				Return(workspace.Pid(0), errors.New("boom"))

			_, err := c.Command("six", "arg1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Start happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Start(gomock.Eq(workspace.Pid(99))).Return(nil)

			err = c.Start(pid)
			assert.NoError(t, err)
		}},
		{"Start error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Start(gomock.Any()).Return(errors.New("boom"))

			err = c.Start(pid)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Wait happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Wait(gomock.Eq(workspace.Pid(99))).Return(nil)

			err = c.Wait(pid)
			assert.NoError(t, err)
		}},
		{"Wait error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Wait(gomock.Any()).Return(errors.New("boom"))

			err = c.Wait(pid)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Signal happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Signal(gomock.Eq(workspace.Pid(99)), gomock.Eq(syscall.SIGTERM)).
				Return(nil)

			err = c.Signal(pid, syscall.SIGTERM)
			assert.NoError(t, err)
		}},
		{"Signal error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Signal(gomock.Any(), gomock.Any()).
				Return(errors.New("boom"))

			err = c.Signal(pid, syscall.SIGKILL)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StdinPipe happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				StdinPipe(gomock.Eq(workspace.Pid(99))).Return(mock, nil)

			pipe, err := c.StdinPipe(pid)
			require.NoError(t, err)

			mock.EXPECT().
				Write(gomock.Eq([]byte("NYC is dirty"))).Return(10, nil)

			n, err := pipe.Write([]byte("NYC is dirty"))
			require.NoError(t, err)
			assert.Equal(t, 10, n)

			mock.EXPECT().
				Write(gomock.Eq([]byte("NYC is dirty"))).Return(0, errors.New("boom"))

			_, err = pipe.Write([]byte("NYC is dirty"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StdinPipe error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				StdinPipe(gomock.Any()).Return(nil, errors.New("boom"))

			_, err = c.StdinPipe(pid)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StdoutPipe happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				StdoutPipe(gomock.Eq(workspace.Pid(99))).Return(mock, nil)

			pipe, err := c.StdoutPipe(pid)
			require.NoError(t, err)

			mock.EXPECT().Read(gomock.Any()).Return(10, nil)
			buf := make([]byte, 10)
			n, err := pipe.Read(buf)
			require.NoError(t, err)
			assert.Equal(t, 10, n)

			mock.EXPECT().Read(gomock.Any()).Return(0, errors.New("boom"))
			_, err = pipe.Read([]byte{})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StdoutPipe error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				StdoutPipe(gomock.Any()).Return(nil, errors.New("boom"))

			_, err = c.StdoutPipe(pid)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StderrPipe happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				StderrPipe(gomock.Eq(workspace.Pid(99))).Return(mock, nil)

			pipe, err := c.StderrPipe(pid)
			require.NoError(t, err)

			mock.EXPECT().Read(gomock.Any()).Return(10, nil)

			buf := make([]byte, 10)
			n, err := pipe.Read(buf)
			require.NoError(t, err)
			assert.Equal(t, 10, n)

			mock.EXPECT().Read(gomock.Any()).Return(0, errors.New("boom"))

			_, err = pipe.Read([]byte{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StderrPipe error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				StderrPipe(gomock.Any()).Return(nil, errors.New("boom"))

			_, err = c.StderrPipe(pid)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"URI happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			uri, err := workspace.ParseURI("ssh://user@my_host:8080/tmp/hello/world.go")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				URI(gomock.Eq("/tmp/hello_world.go")).Return(uri, nil)

			actualUri, err := c.URI("/tmp/hello_world.go")
			assert.NoError(t, err)
			assert.Equal(t, uri.String(), actualUri.String())
		}},
		{"URI error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				URI(gomock.Any()).Return(workspace.URI{}, errors.New("boom"))

			_, err := c.URI("/tmp/hello_world.go")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Getwd happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			uri, err := workspace.ParseURI("ssh://user@my_host:8080/tmp/hello/world.go")
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Getwd().Return(uri, nil)

			actualUri, err := c.Getwd()
			assert.NoError(t, err)
			assert.Equal(t, uri.String(), actualUri.String())
		}},
		{"Getwd error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Getwd().Return(workspace.URI{}, errors.New("boom"))

			_, err := c.Getwd()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Remove happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Remove(gomock.Eq("/tmp/hello_world.go")).
				Return(nil)

			err := c.Remove("/tmp/hello_world.go")
			assert.NoError(t, err)
		}},
		{"Remove error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Remove(gomock.Eq("/tmp/hello_world.go")).
				Return(errors.New("pow"))

			err := c.Remove("/tmp/hello_world.go")
			require.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), "pow"))
		}},
		{"Open happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Open(gomock.Eq("/tmp/hello_world.go"), gomock.Eq(os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_SYNC|os.O_TRUNC), gomock.Eq(os.FileMode(0666))).
				Return(nil, nil)

			f, err := c.Open("/tmp/hello_world.go", os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_SYNC|os.O_TRUNC, 0666)
			assert.Nil(t, err)
			assert.NotNil(t, f)
		}},
		{"Open error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Open(gomock.Eq("/tmp/hello_world.go"), gomock.Eq(os.O_RDONLY), gomock.Eq(os.FileMode(2))).
				Return(nil, &workspace.Error{Err: errors.New("pow")})

			f, err := c.Open("/tmp/hello_world.go", os.O_RDONLY, 2)
			require.NotNil(t, err)
			assert.True(t, strings.Contains(err.Err.Error(), "pow"))
			assert.Nil(t, f)
		}},
		{"NewPty happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			ctrl := gomock.NewController(t)
			mockFile := workspacetest.NewMockFile(ctrl)
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				NewPty().
				Return(workspace.Pty{Pid: 1, Master: mockFile, Slave: "follower"}, nil)

			pty, err := c.NewPty()
			require.NoError(t, err)
			assert.Equal(t, workspace.Pid(1), pty.Pid)
			assert.Equal(t, "follower", pty.Slave)

			mockFile.EXPECT().Read(gomock.Any()).DoAndReturn(func(b []byte) (n int, err error) {
				b[0] = []byte("a")[0]
				return 1, io.EOF
			})
			b, err := ioutil.ReadAll(pty.Master)
			require.NoError(t, err)
			assert.Equal(t, "a", string(b))

			mockFile.EXPECT().Close().Return(nil)
			assert.NoError(t, pty.Master.Close())
		}},
		{"NewPty error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				NewPty().
				Return(workspace.Pty{}, errors.New("bummer"))

			_, err := c.NewPty()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "bummer")
		}},
		{"SetPtySize happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			ctrl := gomock.NewController(t)
			mockFile := workspacetest.NewMockFile(ctrl)
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				NewPty().
				Return(workspace.Pty{Pid: 1, Slave: "one", Master: mockFile}, nil)
			pty, err := c.NewPty()
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				SetPtySize(gomock.Any(), gomock.Eq(1), gomock.Eq(1)).
				DoAndReturn(func(pty workspace.Pty, width, height int) error {
					assert.Equal(t, "one", pty.Slave)
					assert.Equal(t, workspace.Pid(1), pty.Pid)
					assert.Equal(t, 1, width)
					assert.Equal(t, 1, height)
					assert.NotNil(t, pty.Master)
					return nil
				})
			err = c.SetPtySize(pty, 1, 1)
			assert.NoError(t, err)
		}},
		{"SetPtySize error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			ctrl := gomock.NewController(t)
			mockFile := workspacetest.NewMockFile(ctrl)
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				NewPty().
				Return(workspace.Pty{Master: mockFile}, nil)
			pty, err := c.NewPty()
			require.NoError(t, err)

			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				SetPtySize(gomock.Any(), gomock.Eq(1), gomock.Eq(1)).
				Return(errors.New("bummer"))
			err = c.SetPtySize(pty, 1, 1)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "bummer")
		}},
		{"Remove error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				Remove(gomock.Eq("/tmp/hello_world.go")).
				Return(errors.New("pow"))

			err := c.Remove("/tmp/hello_world.go")
			require.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), "pow"))
		}},
		{"ListFiles happy path", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				ListFiles(gomock.Any()).
				DoAndReturn(func(context.Context) (iterator.Iterator[string], error) {
					return iterator.FromSlice([]string{"a"}), nil
				})
			it, err := c.ListFiles(context.Background())
			require.NoError(t, err)
			path, ok, err := it.Next()
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, "a", path)

			path, ok, err = it.Next()
			require.NoError(t, err)
			require.False(t, ok)
			assert.Zero(t, path)
		}},
		{"ListFiles error", func(t *testing.T, mock *workspace.MockOsFile, c *Client, s *Server) {
			s.wp.(*workspacetest.MockWorkspace).EXPECT().
				ListFiles(gomock.Any()).
				DoAndReturn(func(context.Context) (iterator.Iterator[string], error) {
					return nil, errors.New("boom")
				})
			l, err := c.ListFiles(context.Background())
			require.NoError(t, err)
			require.NotNil(t, l)
			_, _, err = l.Next()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.description, func(t *testing.T) {
			client, server, mock, cleanup := setupClientServerUnitTest(t)
			defer cleanup()
			tcase.do(t, mock, client, server)
		})
	}
}
