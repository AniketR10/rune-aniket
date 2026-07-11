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

package workspacessh

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi/workspacerpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"unstable.build/go-tui/debug"
)

const (
	// Scheme represents the URL scheme that this package implements
	Scheme = "ssh"

	// remotePathEnv is prepended to every remote exec string so the
	// supported install location (install.sh symlinks the binary to
	// ~/.local/bin/rune) is found even when the remote's PATH lacks
	// ~/.local/bin — the norm for sshd exec sessions, which run
	// non-login shells that skip the profile files where
	// distributions add ~/.local/bin.
	remotePathEnv = `PATH="$HOME/.local/bin:$PATH"`

	debugPathError = "You can run %q to troubleshoot this. Also, double check your workspace.ssh.command " +
		"and workspace.ssh.shell configuration, if you have any."
)

// ErrSSHConnectionClosed is reported by the gRPC dialer's close
// hook when the underlying SSH transport drops mid-session.
// Exposed so callers can match it via errors.Is.
var ErrSSHConnectionClosed = errors.New("ssh connection closed unexpectedly")

// Option customizes the ssh scheme constructed by New.
type Option func(*scheme)

// WithProvisionManifest supplies a callback that returns an encoded
// package-provisioning manifest (see cmd/rune provisionManifest). When it
// returns a non-empty string, the remote `rune -x` server is asked to mirror
// the local toolchain via a `--install <manifest>` flag. The callback is
// invoked once per connection so the manifest reflects the current local
// install state.
func WithProvisionManifest(fn func() string) Option {
	return func(s *scheme) {
		s.provisionFn = fn
	}
}

// New returns a schemeapi.SchemeFunc capable of managing files over an ssh
// connection. ui drives the interactive auth flow (passphrase / password /
// kbd-interactive prompts). It is intended to be installed into a workspace
// manager:
//
//	mgr.RegisterScheme(workspacessh.Scheme, workspacessh.New(ui))
//
// New panics if ui is nil: a real UI is mandatory because the ssh dial may
// trigger interactive prompts that have no useful default.
func New(ui UI, opts ...Option) schemeapi.SchemeFunc {
	if ui == nil {
		panic("workspacessh.New: ui is required")
	}
	return func(ctx context.Context, cfg config.Config, uri workspaceapi.URI) (schemeapi.Scheme, error) {
		return newScheme(ctx, cfg, uri, ui, opts...)
	}
}

type remote interface {
	NewSession() (schemeapi.Executor, error)
	Close() error
}

type scheme struct {
	cfg      sshConfig
	hostPort string
	user     string
	homedir  string
	basePath string

	getUser         func() (*user.User, error)
	remoteFn        func(context.Context, sshConfig, workspaceapi.URI) (remote, error)
	connectSchemeFn connectSchemeFn
	ctx             context.Context
	cancelCtx       func()
	ui              UI
	provisionFn     func() string

	schemeapi.Scheme
}

