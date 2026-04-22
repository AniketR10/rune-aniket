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

package extensionv2

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ernestrc/logd-go/logging"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/ide/ideauthorizer"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/processctx"
)

var _ extension.Runner = (*workspaceRunner)(nil)

// Runner satisfies extension.Runner with a simple
// protocol that initially exchanges metadata and secrets
// over stdin/stdout and secures resources via TLS and
// per rpc authentication/authorization.
type workspaceRunner struct {
	cfg       runnerConfig
	workspace workspaceapi.URI
	dataDir   string
	grantor   extension.Grantor
	executor  schemeapi.Executor
	socket    string
	tlsCert   []byte
	keys      auth.Keys
	ctx       context.Context
	cancelCtx func()
	mu        sync.Mutex
	states    map[string]*extensionRunState
}

type extensionRunState struct {
	id         string
	cmdAndArgs string
	config     config.Config
	cancel     context.CancelFunc
	pid        workspaceapi.Pid
	started    time.Time
	running    bool
	lastErr    error
	startCount int
	readiness  *extensionReadiness
}

type extensionRunStateSnapshot struct {
	ID         string
	CmdAndArgs string
	Config     config.Config
	Pid        workspaceapi.Pid
	Started    time.Time
	Running    bool
	LastErr    error
	StartCount int
}

var _ schemeapi.Executor = (*workspaceRunner)(nil)
var _ extension.Runner = (*workspaceRunner)(nil)

func newWorkspaceRunner(
	executor schemeapi.Executor, grantor extension.Grantor,
	workspace workspaceapi.URI, socket, dataDir string,
	tlsCert []byte, keys auth.Keys, opts ...Option,
) *workspaceRunner {
	ret := new(workspaceRunner)
	ret.init(executor, grantor, workspace,
		socket, dataDir, tlsCert, keys, opts...)
	return ret
}

// Init initializes this Runner with the given grantor and options.
func (m *workspaceRunner) init(
	executor schemeapi.Executor, grantor extension.Grantor,
	workspace workspaceapi.URI, socket, dataDir string,
	tlsCert []byte, keys auth.Keys, opts ...Option,
) {
	m.ctx, m.cancelCtx = context.WithCancel(context.Background())
	m.states = make(map[string]*extensionRunState)
	m.cfg.authCertEnv = "RUNE_CERT"
	m.cfg.authTokenEnv = "RUNE_TOKEN"
	m.cfg.socketEnv = "RUNE_SOCKET"
	m.cfg.dataDirEnv = "RUNE_DATADIR"
	for _, o := range opts {
		o(&m.cfg)
	}
	m.grantor = grantor
	m.keys = keys
	m.executor = executor
	m.socket = socket
	m.dataDir = dataDir
	m.workspace = workspace
	m.tlsCert = tlsCert
}

// StartCommand runs an ad-hoc program and authorizes it to access resources.
func (m *workspaceRunner) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	var err error
	env, err := m.commandEnvs(ctx, cmd.Path, cmd.Args)
	if err != nil {
		return 0, err
	}
	cmd.Env = append(cmd.Env, env...)
	// emulate the same logic as file scheme
	dir, err := workspaceapi.ExpandPathWithURI(m.workspace.Path(), m.workspace)
	if err == nil {
		cmd.Dir = dir
	}
	return m.executor.StartCommand(ctx, cmd)
}

// Signal sends a signal to the running process.
func (m *workspaceRunner) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	return m.executor.Signal(pid, sig)
}

// Run runs the given extension with an executable at the given path,
// with the given config.
func (m *workspaceRunner) Run(id, cmdAndArgs string, config config.Config) error {
	if id == "" || cmdAndArgs == "" {
		return errors.New("extension id and cmd must not be empty")
	}

	m.mu.Lock()
	if state := m.states[id]; state != nil && state.running {
		m.mu.Unlock()
		return fmt.Errorf("extension %q is already running", id)
	}
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(m.ctx)
	ctx = processctx.ContextWithExtensionID(ctx, id)
	readiness := newExtensionReadiness()
	cmd, err := m.makeCommand(ctx, id, cmdAndArgs, config, readiness)
	if err != nil {
		cancel()
		return fmt.Errorf("make command: %w", err)
	}

	pid, err := m.executor.StartCommand(ctx, cmd)
	if err != nil {
		cancel()
		return fmt.Errorf("start command: %w", err)
	}

	m.log(log.DebugLevel, "running extension with name %q at path %q, pid: %d",
		id, cmdAndArgs, pid)

	m.mu.Lock()
	state := m.states[id]
	if state == nil {
		state = &extensionRunState{id: id}
		m.states[id] = state
	}
	state.id = id
	state.cmdAndArgs = cmdAndArgs
	state.config = config
	state.cancel = cancel
	state.pid = pid
	state.started = time.Now()
	state.running = true
	state.lastErr = nil
	state.startCount++
	state.readiness = readiness
	m.mu.Unlock()

	return nil
}

