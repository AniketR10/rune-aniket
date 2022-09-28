package rpc

import (
	"errors"
	"net"
	"syscall"
	"testing"

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
	server := NewServer(mockExecutor)
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
	}

	for _, tcase := range tsuite {
		t.Run(tcase.description, func(t *testing.T) {
			client, server, mock, cleanup := setupClientServerUnitTest(t)
			defer cleanup()
			tcase.do(t, mock, client, server)
		})
	}
}