func newScheme(
	ctx context.Context, ccfg config.Config, uri workspaceapi.URI, ui UI,
	opts ...Option,
) (*scheme, error) {
	if ui == nil {
		panic("workspacessh.newScheme: ui is required")
	}
	ret := new(scheme)
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	for _, opt := range opts {
		opt(ret)
	}

	cc, err := fromConfig(ccfg)
	if err != nil {
		return nil, err
	}

	ret.getUser = user.Current
	ret.ui = ui
	if cc.command == "" {
		ret.remoteFn = func(c context.Context, sc sshConfig, u workspaceapi.URI) (remote, error) {
			return newStdRemote(c, sc, u, ret.ui)
		}
	} else {
		ret.remoteFn = newProcRemote
	}

	ret.connectSchemeFn = ret.connectScheme
	err = ret.init(ctx, cc, uri)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func parseWorkspaceURI(u workspaceapi.URI, getUser func() (*user.User, error)) (
	username, homedir, hostPort, basePath string, err error,
) {
	username = u.User()

	// respect empty username for URI creation
	// but we need its implicit value for ~ expansion to work
	usernameForHomeDir := username
	if usernameForHomeDir == "" {
		var u *user.User
		u, err = getUser()
		if err != nil {
			return
		}
		usernameForHomeDir = u.Username
	}
	// I doubt we'll ever ssh into a non-linux host. root's home is /root,
	// not /home/root, on every standard Linux distribution.
	if usernameForHomeDir == "root" {
		homedir = "/root"
	} else {
		homedir = filepath.Join("/", "home", usernameForHomeDir)
	}

	basePath, err = workspaceapi.ExpandPath(u.Path(), func() (*user.User, error) {
		return &user.User{Username: username, HomeDir: homedir}, nil
	}, func() (string, error) {
		// return host's base path, but this should never happen
		return "/", nil
	})
	if err != nil {
		return
	}

	hostPort = u.Host()
	if hostPort == "" {
		err = errors.New("ssh scheme with empty host is invalid")
		return
	}

	return
}

func (s *scheme) runAndWait(
	ctx context.Context,
	remote remote, cmdStr string, args ...string,
) (string, bool, error) {
	ses, err := remote.NewSession()
	if err != nil {
		return "", false, fmt.Errorf("new session: %v", err)
	}

	if s.cfg.shell != "" {
		args = append([]string{"-c", cmdStr}, args...)
		cmdStr = s.cfg.shell
	}
	// The env assignment is applied to the outermost command word so
	// it survives the optional shell wrap above: the remote shell
	// that parses the exec string expands $HOME and the child
	// process inherits the amended PATH.
	cmdStr = remotePathEnv + " " + cmdStr
	var stderr, stdout bytes.Buffer
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    cmdStr,
		Args:    args,
		Stderr:  &stderr,
		Stdout:  &stdout,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}
	_, err = ses.StartCommand(s.ctx, cmd)
	if err != nil {
		return "", false, fmt.Errorf("start command: %v", err)
	}

	err = <-ch
	if err != nil {
		if stdout.Len() != 0 {
			err = fmt.Errorf("stdout: %v: %s", err, stdout.String())
		}
		if stderr.Len() != 0 {
			err = fmt.Errorf("stderr: %v: %s", err, stderr.String())
		}
		if procRemote, ok := remote.(*procRemote); ok {
			cmd, args := procRemote.CommandString(cmdStr, args...)
			return fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")), false, err
		}
		return "", false, err
	}

	err = ses.Close()
	// stdlib ssh session returns io.EOF if closing after command returned
	if err != nil && err != io.EOF {
		err = fmt.Errorf("close session: %v", err)
		return "", false, err
	}
	return "", true, nil
}

func (s *scheme) whichCommand(ctx context.Context, remote remote, cmd string) error {
	cmdAndArgs, avail, err := s.runAndWait(ctx, remote, "which", cmd)
	if err != nil {
		err = fmt.Errorf("could not check if %s executable is in PATH: %w", cmd, err)
	}
	if !avail {
		errStr := "%q executable was not found on remote. " +
			"Make sure it's installed at ~/.local/bin/rune or available " +
			"via $PATH to a non-interactive shell. "
		if err != nil {
			errStr = fmt.Sprintf("%s %v. ", errStr, err)
		}
		if cmdAndArgs != "" {
			return fmt.Errorf(errStr+debugPathError, cmd, cmdAndArgs)
		}
		return fmt.Errorf(errStr, cmd)
	}
	return nil
}

func (s *scheme) workspaceExists(
	ctx context.Context, remote remote, uri workspaceapi.URI,
) error {
	cmdAndArgs, ok, err := s.runAndWait(ctx, remote, "ls", uri.Path())
	if err != nil {
		return fmt.Errorf("could not check if workspace path %q exists: %w", uri.Path(), err)
	}
	if !ok {
		return fmt.Errorf("path %q was not found on remote "+ //nolint:staticcheck
			debugPathError, uri.Path(), cmdAndArgs)
	}
	return nil
}

