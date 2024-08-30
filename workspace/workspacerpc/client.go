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
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	bluectx "github.com/unstablebuild/blue/context"
	"github.com/unstablebuild/blue/logging"
	"google.golang.org/protobuf/types/known/timestamppb"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/rpc"
)

const defaultTimeout = 5 * time.Second

// for extension-side
var _ workspaceapi.FileSystem = (*Client)(nil)
var _ workspaceapi.Executor = (*Client)(nil)
var _ workspaceapi.Terminal = (*Client)(nil)

// for scheme registry-side
var _ schemeapi.Scheme = (*Client)(nil)

// Client is a workspace and scheme client.
type Client struct {
	cc        rpc.MuxConn
	exec      ExecutorClient
	scheme    SchemeClient
	files     FilesClient
	term      TerminalClient
	ctx       context.Context
	cancelCtx func()
}

// NewClient allocates storage for a new workspace.Client and
// initializes it with cc. Client satisfies workspaceapi.Workspace
// by connecting to a Server via the given rpc connection.
func NewClient(cc rpc.MuxConn) *Client {
	ret := new(Client)
	ret.Init(cc)
	runtime.SetFinalizer(ret, func(c *Client) { c.Close() })
	return ret
}

// Init initializes this client with cc.
func (c *Client) Init(cc rpc.MuxConn) {
	c.cc = cc
	c.scheme = NewSchemeClient(cc)
	c.files = NewFilesClient(cc)
	c.term = NewTerminalClient(cc)
	c.exec = NewExecutorClient(cc)
	c.ctx, c.cancelCtx = context.WithCancel(context.Background())
}

