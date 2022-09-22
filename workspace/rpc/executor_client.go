package rpc

import (
	"context"
	"errors"
	"io"
	"syscall"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/ernestrc/go-tui/workspace"
)

type executorClientImpl struct {
	client ExecutorClient
	files  map[int32]io.Closer
}

type ioClient struct {
	handlerID int32
	client    ExecutorClient
}

func (c *executorClientImpl) init(client ExecutorClient) {
	c.client = client
	c.files = make(map[int32]io.Closer)
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
	return c.newIOClient("/dev/stderr", int32(resp.GetHandlerId())), nil
}

func (c *executorClientImpl) StdinPipe(pid workspace.Pid) (io.WriteCloser, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StdioPipeRequest{Pid: int32(pid)}
	resp, err := c.client.StdinPipe(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.newIOClient("/dev/stdin", int32(resp.GetHandlerId())), nil
}

func (c *executorClientImpl) StdoutPipe(pid workspace.Pid) (io.ReadCloser, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StdioPipeRequest{Pid: int32(pid)}
	resp, err := c.client.StdoutPipe(ctx, &req)
	if err != nil {
		return nil, err
	}
	return c.newIOClient("/dev/stdout", int32(resp.GetHandlerId())), nil
}

func (c *executorClientImpl) Wait(pid workspace.Pid) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := WaitRequest{Pid: int32(pid)}
	_, err := c.client.Wait(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *executorClientImpl) Close() (ret error) {
	for _, res := range c.files {
		if err := res.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	c.files = nil
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

	req := WriteRequest{HandlerId: c.handlerID, Data: string(p)}
	resp, err := c.client.Write(ctx, &req)
	if err != nil {
		return 0, err
	}
	return int(resp.GetN()), nil
}

func (c *ioClient) Close() error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := CloseFileRequest{HandlerId: c.handlerID}
	_, err := c.client.Close(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *executorClientImpl) addCloser(handlerID int32, closer io.Closer) {
	// NOTE this should never happen, but server is boss here
	// so make sure we do not leak resources
	f, ok := c.files[handlerID]
	if ok {
		_ = f.Close()
	}
	c.files[handlerID] = closer
}

func (c *executorClientImpl) newIOClient(
	filename string, handlerID int32,
) *ioClient {
	ret := &ioClient{
		handlerID: handlerID,
		client:    c.client,
	}
	c.addCloser(handlerID, ret)
	return ret
}