func (s *scheme) connectScheme(
	ctx context.Context, uri workspaceapi.URI, closeHook func(error),
) (schemeapi.Scheme, error) {
	// remoteWorkspaceServerBin is the binary name we expect to find on
	// the remote host. It is the `rune` binary started in workspace
	// server mode (see cmd/rune/main.go: --workspace-server / -x).
	const remoteWorkspaceServerBin = "rune"

	sshPath := s.basePath
	if sshPath == "" {
		sshPath = "."
	}

	remote, err := s.remoteFn(ctx, s.cfg, uri)
	if err != nil {
		return nil, fmt.Errorf("could not initialize remote: %w", err)
	}

	// NOTE: the next checks are to avoid error messages getting lost when
	// trying to connect so we can provide better error messages
	//
	// Skipped entirely when the user opts in via
	// `workspace.ssh.skip_preflight = True` — typically to play nicely
	// with servers that have a tight MaxSessions budget, since each
	// pre-flight probe opens its own session channel.
	if !s.cfg.skipPreflight {
		err = s.whichCommand(ctx, remote, remoteWorkspaceServerBin)
		if err != nil {
			return nil, err
		}

		err = s.workspaceExists(ctx, remote, uri)
		if err != nil {
			return nil, err
		}
	}

	ses, err := remote.NewSession()
	if err != nil {
		return nil, fmt.Errorf("NewSession: %v", err)
	}

	var extraArgs []string
	if log.IsLevelEnabled(log.TraceLevel) {
		extraArgs = []string{"-p", "-o", "rune-workspace-server.log"}
	}

	var installArgs []string
	if s.cfg.provisionPackages && s.provisionFn != nil {
		if manifest := s.provisionFn(); manifest != "" {
			// manifest is a single validated shell-safe token
			// (cmd/rune enforces the [A-Za-z0-9._@,%+~/-] class), so it
			// needs no quoting even though the remote shell reparses the
			// whole command.
			installArgs = []string{"--install", manifest}
		}
	}

	cmdStr := remoteWorkspaceServerBin
	args := append([]string{"-x", sshPath}, installArgs...)
	args = append(args, extraArgs...)
	if s.cfg.shell != "" {
		args = append([]string{"-c", cmdStr}, args...)
		cmdStr = s.cfg.shell
	}
	cmdStr = remotePathEnv + " " + cmdStr
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    cmdStr,
		Args:    args,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}

	stdoutRead, stderrRead, stdinWrite, closers, err := s.setPipes(&cmd)
	if err != nil {
		return nil, fmt.Errorf("could not create pipes: %s", err)
	}

	// context of command should mirror the lifcycle of this scheme
	// not the ctx passed to this constructor, which could have
	// a connection timeout (i.e. retry ctx)
	_, err = ses.StartCommand(s.ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("could not create command: %s", err)
	}

	// During remote provisioning (before StartSchemeServer runs) stderr is the
	// only live back-channel: stdout is the gRPC pipe and does not serve yet.
	// Read it line by line so structured progress lines surface as browser
	// notifications immediately, while plain lines accumulate in a bounded tail
	// for the exit-error path below. This goroutine owns stderrRead, so the
	// exit path must read the tail instead of the pipe to avoid two readers
	// fighting over it.
	tail := newStderrTail()
	stderrDone := make(chan struct{})
	go debug.CapturePanicReport(func() {
		defer close(stderrDone)
		s.scanRemoteStderr(stderrRead, tail)
	})

	conn, err := grpc.Dial("",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// Detect dead SSH transports promptly: without keepalive
		// pings, a remote save (Rename, Stat, ...) can block
		// indefinitely when the transport is silently broken.
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithContextDialer(func(_ context.Context, addr string) (net.Conn, error) {
			return newStdConn(
				log.StandardLogger(), stdoutRead, stdinWrite, false, /* stdio */
				func() {
					closeHook(ErrSSHConnectionClosed)
				})
		}))
	if err != nil {
		return nil, err
	}

	go debug.CapturePanicReport(func() {
		defer remote.Close()
		// Wait for the remote process to exit OR for the scheme's
		// own context to be cancelled. Without the second case, a
		// hung remote (or a Watcher that never delivers the exit
		// status — e.g. when the watcher's send blocks because
		// nothing is reading) would leak this goroutine and the
		// associated SSH session for the lifetime of the test
		// binary.
		var err error
		select {
		case err = <-ch:
		case <-s.ctx.Done():
		}
		if err != nil {
			// The scanner goroutine drains stderrRead; wait for it to
			// observe EOF (process exit closed the write end) so the tail
			// is complete before formatting the error.
			<-stderrDone
			err = fmt.Errorf("error executing remote rune workspace "+
				"server over SSH: %s: %s", err, tail.String())
		}
		closeHook(err)
		for _, closer := range closers {
			_ = closer.Close()
		}
	})

	return workspacerpc.NewClient(s.ctx, conn), nil
}

// scanRemoteStderr reads the remote server's stderr line by line until EOF.
// Structured provisioning progress lines drive a single live progress
// notification (index/total → progress bar); every other line is appended to
// tail, which the exit-error path reads to build the human-readable failure
// message.
func (s *scheme) scanRemoteStderr(r io.Reader, tail *stderrTail) {
	scanner := bufio.NewScanner(r)
	// Allow long remote stderr lines (default is 64 KiB, but a stack trace or
	// long path can exceed that). Cap growth to keep memory bounded.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var progress provisionProgressNotifier
	for scanner.Scan() {
		line := scanner.Bytes()
		if p, ok := ParseProvisionProgressLine(line); ok {
			progress.report(s.ui, p)
			continue
		}
		tail.append(line)
	}
}

// provisionProgressNotifier maps the stream of ProvisionProgress lines onto a
// single live progress notification: the first line creates it, later lines
// advance its bar. Progress is the count of packages that have finished
// (activated or failed), clamped below total until the terminal done line so
// the notification does not close before provisioning is actually complete. A
// failed package is additionally surfaced as its own warning notification so it
// is not lost inside the info-level progress bar.
type provisionProgressNotifier struct {
	id        string
	started   bool
	completed int
}

