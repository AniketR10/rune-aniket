package rpc

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/proto"
)

var _ workspaceapi.File = (*FileClient)(nil)
var _ io.ReadCloser = (*FileClient)(nil)
var _ io.WriteCloser = (*FileClient)(nil)

// FileClient is a client to a remote file.
type FileClient struct {
	closed bool
	// used to ensure that as long as there's a FileClient
	// Client's finalizer doesnot run.
	c      *Client
	client FilesClient
	ctx    context.Context

	fd       uintptr
	filename string
}

func newFileClient(
	ctx context.Context, c *Client,
	conn proto.MuxConn, filename string, fd uintptr,
) workspaceapi.File {
	ret := &FileClient{
		c:        c,
		client:   NewFilesClient(conn),
		filename: filename,
		fd:       fd,
		ctx:      ctx,
	}
	c.log(log.TraceLevel, "new file client: name=%s, fd=%d", filename, fd)
	runtime.SetFinalizer(ret, func(f *FileClient) {
		f.Close()
	})
	return ret
}

// Read satisfies io.Reader.
func (c *FileClient) Read(p []byte) (n int, err error) {
	// Read should not ever timeout as it is expected to block
	// if data is not available yet.
	ctx := c.ctx

	req := ReadRequest{N: int64(len(p)), Fd: uint32(c.fd), Filename: c.filename}
	resp, err := c.client.Read(ctx, &req)
	c.log(log.TraceLevel, "file client read: req:%#v, respN:%d, err=%v",
		req, resp.N, err)
	runtime.KeepAlive(c)
	if err != nil {
		return 0, err
	}
	data := resp.GetData()
	if len(data) > len(p) || int64(len(data)) != resp.GetN() {
		return 0, errors.New("server returned invalid data")
	}
	copy(p, []byte(data))
	if resp.IsEof {
		err = io.EOF
	} else {
		err = nil
	}
	return int(resp.GetN()), err
}

// Write satisfies io.Writer.
func (c *FileClient) Write(p []byte) (n int, err error) {
	// Write should not ever timeout as it is expected to block
	// until deadline is met or we are able to write.
	ctx := c.ctx

	req := WriteRequest{Data: p, Fd: uint32(c.fd), Filename: c.filename}
	resp, err := c.client.Write(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return 0, err
	}
	return int(resp.GetN()), nil
}

// Close satisfies io.Closer.
func (c *FileClient) Close() error {
	if c.closed {
		return nil
	}

	c.closed = true

	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := CloseFileRequest{Fd: uint32(c.fd), Filename: c.filename}
	_, err := c.client.Close(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}

	c.log(log.TraceLevel, "close called for fd %d and name %s", c.fd, c.filename)
	return nil
}

func (c *FileClient) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "workspace.FileClient").
		Logf(level, msg, args...)
}

// Name satisfies workspaceapi.File.
func (c *FileClient) Name() string {
	return c.filename
}

// Stat satisfies workspaceapi.File.
func (c *FileClient) Stat() (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := StatRequest{Filename: c.filename}
	resp, err := c.client.Stat(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	return fileClientInfo{StatResponse: *resp}, nil
}

// Sync satisfies workspaceapi.File.
func (c *FileClient) Sync() error {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := SyncRequest{Fd: uint32(c.fd), Filename: c.filename}
	_, err := c.client.Sync(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}
	return nil
}

// Truncate satisfies workspaceapi.File.
func (c *FileClient) Truncate(size int64) error {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := TruncateRequest{Fd: uint32(c.fd), Filename: c.filename, Size: size}
	_, err := c.client.Truncate(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}
	return nil
}

// Seek satisfies workspaceapi.File.
func (c *FileClient) Seek(offset int64, whence int) (int64, error) {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := SeekRequest{
		Fd:       uint32(c.fd),
		Filename: c.filename,
		Offset:   offset,
		Whence:   int64(whence),
	}
	resp, err := c.client.Seek(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return 0, err
	}
	return resp.GetNewOffset(), nil
}

// Fd satisfies workspaceapi.File.
func (c *FileClient) Fd() uintptr {
	return c.fd
}