// URI satisfies workspaceapi.Workspace.
func (c *Client) URI(path string) (workspaceapi.URI, error) {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := URIRequest{Path: path}
	resp, err := c.scheme.URI(ctx, &req)
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

// Open satisfies workspace.Workspace.
func (c *Client) Open(path string, flag int, mode os.FileMode) (
	workspaceapi.File, *workspaceapi.Error,
) {
	return c.OpenFile(path, flag, mode)
}

// OpenFile satisfies workspaceapi.Workspace.
func (c *Client) OpenFile(path string, flag int, mode os.FileMode) (
	workspaceapi.File, *workspaceapi.Error,
) {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := makeOpenRequest(path, flag, mode)
	resp, err := c.scheme.Open(ctx, req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, &workspaceapi.Error{Err: err}
	}
	if werr, ok := isTypedError(resp); ok {
		return nil, werr
	}
	ret := newFileClient(c.ctx, c, c.cc,
		resp.GetFilename(), uintptr(resp.GetFd()))
	return ret, nil
}

// Stat returns a FileInfo describing the named file.
func (c *Client) Stat(name string) (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := StatRequest{Filename: name}
	resp, err := c.scheme.Stat(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	if werr, ok := isTypedError(resp); ok {
		return nil, werr.ToError()
	}
	return &fileClientInfo{StatResponse: *resp}, nil // nolint:govet
}

// ReadDir reads the named directory, returning all its directory entries.
func (c *Client) ReadDir(name string) ([]os.DirEntry, error) {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := ReadDirRequest{Root: name}
	resp, err := c.scheme.ReadDir(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	if werr, ok := isTypedError(resp); ok {
		return nil, werr.ToError()
	}
	respp := resp.GetPath()
	ret := make([]os.DirEntry, 0, len(respp))
	for _, entry := range respp {
		ret = append(ret, dirEntry{
			c:        c,
			name:     entry.Name,
			isDir:    entry.IsDir,
			modeType: entry.Mode,
		})
	}

	return ret, nil
}

// Remove satisfies workspaceapi.Workspace.
func (c *Client) Remove(path string) error {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := RemoveRequest{Filename: path}
	resp, err := c.scheme.Remove(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}
	if werr, ok := isTypedError(resp); ok {
		return werr.ToError()
	}
	return nil
}

// Rename satisfies workspaceapi.Workspace.
func (c *Client) Rename(oldpath, newpath string) error {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := RenameRequest{Filename: oldpath, Newfilename: newpath}
	resp, err := c.scheme.Rename(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}
	if werr, ok := isTypedError(resp); ok {
		return werr.ToError()
	}
	return nil
}

// Lstat satisfies workspaceapi.Workspace.
func (c *Client) Lstat(name string) (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := StatRequest{Filename: name, Lstat: true}
	resp, err := c.scheme.Stat(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}
	if werr, ok := isTypedError(resp); ok {
		return nil, werr.ToError()
	}
	return &fileClientInfo{StatResponse: *resp}, nil // nolint:govet
}

// ReadLink satisfies workspaceapi.Workspace.
func (c *Client) ReadLink(filename string) (string, error) {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := ReadLinkRequest{Filename: filename}
	resp, err := c.scheme.ReadLink(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return "", err
	}
	return resp.GetFilename(), nil
}

// Start satisfies workspaceapi.Workspace
func (c *Client) Start(cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	return c.StartCommand(context.Background(), cmd)
}

// StartCommand returns the Pid to execute the named program with the given
// arguments. For more details see exec.Command.
func (c *Client) StartCommand(
	commandCtx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	var cancelFn func()
	commandCtx, cancelFn = context.WithCancel(commandCtx)
	commandCtx = bluectx.First(commandCtx, c.ctx)

	stream, err := c.exec.StartCommand(commandCtx)
	if err != nil {
		cancelFn()
		return 0, fmt.Errorf("new stream: %v", err)
	}
	var setsid, setctty bool
	if cmd.SysProcAttr != nil {
		setsid = cmd.SysProcAttr.Setsid
		setctty = cmd.SysProcAttr.Setctty
	}
	req := CommandPayload{
		Type: CommandPayload_TypeStart,
		Start: &StartCommandRequest{
			Name:    cmd.Path,
			Dir:     cmd.Dir,
			Args:    cmd.Args,
			Env:     cmd.Env,
			Stdin:   cmd.Stdin != nil,
			Stdout:  cmd.Stdout != nil,
			Stderr:  cmd.Stderr != nil,
			Setsid:  setsid,
			Setctty: setctty,
		},
	}
	req.Start.StdinFd, req.Start.StdinName = tryUnwrapFile(cmd.Stdin)
	req.Start.StdoutFd, req.Start.StdoutName = tryUnwrapFile(cmd.Stdout)
	req.Start.StderrFd, req.Start.StderrName = tryUnwrapFile(cmd.Stderr)
	streamer := newClientCommandStreamer(commandCtx, req.Start, cmd, stream)

	type result struct {
		pid workspaceapi.Pid
		err error
	}

	// implement rpc timeout
	handshakeCtx, cancel := context.WithTimeout(commandCtx, defaultTimeout)
	defer cancel()

	ch := make(chan result)
	go func() {
		// do not worry about closing stream here something else
		// should take care of closing the connection if deemed appropiate.
		if err := stream.Send(&req); err != nil {
			res := result{err: fmt.Errorf("send start command request: %v", err)}
			select {
			case ch <- res:
			case <-handshakeCtx.Done():
			}
			return
		}
		pid, err := streamer.waitForPid()
		c.log(log.DebugLevel, "wait for pid: %d err=%v", pid, err)
		if err != nil {
			res := result{err: fmt.Errorf("error waiting for pid: %v", err)}
			select {
			case ch <- res:
			case <-handshakeCtx.Done():
			}
			return
		}
		select {
		case ch <- result{pid: pid}:
		case <-handshakeCtx.Done():
		}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			cancelFn()
			return 0, res.err
		}
		go streamer.streamCommandData(c, cancelFn)
		return res.pid, nil
	case <-handshakeCtx.Done():
		cancelFn()
		return 0, handshakeCtx.Err()
	}
}

// Signal sends a signal to the running process.
func (c *Client) Signal(p workspaceapi.Pid, s syscall.Signal) error {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := SignalRequest{Pid: int64(p), Sig: int32(s)}
	_, err := c.exec.Signal(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}
	return nil
}

// StartPty satisfies workspaceapi.Workspace
func (c *Client) StartPty() (workspaceapi.Pty, error) {
	return c.NewPty(context.Background())
}

// NewPty creates a new pseudoterminal.
func (c *Client) NewPty(ctx context.Context) (workspaceapi.Pty, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	ctx = bluectx.First(ctx, c.ctx)
	defer cancel()

	var req NewPtyRequest
	resp, err := c.term.NewPty(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return workspaceapi.Pty{}, err
	}
	master := newFileClient(c.ctx, c, c.cc,
		resp.GetMaster(), uintptr(resp.GetMasterFd()))
	slave := newFileClient(c.ctx, c, c.cc,
		resp.GetSlave(), uintptr(resp.GetSlaveFd()))
	ret := workspaceapi.Pty{
		Master: master,
		Slave:  slave,
	}

	return ret, nil
}

// SetPtySize sets the width and height in columns and rows of
// a pseudoterminal.
func (c *Client) SetPtySize(p workspaceapi.Pty, width, height int) error {
	ctx, cleanup := ctxWithTimeout(c.ctx)
	defer cleanup()

	req := SetPtySizeRequest{
		Master:   p.Master.Name(),
		MasterFd: uint32(p.Master.Fd()),
		Slave:    p.Slave.Name(),
		SlaveFd:  uint32(p.Slave.Fd()),
		Width:    int32(width),
		Height:   int32(height),
	}
	_, err := c.term.SetPtySize(ctx, &req)
	runtime.KeepAlive(c)
	return err
}

// NewFile satisfies schemeapi.Scheme.
func (c *Client) NewFile(fd uintptr, filename string) workspaceapi.File {
	return newFileClient(c.ctx, c, c.cc, filename, fd)
}

// Close closes all resources associated with this client.
func (c *Client) Close() (ret error) {
	c.cancelCtx()

	if err := c.cc.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}

	runtime.SetFinalizer(c, nil)

	return
}

func (c *Client) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "workspacerpc.Client").
		Logf(level, msg, args...)
}

