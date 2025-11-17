// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package workspacerpc

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	grpc "google.golang.org/grpc"
	"unstable.build/go-tui/api/workspaceapi"
)

var _ workspaceapi.File = (*FileClient)(nil)
var _ io.ReadCloser = (*FileClient)(nil)
var _ io.WriteCloser = (*FileClient)(nil)

// FileClient is a client to a remote file.
type FileClient struct {
	root   string
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
	root string, ctx context.Context, c *Client,
	conn grpc.ClientConnInterface, filename string, fd uintptr,
) workspaceapi.File {
	client := NewFilesClient(conn)
	ret := &FileClient{
		root:     root,
		c:        c,
		client:   client,
		filename: filename,
		fd:       fd,
		ctx:      ctx,
	}
	c.log(log.TraceLevel, "new file client: name=%s, fd=%d", filename, fd)
	runtime.SetFinalizer(ret, func(f *FileClient) {
		if f.closed {
			return
		}
		// user should be calling Close, so this is best effort
		// to not leak a fd in the server. If extension is shutting down
		// there's a risk this might not complete before program exits
		go sendCloseRequest(ctx, client, root, fd, filename) //nolint:errcheck
	})
	return ret
}

// Read satisfies io.Reader.
func (c *FileClient) Read(p []byte) (n int, err error) {
	// Read should not ever timeout as it is expected to block
	// if data is not available yet.
	ctx := c.ctx

	req := ReadRequest{
		Root:     c.root,
		N:        int64(len(p)),
		Fd:       uint32(c.fd),
		Filename: c.filename,
	}
	resp, err := c.client.Read(ctx, &req)
	c.log(log.TraceLevel, "file client read: req:%#v, respN:%d, err=%v",
		&req, resp.GetN(), err)
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

// ReadAt satisfies io.ReaderAt.
func (c *FileClient) ReadAt(p []byte, offset int64) (n int, err error) {
	// Read should not ever timeout as it is expected to block
	// if data is not available yet.
	ctx := c.ctx

	req := ReadRequest{
		Root:     c.root,
		Offset:   offset,
		N:        int64(len(p)),
		Fd:       uint32(c.fd),
		Filename: c.filename,
	}
	resp, err := c.client.ReadAt(ctx, &req)
	c.log(log.TraceLevel, "file client read: req:%#v, respN:%d, err=%v",
		&req, resp.GetN(), err)
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

	req := WriteRequest{Root: c.root, Data: p, Fd: uint32(c.fd), Filename: c.filename}
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
	err := sendCloseRequest(c.ctx, c.client, c.root, c.fd, c.filename)
	runtime.KeepAlive(c)
	return err
}

func sendCloseRequest(
	ctx context.Context, client FilesClient,
	root string, fd uintptr, filename string,
) error {
	ctx, cleanup := ctxWithTimeout(ctx)
	defer cleanup()

	req := CloseFileRequest{Root: root, Fd: uint32(fd), Filename: filename}
	_, err := client.Close(ctx, &req)
	if err != nil {
		return err
	}

	log.Tracef("close called for fd %d and name %s", fd, filename)
	return nil
}

func (c *FileClient) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
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

	req := StatRequest{Root: c.root, Filename: c.filename}
	resp, err := c.client.Stat(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	if werr, ok := isTypedError(resp); ok {
		return nil, werr
	}
	return &fileClientInfo{StatResponse: *resp}, nil // nolint:govet
}

// Sync satisfies workspaceapi.File.
func (c *FileClient) Sync() error {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := SyncRequest{Root: c.root, Fd: uint32(c.fd), Filename: c.filename}
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

	req := TruncateRequest{Root: c.root, Fd: uint32(c.fd), Filename: c.filename, Size: size}
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
		Root:     c.root,
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
