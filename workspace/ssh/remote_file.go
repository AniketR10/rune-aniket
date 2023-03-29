package ssh

import (
	"errors"
	"os"
	"runtime"

	workspaceapi "unstable.build/go-tui/api/workspace"
)

var (
	_ workspaceapi.File = (*remoteFile)(nil)

	errInvalidFd = errors.New("invalid file descriptor")
)

type remoteFile struct {
	fd       uintptr
	filename string
	scheme   *remoteScheme
}

func newRemoteFile(s *remoteScheme, fd uintptr, filename string) *remoteFile {
	ret := &remoteFile{scheme: s, fd: fd, filename: filename}
	runtime.SetFinalizer(ret, func(f *remoteFile) {
		f.Close()
	})
	return ret
}

func (c *remoteFile) newFile() (workspaceapi.File, error) {
	err, scheme := c.scheme.state()
	if err != nil {
		return nil, err
	}
	f := scheme.NewFile(c.fd, c.filename)
	if f == nil {
		return nil, errInvalidFd
	}
	// the returned file is transient so GC should
	// not close the remote file.
	runtime.SetFinalizer(f, nil)
	return f, nil
}

func (c *remoteFile) Read(p []byte) (n int, err error) {
	f, err := c.newFile()
	if err != nil {
		return 0, err
	}
	return f.Read(p)
}

func (c *remoteFile) Write(p []byte) (n int, err error) {
	f, err := c.newFile()
	if err != nil {
		return 0, err
	}
	return f.Write(p)
}

func (c *remoteFile) Name() string {
	return c.filename
}

func (c *remoteFile) Stat() (os.FileInfo, error) {
	f, err := c.newFile()
	if err != nil {
		return nil, err
	}
	return f.Stat()
}

func (c *remoteFile) Sync() error {
	f, err := c.newFile()
	if err != nil {
		return err
	}
	return f.Sync()
}

func (c *remoteFile) Truncate(size int64) error {
	f, err := c.newFile()
	if err != nil {
		return err
	}
	return f.Truncate(size)
}

func (c *remoteFile) Fd() uintptr {
	return c.fd
}

func (c *remoteFile) Seek(offset int64, whence int) (int64, error) {
	f, err := c.newFile()
	if err != nil {
		return 0, err
	}
	return f.Seek(offset, whence)
}

func (c *remoteFile) Close() error {
	f, err := c.newFile()
	if err != nil {
		return err
	}
	runtime.SetFinalizer(c, nil)
	c.scheme.files.Delete(c.Fd())
	return f.Close()
}