type fileClientInfo struct {
	StatResponse
}

func (f *fileClientInfo) Name() string {
	return f.StatResponse.GetName()
}

func (f *fileClientInfo) Size() int64 {
	return f.StatResponse.GetSize()
}

func (f *fileClientInfo) Mode() os.FileMode {
	return os.FileMode(f.StatResponse.GetMode())
}

func (f *fileClientInfo) ModTime() time.Time {
	return protoTimeToStd(f.StatResponse.GetModTime())
}

func (f *fileClientInfo) IsDir() bool {
	return f.StatResponse.GetIsDir()
}

func (f *fileClientInfo) Sys() interface{} {
	return nil
}

type dirEntry struct {
	c        *Client
	name     string
	isDir    bool
	modeType int32
}

func (e dirEntry) Name() string {
	return e.name
}

func (e dirEntry) IsDir() bool {
	return e.isDir
}

func (e dirEntry) Type() os.FileMode {
	return os.FileMode(e.modeType)
}

func (e dirEntry) Info() (os.FileInfo, error) {
	return e.c.Stat(e.Name())
}

func protoTimeToStd(ts *timestamppb.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
}

func makeOpenRequest(name string, flag int, perm os.FileMode) *OpenRequest {
	return &OpenRequest{
		Filename: name,
		Mode:     int32(perm),
		O_RDONLY: flag&^(os.O_APPEND|os.O_CREATE|os.O_EXCL|os.O_SYNC|os.O_TRUNC) == os.O_RDONLY,
		O_WRONLY: flag&^(os.O_APPEND|os.O_CREATE|os.O_EXCL|os.O_SYNC|os.O_TRUNC) == os.O_WRONLY,
		O_APPEND: flag&os.O_APPEND != 0,
		O_CREATE: flag&os.O_CREATE != 0,
		O_EXCL:   flag&os.O_EXCL != 0,
		O_SYNC:   flag&os.O_SYNC != 0,
		O_TRUNC:  flag&os.O_TRUNC != 0,
	}
}

type errResponse interface {
	GetIsExistErr() bool
	GetIsNotExistErr() bool
	GetIsPermissionErr() bool
}

func isTypedError(resp errResponse) (*workspaceapi.Error, bool) {
	if resp.GetIsExistErr() || resp.GetIsNotExistErr() || resp.GetIsPermissionErr() {
		return &workspaceapi.Error{
			IsExist:      resp.GetIsExistErr(),
			IsNotExist:   resp.GetIsNotExistErr(),
			IsPermission: resp.GetIsPermissionErr(),
		}, true
	}
	return nil, false
}

func ctxWithTimeout(resourceCtx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	ctx = bluectx.First(ctx, resourceCtx)
	return ctx, cancel
}

func tryUnwrapFile(ifc interface{}) (uint32, string) {
	fc, ok := ifc.(*FileClient)
	if !ok {
		return 0, ""
	}
	return uint32(fc.Fd()), fc.Name()
}
