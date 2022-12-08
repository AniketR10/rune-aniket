package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"syscall"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"unstable.build/go-tui/workspace"
)

type executorClientImpl struct {
	// all Scheme implementations must be goroutine-safe
	mu        sync.Mutex
	client    ExecutorClient
	resources map[int32]executorResource
	ptys      map[workspace.Pid]int32
}

type ioClient struct {
	s         *executorClientImpl
	closed    bool
	handlerID int32
	client    interface {
		Close(ctx context.Context, in *CloseFileRequest, opts ...grpc.CallOption) (*CloseFileResponse, error)
		Read(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (*ReadResponse, error)
		Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*WriteResponse, error)
	}
}

type nopLocker struct{}

func (l nopLocker) Lock() {
}
func (l nopLocker) Unlock() {
}

func (c *executorClientImpl) init(client ExecutorClient) {
	c.client = client
	c.resources = make(map[int32]executorResource)
	c.ptys = make(map[workspace.Pid]int32)
}

func (c *executorClientImpl) Command(name string, arg ...string) (workspace.Pid, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := CommandRequest{Name: name, Args: arg}
	resp, err := c.client.Command(ctx, &req)
	if err != nil {
		return 0, err
	}
	return workspace.Pid(resp.GetPid()), nil
}

func (c *executorClientImpl) Start(pid workspace.Pid) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StartRequest{Pid: int32(pid)}
	_, err := c.client.Start(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *executorClientImpl) Signal(pid workspace.Pid, sig syscall.Signal) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := SignalRequest{Pid: int32(pid), Sig: int32(sig)}
	_, err := c.client.Signal(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *executorClientImpl) StderrPipe(pid workspace.Pid) (io.ReadCloser, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StdioPipeRequest{Pid: int32(pid)}
	resp, err := c.client.StderrPipe(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.newIOClient(pid, "/dev/stderr", int32(resp.GetHandlerId())), nil
}

func (c *executorClientImpl) StdinPipe(pid workspace.Pid) (io.WriteCloser, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StdioPipeRequest{Pid: int32(pid)}
	resp, err := c.client.StdinPipe(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.newIOClient(pid, "/dev/stdin", int32(resp.GetHandlerId())), nil
}

func (c *executorClientImpl) StdoutPipe(pid workspace.Pid) (io.ReadCloser, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StdioPipeRequest{Pid: int32(pid)}
	resp, err := c.client.StdoutPipe(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.newIOClient(pid, "/dev/stdout", int32(resp.GetHandlerId())), nil
}

func (c *executorClientImpl) log(level log.Level, msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "executorClientImpl").
		Logf(level, msg, args...)
}

func (c *executorClientImpl) removePidResources(pid workspace.Pid) {
	res := removePidResources(&c.mu, c.resources, pid)
	c.mu.Lock()
	delete(c.ptys, pid)
	c.mu.Unlock()
	c.log(log.TraceLevel, "cleaned all resources of pid %d: %#v", pid, res)
}

func (c *executorClientImpl) Wait(pid workspace.Pid) error {
	// do not set timeout for Wait, as there's no guarantee it should ever return,
	// for instance when plugins run servers that last the entire tui session.
	ctx := context.Background()

	c.log(log.TraceLevel, "Wait(pid=%d)", pid)

	req := WaitRequest{Pid: int32(pid)}
	_, err := c.client.Wait(ctx, &req)
	if err != nil {
		return err
	}
	// Wait waits for the command and copying to stdin or from stdout/err
	// so it's safe to cleanup all resources of pid here
	c.removePidResources(pid)
	return nil
}

func (c *executorClientImpl) NewPty() (workspace.Pty, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := NewPtyRequest{}
	resp, err := c.client.NewPty(ctx, &req)
	if err != nil {
		return workspace.Pty{}, err
	}
	// allow cleanup of master File upon return of Wait
	pid := workspace.Pid(resp.GetPid())
	file := c.newFileClient(pid, "/dev/ptmx", resp.GetMaster())
	ret := workspace.Pty{
		Pid:    pid,
		Master: file,
		Slave:  resp.GetSlave(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.ptys[ret.Pid] = resp.GetMaster()
	return ret, nil
}

func (c *executorClientImpl) SetPtySize(p workspace.Pty, width, height int) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	c.mu.Lock()
	master, ok := c.ptys[p.Pid]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("Extraneous Pty Pid %d", p.Pid)
	}

	req := SetPtySizeRequest{
		Pid:    int32(p.Pid),
		Master: master,
		Slave:  p.Slave,
		Width:  int32(width),
		Height: int32(height),
	}
	_, err := c.client.SetPtySize(ctx, &req)
	return err
}

func (c *executorClientImpl) Close() (ret error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, res := range c.resources {
		if err := res.closer.stop(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	c.resources = nil
	return
}

func (c *ioClient) Read(p []byte) (n int, err error) {
	// Read should not ever timeout as it is expected to block
	// if data is not available yet.
	ctx := context.Background()

	req := ReadRequest{HandlerId: c.handlerID, N: int64(len(p))}
	resp, err := c.client.Read(ctx, &req)
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

func (c *ioClient) Write(p []byte) (n int, err error) {
	// Write should not ever timeout as it is expected to block
	// if data is not available yet.
	ctx := context.Background()

	req := WriteRequest{HandlerId: c.handlerID, Data: p}
	resp, err := c.client.Write(ctx, &req)
	if err != nil {
		return 0, err
	}
	return int(resp.GetN()), nil
}

func (s *ioClient) removeHandle(handlerID int32) {
	delete(s.s.resources, handlerID)
	s.s = nil
}

func (c *ioClient) Close() error {
	if c.closed {
		return nil
	}

	c.closed = true
	c.removeHandle(c.handlerID)

	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := CloseFileRequest{HandlerId: c.handlerID}
	_, err := c.client.Close(ctx, &req)
	if err != nil {
		return err
	}

	log.
		WithField(logging.KeyClass, "ioClient").
		Debugf("close called for handler ID %d", c.handlerID)

	return nil
}

func (c *ioClient) stop() error {
	return c.Close()
}

type executorResource struct {
	pid    workspace.Pid
	closer executorCloser
}

func (c *executorClientImpl) addCloser(
	pid workspace.Pid, handlerID int32, closer executorCloser,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// NOTE this should never happen, but server is boss here
	// so make sure we do not leak resources
	f, ok := c.resources[handlerID]
	if ok {
		c.log(log.WarnLevel, "overriding handler ID %d for pid %d",
			handlerID, pid)
		_ = f.closer.Close()
	}
	c.log(log.TraceLevel, "add closer %d for pid %d", handlerID, pid)
	c.resources[handlerID] = executorResource{pid: pid, closer: closer}
}

func (c *executorClientImpl) newIOClient(
	pid workspace.Pid, filename string, handlerID int32,
) *ioClient {
	ret := &ioClient{
		s:         c,
		handlerID: handlerID,
		client:    c.client,
	}
	c.log(log.TraceLevel, "new I/O client with name %s and handlerID %d for pid %d",
		filename, handlerID, pid)
	c.addCloser(pid, handlerID, ret)
	return ret
}

func (c *executorClientImpl) newFileClient(
	pid workspace.Pid, filename string, handlerID int32,
) *fileClient {
	ret := &fileClient{
		handlerID: handlerID,
		client:    c.client,
		filename:  filename,
		// satisfies Read/Write/Close
		ioClient: ioClient{
			s:         c,
			client:    c.client,
			handlerID: handlerID,
		},
	}
	// TODO use runtime.SetFinalizer
	c.addCloser(pid, handlerID, ret)
	return ret
}

func (c *fileClient) Name() string {
	return c.filename
}

func (c *fileClient) Stat() (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StatRequest{Filename: c.filename}
	resp, err := c.client.Stat(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	return fileClientInfo{StatResponse: *resp}, nil
}

func (c *fileClient) Sync() error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := SyncRequest{HandlerId: c.handlerID}
	_, err := c.client.Sync(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}
	return nil
}

func (c *fileClient) Truncate(size int64) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := TruncateRequest{HandlerId: c.handlerID, Size: size}
	_, err := c.client.Truncate(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}
	return nil
}

func (c *fileClient) Seek(offset int64, whence int) (int64, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := SeekRequest{
		HandlerId: c.handlerID,
		Offset:    offset,
		Whence:    int64(whence),
	}
	resp, err := c.client.Seek(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return 0, err
	}
	return resp.GetNewOffset(), nil
}
