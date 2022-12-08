package rpc

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"syscall"

	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
)

// NewScheme returns a workspace.Scheme RPC-based client over
// the given connection. It expects a SchemeServer to be listening
// on the other side of the connection.
func NewScheme(cc proto.MuxConn) workspace.Scheme {
	ret := new(schemeClientImpl)
	ret.init(cc)
	runtime.SetFinalizer(ret, func(c *schemeClientImpl) { c.Close() })
	return ret
}

type schemeClientImpl struct {
	client SchemeClient
	impl   openRemoveClientImpl
	cc     proto.MuxConn
}

func (c *schemeClientImpl) init(cc proto.MuxConn) {
	client := NewSchemeClient(cc)
	c.client = client
	c.impl.executorClientImpl.init(c.client)
	c.impl.client = client
	c.cc = cc
}

func (c *schemeClientImpl) Open(name string, flag int, perm os.FileMode) (
	workspace.File, *workspace.Error,
) {
	ret, err := c.impl.Open(name, flag, perm)
	runtime.KeepAlive(c)
	return ret, err
}

func (c *schemeClientImpl) Remove(name string) error {
	err := c.impl.Remove(name)
	runtime.KeepAlive(c)
	return err
}

func (c *schemeClientImpl) ReadDir(name string) ([]os.DirEntry, error) {
	ret, err := c.impl.ReadDir(name)
	runtime.KeepAlive(c)
	return ret, err
}

func (c *schemeClientImpl) Stat(name string) (os.FileInfo, error) {
	ret, err := c.impl.Stat(name)
	runtime.KeepAlive(c)
	return ret, err
}

func (c *schemeClientImpl) URI(path string) (workspace.URI, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := URIRequest{Path: path}
	resp, err := c.client.URI(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return workspace.URI{}, err
	}
	uri, err := workspace.ParseURI(resp.GetUri())
	if err != nil {
		return workspace.URI{}, fmt.Errorf("Could not parse URI response from server: %w", err)
	}
	return uri, nil
}

func (c *schemeClientImpl) Rename(oldpath, newpath string) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := RenameRequest{Filename: oldpath, Newfilename: newpath}
	resp, err := c.client.Rename(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}
	if werr, ok := isTypedError(resp); ok {
		return werr.ToError()
	}
	return nil
}

func (c *schemeClientImpl) Lstat(name string) (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StatRequest{Filename: name, Lstat: true}
	resp, err := c.client.Stat(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	if werr, ok := isTypedError(resp); ok {
		return nil, werr.ToError()
	}
	return fileClientInfo{StatResponse: *resp}, nil
}

func (c *schemeClientImpl) ReadLink(filename string) (string, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := ReadLinkRequest{Filename: filename}
	resp, err := c.client.ReadLink(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return "", err
	}
	return resp.GetFilename(), nil
}

func (c *schemeClientImpl) NewPty() (workspace.Pty, error) {
	ret, err := c.impl.NewPty()
	runtime.KeepAlive(c)
	return ret, err
}

func (c *schemeClientImpl) SetPtySize(p workspace.Pty, width, height int) error {
	err := c.impl.SetPtySize(p, width, height)
	runtime.KeepAlive(c)
	return err
}

func (c *schemeClientImpl) Command(name string, arg ...string) (workspace.Pid, error) {
	ret, err := c.impl.Command(name, arg...)
	runtime.KeepAlive(c)
	return ret, err
}

func (c *schemeClientImpl) Start(p workspace.Pid) error {
	err := c.impl.Start(p)
	runtime.KeepAlive(c)
	return err
}

func (c *schemeClientImpl) Signal(p workspace.Pid, s syscall.Signal) error {
	err := c.impl.Signal(p, s)
	runtime.KeepAlive(c)
	return err
}

func (c *schemeClientImpl) StderrPipe(p workspace.Pid) (io.ReadCloser, error) {
	ret, err := c.impl.StderrPipe(p)
	runtime.KeepAlive(c)
	return ret, err
}

func (c *schemeClientImpl) StdinPipe(p workspace.Pid) (io.WriteCloser, error) {
	ret, err := c.impl.StdinPipe(p)
	runtime.KeepAlive(c)
	return ret, err
}

func (c *schemeClientImpl) StdoutPipe(p workspace.Pid) (io.ReadCloser, error) {
	ret, err := c.impl.StdoutPipe(p)
	runtime.KeepAlive(c)
	return ret, err
}

func (c *schemeClientImpl) Wait(p workspace.Pid) error {
	ret := c.impl.Wait(p)
	runtime.KeepAlive(c)
	return ret
}

func (c *schemeClientImpl) Close() (ret error) {
	if err := c.impl.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if closer, ok := c.cc.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	runtime.SetFinalizer(c, nil)
	return
}