// Close stops all extensions and cleans up all resources associated
// with this Runner.
func (m *workspaceRunner) Close() error {
	m.cancelCtx() // kills all extensions
	return nil
}

func (m *workspaceRunner) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithFields(log.Fields{
		logging.KeyClass: "extensionv2.workspaceRunner",
		"workspace":      m.workspace.String(),
	}).Logf(level, msg, args...)
}

func (m *workspaceRunner) makeCommand(
	ctx context.Context, extensionID, path string, config config.Config,
	readiness *extensionReadiness,
) (ret workspaceapi.Cmd, err error) {
	waitCh := make(chan error)
	go debug.CapturePanicReport(func() {
		select {
		case err := <-waitCh:
			m.setExtensionExit(extensionID, err)
			if err != nil {
				m.log(log.ErrorLevel, "extension %s exit: %v", extensionID, err)
			}
		case <-m.ctx.Done():
		}
	})
	// allow args to be passed to extensions
	argv := strings.Split(path, " ")
	ret = workspaceapi.Cmd{
		Path:    argv[0],
		Args:    argv[1:],
		Dir:     m.dataDir, // default
		Watcher: workspaceapi.ChanProcessWatcher(waitCh),
	}

	// if local workspace, then do set dir in a best effort for
	// extensions that do not use APIs and call os functions directly.
	if m.workspace.Scheme() == workspace.FileScheme {
		var err error
		ret.Dir, err = workspaceapi.ExpandPathWithURI(m.workspace.Path(), m.workspace)
		if err != nil {
			err = fmt.Errorf("could not expand workspace "+
				"path: %q: %w", m.workspace.Path(), err)
			return workspaceapi.Cmd{}, err
		}
	}

	ret.Env, err = m.commandEnvs(ctx, ret.Path, ret.Args)
	if err != nil {
		return workspaceapi.Cmd{}, err
	}
	ret.Stdin, ret.Stdout, ret.Stderr = m.makeProtocolExchange(extensionID, config, readiness)
	return
}

func (m *workspaceRunner) commandEnvs(ctx context.Context, path string, args []string) ([]string, error) {
	// TODO expire token manually when program finishes
	const tokenExpiresIn = 24 * 365 * time.Hour

	env := []string{makeLogLevelEnv(log.GetLevel())}
	env = append(env, fmt.Sprintf("%s=%s", m.cfg.socketEnv, m.socket))
	env = append(env, fmt.Sprintf("%s=%s", m.cfg.dataDirEnv, m.dataDir))
	cert := base64.StdEncoding.EncodeToString(m.tlsCert)
	env = append(env, fmt.Sprintf("%s=%s", m.cfg.authCertEnv, cert))
	if m.cfg.insecureAuth {
		return env, nil
	}

	signKey, err := m.keys.Sign(ctx)
	if err != nil {
		return nil, fmt.Errorf("get sign key: %w", err)
	}
	name := fmt.Sprintf("%s_%s", path, strings.Join(args, "_"))
	id := uuid.New().String()
	if extensionID, ok := processctx.ExtensionIDFromContext(ctx); ok {
		id = extensionID
	}
	permissions := extensionapi.AllPermissions()
	m.log(log.DebugLevel, "creating one shot authentication for "+
		"command %s, id: %s, permissions: %v", name, id, permissions)
	claimsExtra := ideauthorizer.Extension{
		Metadata: extensionapi.Metadata{
			DeveloperID:   "you",
			DeveloperKey:  "",
			ExtensionID:   id,
			ExtensionName: name,
			Permissions:   permissions,
		},
		Plugin: true,
		Path:   path,
		Args:   append([]string(nil), args...),
	}
	accessToken, err := auth.SignToken(signKey,
		claimsExtra.DeveloperID, claimsExtra.DeveloperEmail, claimsExtra,
		tokenExpiresIn)
	if err != nil {
		return nil, fmt.Errorf("sign token: %w", err)
	}
	env = append(env, fmt.Sprintf("%s=%s", m.cfg.authTokenEnv, accessToken))
	return env, nil
}

