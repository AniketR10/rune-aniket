package workspace

import (
	"errors"
	"io"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	workspacepb "github.com/ernestrc/go-tui/workspace/proto"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func doSetupClientServerTest(
	t *testing.T, s *Server,
) (conn *grpc.ClientConn, closeFn func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	workspacepb.RegisterWorkspaceServer(grpcServer, s)

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

func setupClientServerUnitTest(t *testing.T) (*Client, *Server, *MockOsFile, func()) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := NewMockOsFile(ctrl)
	mockExecutor := NewMockExecutor(ctrl)
	server := NewServer(mockExecutor)
	server.openFunc = func(name string, flag int, perm os.FileMode) (osFile, *osError) {
		return mock, nil
	}
	server.removeFunc = func(name string) error {
		return nil
	}
	server.renameFunc = func(oldpath, newpath string) error {
		return nil
	}
	server.statFunc = func(name string) (os.FileInfo, error) {
		return nil, nil
	}
	server.lstatFunc = func(name string) (os.FileInfo, error) {
		return nil, nil
	}
	client, cleanup := setupClientServerTest(t, server)
	return client, server, mock, cleanup
}

func expectCommand(t *testing.T, s *Server, pid int) {
	s.executor.(*MockExecutor).EXPECT().Command(gomock.Any(), gomock.Any()).Return(Pid(pid), nil)
}

func TestClientServer(t *testing.T) {
	tsuite := []struct {
		description string
		do          func(*testing.T, *MockOsFile, *Client, *Server)
	}{
		{"Open happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			var called int
			s.openFunc = func(name string, flag int, perm os.FileMode) (osFile, *osError) {
				called++
				assert.Equal(t, "myFile", name)
				assert.Equal(t, os.FileMode(0666), perm)
				assert.True(t, flag&os.O_CREATE == os.O_CREATE)
				assert.True(t, flag&os.O_EXCL == os.O_EXCL)
				assert.True(t, flag&os.O_RDWR == os.O_RDWR)
				return mock, nil
			}

			_, err := c.Open("myFile", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
			require.NoError(t, err)
			assert.Equal(t, 1, called)
		}},
		{"Open unknown error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			s.openFunc = func(name string, flag int, perm os.FileMode) (osFile, *osError) {
				return nil, nopOsError(errors.New("boom"))
			}
			_, err := c.Open("myFile", 1, 1)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Remove happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			var called int
			s.removeFunc = func(name string) error {
				called++
				assert.Equal(t, "myFile", name)
				return nil
			}
			err := c.Remove("myFile")
			require.NoError(t, err)
			assert.Equal(t, 1, called)
		}},
		{"Remove error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			s.removeFunc = func(name string) error {
				return errors.New("boom")
			}
			err := c.Remove("myFile")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Rename happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			var called int
			s.renameFunc = func(name, name2 string) error {
				called++
				assert.Equal(t, "myFile", name)
				assert.Equal(t, "myFile2", name2)
				return nil
			}
			err := c.Rename("myFile", "myFile2")
			require.NoError(t, err)
			assert.Equal(t, 1, called)
		}},
		{"Rename error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			s.renameFunc = func(name, name2 string) error {
				return errors.New("boom")
			}
			err := c.Rename("myFile", "myFile2")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Stat happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			tt := time.Now()
			var called int
			s.statFunc = func(name string) (os.FileInfo, error) {
				called++
				assert.Equal(t, "myFile", name)
				return testFileInfo{name: "myFile", isDir: true, modTime: tt, size: 129, mode: 4}, nil
			}
			fi, err := c.Stat("myFile")
			require.NoError(t, err)
			assert.Equal(t, 1, called)

			assert.Equal(t, "myFile", fi.Name())
			assert.True(t, fi.IsDir())
			assert.Equal(t, tt.Unix(), fi.ModTime().Unix())
			assert.Equal(t, int64(129), fi.Size())
			assert.Equal(t, os.FileMode(4), fi.Mode())
		}},
		{"Stat error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			s.statFunc = func(name string) (os.FileInfo, error) {
				return nil, errors.New("boom")
			}
			_, err := c.Stat("myFile")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"LStat happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			tt := time.Now()
			var called int
			s.lstatFunc = func(name string) (os.FileInfo, error) {
				called++
				assert.Equal(t, "myFile", name)
				return testFileInfo{name: "myFile", isDir: true, modTime: tt, size: 129, mode: 4}, nil
			}
			fi, err := c.LStat("myFile")
			require.NoError(t, err)
			assert.Equal(t, 1, called)

			assert.Equal(t, "myFile", fi.Name())
			assert.True(t, fi.IsDir())
			assert.Equal(t, tt.Unix(), fi.ModTime().Unix())
			assert.Equal(t, int64(129), fi.Size())
			assert.Equal(t, os.FileMode(4), fi.Mode())
		}},
		{"LStat error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			s.lstatFunc = func(name string) (os.FileInfo, error) {
				return nil, errors.New("boom")
			}
			_, err := c.LStat("myFile")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Name happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Name().Return("myFile")
			name := f.Name()
			assert.Equal(t, "myFile", name)
		}},
		{"file Stat happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)
			tt := time.Now()
			var called int
			s.statFunc = func(name string) (os.FileInfo, error) {
				called++
				assert.Equal(t, "myFile", name)
				return testFileInfo{name: "myFile", isDir: true, modTime: tt, size: 129, mode: 4}, nil
			}
			fi, err := f.Stat()
			require.NoError(t, err)
			assert.Equal(t, 1, called)

			assert.Equal(t, "myFile", fi.Name())
			assert.True(t, fi.IsDir())
			assert.Equal(t, tt.Unix(), fi.ModTime().Unix())
			assert.Equal(t, int64(129), fi.Size())
			assert.Equal(t, os.FileMode(4), fi.Mode())
		}},
		{"file Stat error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			s.statFunc = func(name string) (os.FileInfo, error) {
				return nil, errors.New("boom")
			}

			_, err = f.Stat()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Sync happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Sync().Return(nil)
			err = f.Sync()
			require.NoError(t, err)
		}},
		{"file Sync error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Sync().Return(errors.New("boom"))
			err = f.Sync()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Truncate happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Truncate(gomock.Eq(int64(10))).Return(nil)
			err = f.Truncate(10)
			require.NoError(t, err)
		}},
		{"file Truncate error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Truncate(gomock.Any()).Return(errors.New("boom"))
			err = f.Truncate(10)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Seek happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Seek(gomock.Eq(int64(10)), gomock.Eq(1)).Return(int64(111), nil)
			actualOffset, err := f.Seek(10, 1)
			require.NoError(t, err)
			assert.Equal(t, int64(111), actualOffset)
		}},
		{"file Seek error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Seek(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("boom"))
			_, err = f.Seek(10, 1)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Close happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Close().Return(nil)
			err = f.Close()
			require.NoError(t, err)
		}},
		{"file Close error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Close().Return(errors.New("boom"))
			err = f.Close()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Read happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Read(gomock.Any()).DoAndReturn(func(b []byte) (n int, err error) {
				copy(b, "111111111")
				return 9, nil
			})
			b := make([]byte, 10)
			actualN, err := f.Read(b)
			require.NoError(t, err)
			assert.Equal(t, 9, actualN)
			assert.Equal(t, "111111111", string(b[:actualN]))
		}},
		{"file Read error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Read(gomock.Any()).Return(0, errors.New("boom"))
			_, err = f.Read(nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Read bubbles up io.EOF", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Read(gomock.Any()).Return(4, io.EOF)
			n, err := f.Read(make([]byte, 4))
			require.Equal(t, io.EOF, err)
			assert.Equal(t, 4, n)
		}},
		{"file Write happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			out := make([]byte, 9)
			mock.EXPECT().Write(gomock.Any()).DoAndReturn(func(b []byte) (n int, err error) {
				copy(out, b)
				return len(out), nil
			})
			actualN, err := f.Write([]byte("8888888888"))
			require.NoError(t, err)
			assert.Equal(t, 9, actualN)
			assert.Equal(t, "888888888", string(out))
		}},
		{"file Write error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			f, err := c.Open("myFile", 0, 0600)
			require.NoError(t, err)

			mock.EXPECT().Write(gomock.Any()).Return(0, errors.New("boom"))
			_, err = f.Write(nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"ReadLink happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			var called int
			s.readLinkFunc = func(name string) (string, error) {
				called++
				assert.Equal(t, "myFile", name)
				return "myFile.orig", nil
			}
			fil, err := c.ReadLink("myFile")
			require.NoError(t, err)
			assert.Equal(t, 1, called)
			assert.Equal(t, "myFile.orig", fil)
		}},
		{"ReadLink error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			s.readLinkFunc = func(name string) (string, error) {
				return "", errors.New("boom")
			}
			_, err := c.ReadLink("myFile")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Command happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			s.executor.(*MockExecutor).EXPECT().Command(gomock.Eq("six"), gomock.Eq("arg1")).Return(Pid(1), nil)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)
			assert.Equal(t, Pid(1), pid)
		}},
		{"Command error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			s.executor.(*MockExecutor).EXPECT().Command(gomock.Any(), gomock.Any()).
				Return(Pid(0), errors.New("boom"))
			_, err := c.Command("six", "arg1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Start happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().Start(gomock.Eq(Pid(99))).Return(nil)
			err = c.Start(pid)
			assert.NoError(t, err)
		}},
		{"Start error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().Start(gomock.Any()).Return(errors.New("boom"))
			err = c.Start(pid)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Wait happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().Wait(gomock.Eq(Pid(99))).Return(nil)
			err = c.Wait(pid)
			assert.NoError(t, err)
		}},
		{"Wait error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().Wait(gomock.Any()).Return(errors.New("boom"))
			err = c.Wait(pid)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Signal happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().Signal(gomock.Eq(Pid(99)), gomock.Eq(syscall.SIGTERM)).
				Return(nil)
			err = c.Signal(pid, syscall.SIGTERM)
			assert.NoError(t, err)
		}},
		{"Signal error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().Signal(gomock.Any(), gomock.Any()).
				Return(errors.New("boom"))
			err = c.Signal(pid, syscall.SIGKILL)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StdinPipe happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().StdinPipe(gomock.Eq(Pid(99))).Return(mock, nil)
			pipe, err := c.StdinPipe(pid)
			require.NoError(t, err)

			mock.EXPECT().Write(gomock.Eq([]byte("NYC is dirty"))).Return(10, nil)
			n, err := pipe.Write([]byte("NYC is dirty"))
			require.NoError(t, err)
			assert.Equal(t, 10, n)

			mock.EXPECT().Write(gomock.Eq([]byte("NYC is dirty"))).Return(0, errors.New("boom"))
			_, err = pipe.Write([]byte("NYC is dirty"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StdinPipe error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().StdinPipe(gomock.Any()).Return(nil, errors.New("boom"))
			_, err = c.StdinPipe(pid)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StdoutPipe happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().StdoutPipe(gomock.Eq(Pid(99))).Return(mock, nil)
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
		{"StdoutPipe error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().StdoutPipe(gomock.Any()).Return(nil, errors.New("boom"))
			_, err = c.StdoutPipe(pid)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"StderrPipe happy path", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().StderrPipe(gomock.Eq(Pid(99))).Return(mock, nil)
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
		{"StderrPipe error", func(t *testing.T, mock *MockOsFile, c *Client, s *Server) {
			expectCommand(t, s, 99)
			pid, err := c.Command("six", "arg1")
			require.NoError(t, err)

			s.executor.(*MockExecutor).EXPECT().StderrPipe(gomock.Any()).Return(nil, errors.New("boom"))
			_, err = c.StderrPipe(pid)
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
