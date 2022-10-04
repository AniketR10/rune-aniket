package rpc

import (
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui/workspace"
	workspacetest "unstable.build/go-tui/workspace/test"
)

func doSetupSchemeClientServerTest(
	t *testing.T, s *SchemeServerImpl,
) (conn *grpc.ClientConn, closeFn func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
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

func setupSchemeClientServerTest(
	t *testing.T, s *SchemeServerImpl,
) (workspace.Scheme, func()) {
	conn, closeFn := doSetupSchemeClientServerTest(t, s)
	client := NewScheme(conn)
	return client, func() {
		client.Close()
		closeFn()
	}
}

func setupSchemeClientServerUnitTest(t *testing.T, ctrl *gomock.Controller) (
	workspace.Scheme, *SchemeServerImpl, func(),
) {

	mockScheme := workspacetest.NewMockScheme(ctrl)
	server := NewSchemeServer(mockScheme, new(sync.Mutex))
	client, cleanup := setupSchemeClientServerTest(t, server)
	return client, server, cleanup
}

func TestSchemeClientServer(t *testing.T) {
	testSchemeClientServer(t, func(t *testing.T, ctrl *gomock.Controller) (workspace.Scheme, *workspacetest.MockScheme, func()) {
		client, server, cleanup := setupSchemeClientServerUnitTest(t, ctrl)
		return client, server.scheme.(*workspacetest.MockScheme), cleanup
	})
}

func testSchemeClientServer(
	t *testing.T,
	fn func(*testing.T, *gomock.Controller) (workspace.Scheme, *workspacetest.MockScheme, func()),
) {
	tsuite := []struct {
		description string
		do          func(*testing.T, *gomock.Controller, workspace.Scheme, *workspacetest.MockScheme)
	}{
		{"Open happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			var called int
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(name string, flag int, perm os.FileMode) (
					workspace.File, *workspace.Error,
				) {
					called++
					assert.Equal(t, "myFile", name)
					assert.Equal(t, os.FileMode(0666), perm)
					assert.True(t, flag&os.O_CREATE == os.O_CREATE)
					assert.True(t, flag&os.O_EXCL == os.O_EXCL)
					assert.True(t, flag&os.O_RDWR == os.O_RDWR)
					return mock, nil
				})

			_, err := c.Open("myFile", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
			require.Nil(t, err)
			assert.Equal(t, 1, called)
		}},
		{"Open unknown error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(name string, flag int, perm os.FileMode) (workspace.File, *workspace.Error) {
					return nil, workspace.NopError(errors.New("boom"))
				})
			_, err := c.Open("myFile", 1, 1)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Remove happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			var called int
			s.EXPECT().
				Remove(gomock.Any()).
				DoAndReturn(func(name string) error {
					called++
					assert.Equal(t, "myFile", name)
					return nil
				})
			err := c.Remove("myFile")
			require.NoError(t, err)
			assert.Equal(t, 1, called)
		}},
		{"Remove error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			s.EXPECT().
				Remove(gomock.Any()).
				DoAndReturn(func(name string) error {
					return errors.New("boom")
				})
			err := c.Remove("myFile")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Rename happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			var called int
			s.EXPECT().
				Rename(gomock.Any(), gomock.Any()).
				DoAndReturn(func(name, name2 string) error {
					called++
					assert.Equal(t, "myFile", name)
					assert.Equal(t, "myFile2", name2)
					return nil
				})
			err := c.Rename("myFile", "myFile2")
			require.NoError(t, err)
			assert.Equal(t, 1, called)
		}},
		{"Rename error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			s.EXPECT().
				Rename(gomock.Any(), gomock.Any()).
				DoAndReturn(func(name, name2 string) error {
					return errors.New("boom")
				})
			err := c.Rename("myFile", "myFile2")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Stat happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			tt := time.Now()
			var called int
			s.EXPECT().
				Stat(gomock.Any()).
				DoAndReturn(func(name string) (os.FileInfo, error) {
					called++
					assert.Equal(t, "myFile", name)
					return testFileInfo{name: "myFile", isDir: true, modTime: tt, size: 129, mode: 4}, nil
				})
			fi, err := c.Stat("myFile")
			require.NoError(t, err)
			assert.Equal(t, 1, called)

			assert.Equal(t, "myFile", fi.Name())
			assert.True(t, fi.IsDir())
			assert.Equal(t, tt.Unix(), fi.ModTime().Unix())
			assert.Equal(t, int64(129), fi.Size())
			assert.Equal(t, os.FileMode(4), fi.Mode())
		}},
		{"Stat error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			s.EXPECT().
				Stat(gomock.Any()).
				DoAndReturn(func(name string) (os.FileInfo, error) {
					return nil, errors.New("boom")
				})
			_, err := c.Stat("myFile")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"LStat happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			tt := time.Now()
			var called int
			s.EXPECT().
				Lstat(gomock.Any()).
				DoAndReturn(func(name string) (os.FileInfo, error) {
					called++
					assert.Equal(t, "myFile", name)
					return testFileInfo{name: "myFile", isDir: true, modTime: tt, size: 129, mode: 4}, nil
				})
			fi, err := c.Lstat("myFile")
			require.NoError(t, err)
			assert.Equal(t, 1, called)

			assert.Equal(t, "myFile", fi.Name())
			assert.True(t, fi.IsDir())
			assert.Equal(t, tt.Unix(), fi.ModTime().Unix())
			assert.Equal(t, int64(129), fi.Size())
			assert.Equal(t, os.FileMode(4), fi.Mode())
		}},
		{"LStat error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			s.EXPECT().
				Lstat(gomock.Any()).
				DoAndReturn(func(name string) (os.FileInfo, error) {
					return nil, errors.New("boom")
				})
			_, err := c.Lstat("myFile")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Name happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, err := c.Open("myFile", 0, 0600)
			require.Nil(t, err)

			name := f.Name()
			assert.Equal(t, "myFile", name)
		}},
		{"file Stat happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)
			tt := time.Now()
			var called int
			s.EXPECT().
				Stat(gomock.Any()).
				DoAndReturn(func(name string) (os.FileInfo, error) {
					called++
					assert.Equal(t, "myFile", name)
					return testFileInfo{name: "myFile", isDir: true, modTime: tt, size: 129, mode: 4}, nil
				})
			fi, err := f.Stat()
			require.NoError(t, err)
			assert.Equal(t, 1, called)

			assert.Equal(t, "myFile", fi.Name())
			assert.True(t, fi.IsDir())
			assert.Equal(t, tt.Unix(), fi.ModTime().Unix())
			assert.Equal(t, int64(129), fi.Size())
			assert.Equal(t, os.FileMode(4), fi.Mode())
		}},
		{"file Stat error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			s.EXPECT().
				Stat(gomock.Any()).
				DoAndReturn(func(name string) (os.FileInfo, error) {
					return nil, errors.New("boom")
				})

			_, err := f.Stat()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Sync happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Sync().Return(nil)
			err := f.Sync()
			require.NoError(t, err)
		}},
		{"file Sync error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Sync().Return(errors.New("boom"))
			err := f.Sync()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Truncate happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Truncate(gomock.Eq(int64(10))).Return(nil)
			err := f.Truncate(10)
			require.NoError(t, err)
		}},
		{"file Truncate error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Truncate(gomock.Any()).Return(errors.New("boom"))
			err := f.Truncate(10)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Seek happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Seek(gomock.Eq(int64(10)), gomock.Eq(1)).Return(int64(111), nil)
			actualOffset, err := f.Seek(10, 1)
			require.NoError(t, err)
			assert.Equal(t, int64(111), actualOffset)
		}},
		{"file Seek error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Seek(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("boom"))
			_, err := f.Seek(10, 1)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"Close non-files", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			s.EXPECT().
				StdoutPipe(gomock.Any()).
				Return(ioutil.NopCloser(strings.NewReader("")), nil)
			p, err := c.StdoutPipe(0)
			require.NoError(t, err)
			require.NoError(t, p.Close())
		}},
		{"file Close happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Close().Return(nil)
			err := f.Close()
			require.NoError(t, err)
		}},
		{"file Close error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Close().Return(errors.New("boom"))
			err := f.Close()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Read happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

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
		{"file Read error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Read(gomock.Any()).Return(0, errors.New("boom"))
			_, err := f.Read(nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"file Read bubbles up io.EOF", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Read(gomock.Any()).Return(4, io.EOF)
			n, err := f.Read(make([]byte, 4))
			require.Equal(t, io.EOF, err)
			assert.Equal(t, 4, n)
		}},
		{"file Write happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

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
		{"file Write error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			mock := workspace.NewMockOsFile(ctrl)
			mock.EXPECT().Close().AnyTimes()
			s.EXPECT().
				Open(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(mock, nil)
			f, werr := c.Open("myFile", 0, 0600)
			require.Nil(t, werr)

			mock.EXPECT().Write(gomock.Any()).Return(0, errors.New("boom"))
			_, err := f.Write(nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"URI happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			s.EXPECT().
				URI(gomock.Any()).
				DoAndReturn(func(name string) (workspace.URI, error) {
					assert.Equal(t, "myFile", name)
					return workspace.ParseURI("test:///myFile")
				})
			fil, err := c.URI("myFile")
			require.NoError(t, err)
			assert.Equal(t, "test:///myFile", fil.String())
		}},
		{"URI error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			s.EXPECT().
				URI(gomock.Any()).
				DoAndReturn(func(name string) (string, error) {
					return "", errors.New("boom")
				})
			_, err := c.URI("myFile")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
		{"ReadLink happy path", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			var called int
			s.EXPECT().
				ReadLink(gomock.Any()).
				DoAndReturn(func(name string) (string, error) {
					called++
					assert.Equal(t, "myFile", name)
					return "myFile.orig", nil
				})
			fil, err := c.ReadLink("myFile")
			require.NoError(t, err)
			assert.Equal(t, 1, called)
			assert.Equal(t, "myFile.orig", fil)
		}},
		{"ReadLink error", func(t *testing.T, ctrl *gomock.Controller, c workspace.Scheme, s *workspacetest.MockScheme) {
			s.EXPECT().
				ReadLink(gomock.Any()).
				DoAndReturn(func(name string) (string, error) {
					return "", errors.New("boom")
				})
			_, err := c.ReadLink("myFile")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom")
		}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.description, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client, mock, cleanup := fn(t, ctrl)
			defer cleanup()

			tcase.do(t, ctrl, client, mock)
		})
	}
}

type testFileInfo struct {
	name    string
	isDir   bool
	modTime time.Time
	size    int64
	mode    os.FileMode
}

func (t testFileInfo) Name() string {
	return t.name
}
func (t testFileInfo) Size() int64 {
	return t.size
}

func (t testFileInfo) Mode() os.FileMode {
	return t.mode
}

func (t testFileInfo) ModTime() time.Time {
	return t.modTime
}

func (t testFileInfo) IsDir() bool {
	return t.isDir
}

func (t testFileInfo) Sys() interface{} {
	return nil
}
