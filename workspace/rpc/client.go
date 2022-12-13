package rpc

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"syscall"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	"google.golang.org/grpc"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

const defaultTimeout = 10 * time.Second

var _ workspaceapi.Workspace = (*Client)(nil)

type Client struct {
	cc     grpc.ClientConnInterface
	client WorkspaceClient
	impl   openRemoveClientImpl
}

// NewClient allocates storage for a new workspace.Client and
// initializes it with cc. Client satisfies workspace.API
// by connecting to a Server via the given grpc connection.
func NewClient(cc grpc.ClientConnInterface) *Client {
	ret := new(Client)
	ret.Init(cc)
	runtime.SetFinalizer(ret, func(c *Client) { c.Close() })
	return ret
}

// Init initializes this client with cc.
func (c *Client) Init(cc grpc.ClientConnInterface) {
	client := NewWorkspaceClient(cc)
	c.cc = cc
	c.client = client
	c.impl.executorClientImpl.init(c.client)
	c.impl.client = client
}

// Open satisfies workspace.API.
func (c *Client) Open(path string, flag int, mode os.FileMode) (workspaceapi.File, *workspaceapi.Error) {
	f, err := c.impl.Open(path, flag, mode)
	runtime.KeepAlive(c)
	return f, err
}

// Remove satisfies workspace.API.
func (c *Client) Remove(path string) error {
	err := c.impl.Remove(path)
	runtime.KeepAlive(c)
	return err
}

// ReadDir reads the named directory, returning all its directory entries.
func (c *Client) ReadDir(name string) ([]os.DirEntry, error) {
	ret, err := c.impl.ReadDir(name)
	runtime.KeepAlive(c)
	return ret, err
}

// Stat returns a FileInfo describing the named file.
func (c *Client) Stat(name string) (os.FileInfo, error) {
	ret, err := c.impl.Stat(name)
	runtime.KeepAlive(c)
	return ret, err
}

// URI satisfies workspace.API.
func (c *Client) URI(path string) (workspaceapi.URI, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := URIRequest{Path: path}
	resp, err := c.client.URI(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return workspaceapi.URI{}, err
	}
	uri, err := workspaceapi.ParseURI(resp.GetUri())
	if err != nil {
		return workspaceapi.URI{}, fmt.Errorf("Could not parse URI response from server: %w", err)
	}
	return uri, nil
}

// NewPty creates a new pseudoterminal.
func (c *Client) NewPty() (workspaceapi.Pty, error) {
	ret, err := c.impl.NewPty()
	runtime.KeepAlive(c)
	return ret, err
}

// SetPtySize sets the width and height in columns and rows of
// a pseudoterminal.
func (c *Client) SetPtySize(p workspaceapi.Pty, width, height int) error {
	err := c.impl.SetPtySize(p, width, height)
	runtime.KeepAlive(c)
	return err
}

// Command returns the Pid to execute the named program with the given
// arguments. For more details see exec.Command.
func (c *Client) Command(name string, arg ...string) (workspaceapi.Pid, error) {
	ret, err := c.impl.Command(name, arg...)
	runtime.KeepAlive(c)
	return ret, err
}

// Start starts the specified command but does not wait for it to complete.
// The Wait method will return an error if there's any while running command
// and release associated resources.
func (c *Client) Start(p workspaceapi.Pid) error {
	err := c.impl.Start(p)
	runtime.KeepAlive(c)
	return err
}

// Signal sends a signal to the running process.
func (c *Client) Signal(p workspaceapi.Pid, s syscall.Signal) error {
	err := c.impl.Signal(p, s)
	runtime.KeepAlive(c)
	return err
}

// StderrPipe returns a pipe that will be connected to the command's standard
// error when the command starts. See exec.Cmd.StderrPipe for more details.
func (c *Client) StderrPipe(p workspaceapi.Pid) (io.ReadCloser, error) {
	ret, err := c.impl.StderrPipe(p)
	runtime.KeepAlive(c)
	return ret, err
}

// StdinPipe returns a pipe that will be connected to the command's standard
// input when the command starts. See exec.Cmd.StdinPipe for more details.
func (c *Client) StdinPipe(p workspaceapi.Pid) (io.WriteCloser, error) {
	ret, err := c.impl.StdinPipe(p)
	runtime.KeepAlive(c)
	return ret, err
}

// StdoutPipe returns a pipe that will be connected to the command's standard
// output when the command starts. See exec.Cmd.StdoutPipe for more details.
func (c *Client) StdoutPipe(p workspaceapi.Pid) (io.ReadCloser, error) {
	ret, err := c.impl.StdoutPipe(p)
	runtime.KeepAlive(c)
	return ret, err
}

// Wait waits for the command to exit and waits for any copying to stdin or
// opying from stdout or stderr to complete.
// The command must have been started by Start.
// The returned error is nil if the command runs, has no problems copying
// stdin, stdout, and stderr, and exits with a zero exit status.
func (c *Client) Wait(p workspaceapi.Pid) error {
	err := c.impl.Wait(p)
	runtime.KeepAlive(c)
	return err
}

// Close closes all resources associated with this client.
func (c *Client) Close() (ret error) {
	if closer, ok := c.cc.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	if err := c.impl.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}

	runtime.SetFinalizer(c, nil)

	return
}

func ctxWithTimeout() (context.Context, func()) {
	return context.WithTimeout(context.Background(), defaultTimeout)
}