func (n *provisionProgressNotifier) report(ui UI, p ProvisionProgress) {
	if p.Phase == ProvisionPhaseFailed {
		ui.Notify(NotificationWarning, p.Message())
	}
	// total must be positive for the progress bar; the remote always sends a
	// positive total, but guard so a malformed line cannot break the bar.
	if p.Total <= 0 {
		return
	}
	if !n.started {
		n.id = ui.Notify(NotificationInfo, p.Message())
		n.started = true
	}

	progress := n.completed
	switch p.Phase {
	case ProvisionPhaseActivating, ProvisionPhaseFailed:
		n.completed++
		progress = n.completed
	case ProvisionPhaseDone:
		progress = p.Total
	}
	// Keep the bar strictly below total until the done line, since a per-package
	// activating/failed for the last package shares total's index and would
	// otherwise close the notification before provisioning finishes.
	if p.Phase != ProvisionPhaseDone && progress >= p.Total {
		progress = p.Total - 1
	}
	ui.UpdateNotificationProgress(n.id, p.Message(), progress, p.Total)
}

// stderrTailCap bounds the human-readable stderr the local side retains, so a
// chatty remote cannot grow the buffer without limit. Only the most recent
// bytes are kept, which is what a failure message needs.
const stderrTailCap = 8 * 1024

// stderrTail is a bounded, concurrency-safe buffer holding the last
// stderrTailCap bytes of non-progress stderr for the exit-error message.
type stderrTail struct {
	mu  sync.Mutex
	buf []byte
}

func newStderrTail() *stderrTail {
	return &stderrTail{}
}

func (t *stderrTail) append(line []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, line...)
	t.buf = append(t.buf, '\n')
	if len(t.buf) > stderrTailCap {
		t.buf = t.buf[len(t.buf)-stderrTailCap:]
	}
}

func (t *stderrTail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

func (s *scheme) setPipes(
	cmd *workspaceapi.Cmd,
) (stdout, stderr, stdin *os.File, closers []io.Closer, err error) {
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("pipe: %v", err)
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("pipe: %v", err)
	}
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("pipe: %v", err)
	}
	cmd.Stdout = stdoutWrite
	cmd.Stderr = stderrWrite
	cmd.Stdin = stdinRead
	closers = []io.Closer{
		stdoutWrite, stderrWrite, stdinWrite,
		stdoutRead, stderrRead, stdinRead,
	}
	return stdoutRead, stderrRead, stdinWrite, closers, nil
}

func (s *scheme) init(
	ctx context.Context, cc sshConfig, uri workspaceapi.URI,
) (err error) {
	if uri.Scheme() != Scheme {
		return errors.New("invalid non-ssh scheme")
	}

	s.cfg = cc
	s.user, s.homedir, s.hostPort, s.basePath, err = parseWorkspaceURI(uri, s.getUser)
	if err != nil {
		return fmt.Errorf("could not parse ssh workspaceapi.URI: %s", err)
	}

	// expand any relative path or home aliases
	uri, err = s.URI(uri.Path())
	if err != nil {
		return fmt.Errorf("URI from path %s: %v", uri.Path(), err)
	}

	s.Scheme = newRemoteScheme(ctx, s.connectSchemeFn, uri)
	return nil
}

// override to provide user with an error message that guides to a solution
func (s *scheme) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	pid, err := s.Scheme.StartCommand(ctx, cmd)
	if err != nil && strings.Contains(err.Error(), "executable file not found in $PATH") {
		return 0, fmt.Errorf("%w. Make sure that $PATH is configured "+
			"even for non-interactive shells", err)
	}
	return pid, err
}

func (s *scheme) URI(path string) (workspaceapi.URI, error) {
	absPath, err := s.expandPath(path)
	if err != nil {
		return workspaceapi.URI{}, err
	}

	var uriStr string
	if s.user != "" {
		uriStr = fmt.Sprintf("ssh://%s@%s%s", s.user, s.hostPort, absPath)
	} else {
		uriStr = fmt.Sprintf("ssh://%s%s", s.hostPort, absPath)
	}

	return workspaceapi.ParseURI(uriStr)
}

func (s *scheme) Close() (ret error) {
	if err := s.Scheme.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	s.cancelCtx()
	return ret
}

func (s *scheme) expandPath(path string) (string, error) {
	return workspaceapi.ExpandPath(path, func() (*user.User, error) {
		return &user.User{Username: s.user, HomeDir: s.homedir}, nil
	}, func() (string, error) {
		return s.basePath, nil
	})
}
