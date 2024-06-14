package rpc

import (
	"context"
	"errors"
	"io"

	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceapitest "unstable.build/go-tui/api/workspace/test"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/test"
	workspacetest "unstable.build/go-tui/workspace/test"
)

func doSetupClientServerTest(
	t *testing.T, s *Server,
) (conn *grpc.ClientConn, closeFn func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	RegisterFilesServer(grpcServer, s)
	RegisterExecutorServer(grpcServer, s)
	RegisterTerminalServer(grpcServer, s)
	RegisterSchemeServer(grpcServer, s)

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

func setupClientServerUnitTest(t *testing.T) (*Client, *Server, *workspaceapitest.MockFile, func()) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := workspaceapitest.NewMockFile(ctrl)
	mockExecutor := workspacetest.NewMockWorkspace(ctrl)
	server := NewServer(mockExecutor, new(sync.Mutex))
	client, cleanup := setupClientServerTest(t, server)
	return client, server, mock, cleanup
}

func expectCommand(t *testing.T, s *Server, pid int) {
	s.s.(*workspacetest.MockWorkspace).EXPECT().
		StartCommand(gomock.Any(), gomock.Any()).
		Return(workspaceapi.Pid(pid), nil)
}

func TestClientServer(t *testing.T) {
	ctx := context.Background()

	tsuite := []struct {
		description string
		do          func(*testing.T, *workspaceapitest.MockFile, *Client, *Server)
	}{
		{"Command happy path", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				StartCommand(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
					assert.Equal(t, "six", cmd.Path)
					assert.Equal(t, []string{"arg1"}, cmd.Args)
					return workspaceapi.Pid(1), nil
				})
			pid, err := c.StartCommand(ctx, workspaceapi.Cmd{
				Path: "six",
				Args: []string{"arg1"},
			})
			require.NoError(t, err)
			assert.Equal(t, workspaceapi.Pid(1), pid)
		}},
		{"Command Dir is passed from client to server", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				StartCommand(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
					assert.Equal(t, "six", cmd.Path)
					assert.Equal(t, []string{"arg1"}, cmd.Args)
					assert.Equal(t, "/tmp", cmd.Dir)
					return workspaceapi.Pid(1), nil
				})
			pid, err := c.StartCommand(ctx, workspaceapi.Cmd{
				Path: "six",
				Args: []string{"arg1"},
				Dir:  "/tmp",
			})
			require.NoError(t, err)
			assert.Equal(t, workspaceapi.Pid(1), pid)
		}},
		{"Command error", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				StartCommand(gomock.Any(), gomock.Any()).
				Return(workspaceapi.Pid(0), errors.New("boom"))

			_, err := c.StartCommand(ctx, workspaceapi.Cmd{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Signal happy path", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.StartCommand(ctx, workspaceapi.Cmd{
				Path: "six",
				Args: []string{"arg1"},
			})
			require.NoError(t, err)

			s.s.(*workspacetest.MockWorkspace).EXPECT().
				Signal(gomock.Eq(workspaceapi.Pid(99)), gomock.Eq(syscall.SIGTERM)).
				Return(nil)

			err = c.Signal(pid, syscall.SIGTERM)
			assert.NoError(t, err)
		}},
		{"Signal error", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.StartCommand(ctx, workspaceapi.Cmd{
				Path: "six",
				Args: []string{"arg1"},
			})
			require.NoError(t, err)

			s.s.(*workspacetest.MockWorkspace).EXPECT().
				Signal(gomock.Any(), gomock.Any()).
				Return(errors.New("boom"))

			err = c.Signal(pid, syscall.SIGKILL)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"URI happy path", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			uri, err := workspaceapi.ParseURI("ssh://user@my_host:8080/tmp/hello/world.go")
			require.NoError(t, err)

			s.s.(*workspacetest.MockWorkspace).EXPECT().
				URI(gomock.Eq("/tmp/hello_world.go")).Return(uri, nil)

			actualUri, err := c.URI("/tmp/hello_world.go")
			assert.NoError(t, err)
			assert.Equal(t, uri.String(), actualUri.String())
		}},
		{"URI error", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				URI(gomock.Any()).Return(workspaceapi.URI{}, errors.New("boom"))

			_, err := c.URI("/tmp/hello_world.go")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Remove happy path", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				Remove(gomock.Eq("/tmp/hello_world.go")).
				Return(nil)

			err := c.Remove("/tmp/hello_world.go")
			assert.NoError(t, err)
		}},
		{"Remove error", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				Remove(gomock.Eq("/tmp/hello_world.go")).
				Return(errors.New("pow"))

			err := c.Remove("/tmp/hello_world.go")
			require.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), "pow"))
		}},
		{"Open happy path", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				Open(gomock.Eq("/tmp/hello_world.go"), gomock.Eq(os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_SYNC|os.O_TRUNC), gomock.Eq(os.FileMode(0666))).
				Return(testFile{}, nil)

			f, err := c.Open("/tmp/hello_world.go", os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_SYNC|os.O_TRUNC, 0666)
			assert.Nil(t, err)
			assert.NotNil(t, f)
		}},
		{"Open error", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				Open(gomock.Eq("/tmp/hello_world.go"), gomock.Eq(os.O_RDONLY), gomock.Eq(os.FileMode(2))).
				Return(nil, &workspaceapi.Error{Err: errors.New("pow")})

			f, err := c.Open("/tmp/hello_world.go", os.O_RDONLY, 2)
			require.NotNil(t, err)
			assert.True(t, strings.Contains(err.Err.Error(), "pow"))
			assert.Nil(t, f)
		}},
		{"NewPty happy path", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			ctrl := gomock.NewController(t)
			mockFile := workspaceapitest.NewMockFile(ctrl)
			mockFile.EXPECT().Name().Return("bla").AnyTimes()
			mockFile.EXPECT().Fd().Return(uintptr(99)).AnyTimes()
			slaveMockFile := workspaceapitest.NewMockFile(ctrl)
			slaveMockFile.EXPECT().Name().Return("blo").AnyTimes()
			slaveMockFile.EXPECT().Fd().Return(uintptr(199)).AnyTimes()
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				NewPty(gomock.Any()).
				Return(workspaceapi.Pty{Master: mockFile, Slave: slaveMockFile}, nil)

			pty, err := c.NewPty(ctx)
			require.NoError(t, err)

			mockFileForRead := workspaceapitest.NewMockFile(ctrl)
			mockFileForRead.EXPECT().Name().Return("bla").AnyTimes()
			mockFileForRead.EXPECT().Fd().Return(uintptr(99)).AnyTimes()
			mockFileForRead.EXPECT().Close().Return(nil).
				AnyTimes( /* Close runs in runtime.Finalizer */ )
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				NewFile(gomock.Any(), gomock.Any()).
				Return(mockFileForRead).Times(1)
			mockFileForRead.EXPECT().Read(gomock.Any()).DoAndReturn(func(b []byte) (n int, err error) {
				b[0] = []byte("a")[0]
				return 1, io.EOF
			})
			b, err := io.ReadAll(pty.Master)
			require.NoError(t, err)
			assert.Equal(t, "a", string(b))
			assert.Equal(t, uintptr(99), pty.Master.Fd())

			s.s.(*workspacetest.MockWorkspace).EXPECT().
				NewFile(gomock.Any(), gomock.Any()).
				Return(mockFile).Times(1)
			mockFile.EXPECT().Close().Return(nil).Times(1)
			assert.NoError(t, pty.Master.Close())

			// cannot test slave mock file due to having to intercept via
			// MockWorkspace above. See AnyTimes() details.
			// slaveMockFile.EXPECT().Close().Return(nil)
			// assert.NoError(t, pty.Slave.Close())
		}},
		{"NewPty error", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				NewPty(gomock.Any()).
				Return(workspaceapi.Pty{}, errors.New("bummer"))

			_, err := c.NewPty(ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "bummer")
		}},
		{"SetPtySize happy path", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			ctrl := gomock.NewController(t)
			mockFile := workspaceapitest.NewMockFile(ctrl)
			mockFile.EXPECT().Name().Return("bla").AnyTimes()
			mockFile.EXPECT().Fd().Return(uintptr(1)).AnyTimes()
			slaveMockFile := workspaceapitest.NewMockFile(ctrl)
			slaveMockFile.EXPECT().Name().Return("blo").AnyTimes()
			slaveMockFile.EXPECT().Fd().Return(uintptr(2)).AnyTimes()
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				NewPty(gomock.Any()).
				Return(workspaceapi.Pty{Slave: slaveMockFile, Master: mockFile}, nil)
			pty, err := c.NewPty(ctx)
			require.NoError(t, err)

			s.s.(*workspacetest.MockWorkspace).EXPECT().
				NewFile(gomock.Any(), gomock.Any()).
				Return(mockFile).AnyTimes()

			mockFile.EXPECT().Close().Return(nil).
				AnyTimes( /* Close runs in runtime.Finalizer */ )

			s.s.(*workspacetest.MockWorkspace).EXPECT().
				SetPtySize(gomock.Any(), gomock.Eq(1), gomock.Eq(1)).
				DoAndReturn(func(pty workspaceapi.Pty, width, height int) error {
					assert.Equal(t, 1, width)
					assert.Equal(t, 1, height)
					// must be exact instance returned by underlying Scheme
					// or else certain implementations might fail
					assert.Equal(t, mockFile, pty.Master)
					return nil
				})
			err = c.SetPtySize(pty, 1, 1)
			assert.NoError(t, err)
		}},
		{"SetPtySize error", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			ctrl := gomock.NewController(t)
			mockFile := workspaceapitest.NewMockFile(ctrl)
			mockFile.EXPECT().Name().Return("bla").AnyTimes()
			mockFile.EXPECT().Fd().Return(uintptr(1)).AnyTimes()
			slaveMockFile := workspaceapitest.NewMockFile(ctrl)
			slaveMockFile.EXPECT().Name().Return("blo").AnyTimes()
			slaveMockFile.EXPECT().Fd().Return(uintptr(11)).AnyTimes()
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				NewPty(gomock.Any()).
				Return(workspaceapi.Pty{Slave: slaveMockFile, Master: mockFile}, nil)
			pty, err := c.NewPty(ctx)
			require.NoError(t, err)

			s.s.(*workspacetest.MockWorkspace).EXPECT().
				NewFile(gomock.Any(), gomock.Any()).
				Return(mockFile).AnyTimes()

			s.s.(*workspacetest.MockWorkspace).EXPECT().
				SetPtySize(gomock.Any(), gomock.Eq(1), gomock.Eq(1)).
				Return(errors.New("bummer"))
			err = c.SetPtySize(pty, 1, 1)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "bummer")
		}},
		{"Remove error", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				Remove(gomock.Eq("/tmp/hello_world.go")).
				Return(errors.New("pow"))

			err := c.Remove("/tmp/hello_world.go")
			require.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), "pow"))
		}},
		{"ReadDir happy path", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				ReadDir(gomock.Any()).
				DoAndReturn(func(root string) ([]os.DirEntry, error) {
					return []os.DirEntry{dirEntry{name: "a"}}, nil
				})
			dirs, err := c.ReadDir("")
			require.NoError(t, err)
			require.Len(t, dirs, 1)
			assert.Equal(t, "a", dirs[0].Name())
		}},
		{"ReadDir error", func(t *testing.T, mock *workspaceapitest.MockFile, c *Client, s *Server) {
			s.s.(*workspacetest.MockWorkspace).EXPECT().
				ReadDir(gomock.Any()).
				DoAndReturn(func(string) ([]os.DirEntry, error) {
					return nil, errors.New("boom")
				})
			l, err := c.ReadDir("")
			require.Error(t, err)
			assert.Nil(t, l)
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

func setupClientServerIntegrationTest(
	t *testing.T, scheme schemeapi.Scheme,
) (*Client, func()) {
	server := NewServer(scheme, new(sync.Mutex))
	return setupClientServerTest(t, server)
}

func TestSchemeIntegration(t *testing.T) {
	test.TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
		memURI, err := workspaceapi.ParseURI("memory:///tmp")
		require.NoError(t, err)
		scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), memURI)
		require.NoError(t, err)
		client, cleanup := setupClientServerIntegrationTest(t, scheme)
		t.Cleanup(cleanup)
		return client
	})

	test.TestWorkspaceSchemeExecutor(t, func(t *testing.T) schemeapi.Scheme {
		dir, err := os.MkdirTemp("", "workspacepb_suite")
		require.NoError(t, err)

		workspaceURI, err := workspaceapi.ParseURI("file://" + dir)
		require.NoError(t, err)

		fileScheme, err := workspace.NewFileScheme(
			context.Background(), config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		client, cleanup := setupClientServerIntegrationTest(t, fileScheme)
		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
			cleanup()
		})
		return client
	})
}

type testFile struct {
}

func (t testFile) Name() string {
	return ""
}

func (t testFile) Stat() (os.FileInfo, error) {
	panic("unimplemented")
}

func (t testFile) Sync() error {
	return nil
}
func (t testFile) Truncate(size int64) error {
	return nil
}

func (t testFile) Seek(x int64, y int) (int64, error) {
	return 0, nil
}

func (t testFile) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (t testFile) Write(b []byte) (int, error) {
	return 0, nil
}

func (t testFile) Fd() uintptr {
	return 0
}

func (t testFile) Close() error {
	return nil
}