func (m *workspaceRunner) makeProtocolExchange(
	extensionID string, cfg config.Config, readiness *extensionReadiness,
) (
	io.Reader, io.Writer, io.Writer,
) {
	protocol := newProtocol(m.ctx, m.grantor, extensionID, m.socket,
		m.dataDir, m.tlsCert, m.cfg.insecureAuth, cfg, m.keys, readiness)
	collector := newCollector(extensionID, m.workspace)
	stdout := errIntercept{
		m:           m,
		extensionID: extensionID,
		protocol:    protocol,
	}
	return protocol, stdout, collector
}

func (m *workspaceRunner) setExtensionExit(extensionID string, err error) {
	m.mu.Lock()
	state := m.states[extensionID]
	if state == nil {
		m.mu.Unlock()
		return
	}
	state.running = false
	state.pid = 0
	state.cancel = nil
	state.lastErr = err
	readiness := state.readiness
	m.mu.Unlock()
	readiness.Set(err)
}

func (m *workspaceRunner) listExtensions() []extensionRunStateSnapshot {
	m.mu.Lock()
	ret := make([]extensionRunStateSnapshot, 0, len(m.states))
	for _, state := range m.states {
		ret = append(ret, extensionRunStateSnapshot{
			ID:         state.id,
			CmdAndArgs: state.cmdAndArgs,
			Config:     state.config,
			Pid:        state.pid,
			Started:    state.started,
			Running:    state.running,
			LastErr:    state.lastErr,
			StartCount: state.startCount,
		})
	}
	m.mu.Unlock()
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].ID < ret[j].ID
	})
	return ret
}

func (m *workspaceRunner) extension(id string) (extensionRunStateSnapshot, bool) {
	m.mu.Lock()
	state := m.states[id]
	m.mu.Unlock()
	if state == nil {
		return extensionRunStateSnapshot{}, false
	}
	return extensionRunStateSnapshot{
		ID:         state.id,
		CmdAndArgs: state.cmdAndArgs,
		Config:     state.config,
		Pid:        state.pid,
		Started:    state.started,
		Running:    state.running,
		LastErr:    state.lastErr,
		StartCount: state.startCount,
	}, true
}

func (m *workspaceRunner) startExtension(
	ctx context.Context, id, cmdAndArgs string, cfg config.Config,
) error {
	if err := m.Run(id, cmdAndArgs, cfg); err != nil {
		return err
	}
	// Run just stored a fresh readiness in the state. Capture it and wait
	// for the protocol handshake (or a stop/exit) to resolve it.
	m.mu.Lock()
	readiness := m.states[id].readiness
	m.mu.Unlock()
	return readiness.Wait(ctx)
}

func (m *workspaceRunner) stopExtensionByID(id string) error {
	m.mu.Lock()
	state := m.states[id]
	m.mu.Unlock()
	if state == nil {
		return fmt.Errorf("extension %q not found", id)
	}
	if !state.running {
		return fmt.Errorf("extension %q is not running", id)
	}
	m.stopExtension(id, nil)
	return nil
}

func (m *workspaceRunner) restartExtension(ctx context.Context, id string) error {
	m.mu.Lock()
	state := m.states[id]
	if state == nil {
		m.mu.Unlock()
		return fmt.Errorf("extension %q not found", id)
	}
	cmdAndArgs := state.cmdAndArgs
	cfg := state.config
	running := state.running
	m.mu.Unlock()
	if cmdAndArgs == "" {
		return fmt.Errorf("extension %q has no stored command", id)
	}
	if running {
		if err := m.stopExtensionByID(id); err != nil {
			return err
		}
	}
	return m.startExtension(ctx, id, cmdAndArgs, cfg)
}

func (m *workspaceRunner) stopExtension(extensionID string, reason error) {
	m.log(log.WarnLevel, "stopping extension %q: reason: %v", extensionID, reason)
	m.mu.Lock()
	state := m.states[extensionID]
	if state == nil {
		m.mu.Unlock()
		return
	}
	cancel := state.cancel
	state.cancel = nil
	state.running = false
	state.pid = 0
	state.lastErr = reason
	readiness := state.readiness
	m.mu.Unlock()
	readiness.Set(reason)
	if cancel != nil {
		cancel()
	}
}

func makeLogLevelEnv(l log.Level) string {
	return fmt.Sprintf("%s=%s", extensionapi.EnvLogLevel, l)
}

type errIntercept struct {
	m           *workspaceRunner
	protocol    io.Writer
	extensionID string
}

func (e errIntercept) Write(data []byte) (int, error) {
	n, err := e.protocol.Write(data)
	if err != nil {
		e.m.stopExtension(e.extensionID, err)
		return n, err
	}
	return n, err
}
