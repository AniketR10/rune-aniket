// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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


package idedebug

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-dap"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/workspace/processctx"
)

type debugServer struct {
	mu         sync.Mutex
	writeMu    sync.Mutex
	wg         sync.WaitGroup
	closeOnce  sync.Once
	ctx        context.Context
	cancel     func()
	stopCalled bool
	cfg        debugConfig
	binPath    string
	rootURI    string
	pid        workspaceapi.Pid
	watcher    chan error
	executor   schemeapi.Executor
	conn       net.Conn
	reader     *bufio.Reader
	seq        atomic.Int64
	pending    map[int]chan dap.Message
	pendingMu  sync.Mutex
	eventSub   EventSubscriber
	alive      bool
	log        *slog.Logger
	caps       *dap.Capabilities
	// launchErr stores the error from a failed DAP launch or
	// attach response. These requests use writeRequest
	// (fire-and-forget), so the response has no pending
	// channel. readLoop captures the error here so that
	// ConfigurationDone can surface it.
	launchErr error
}

func newDebugServer(
	ctx context.Context,
	cfg debugConfig,
	binPath string,
	executor schemeapi.Executor,
	rootURI string,
	eventSub EventSubscriber,
) *debugServer {
	ctx, cancel := context.WithCancel(ctx)
	return &debugServer{
		ctx:      ctx,
		cancel:   cancel,
		cfg:      cfg,
		binPath:  binPath,
		executor: executor,
		rootURI:  rootURI,
		eventSub: eventSub,
		pending:  make(map[int]chan dap.Message),
		log:      slog.With("struct", "idedebug.debugServer", "adapter", cfg.id, "uri", rootURI),
	}
}

func (s *debugServer) start(ctx context.Context) error {
	// Pick a free TCP port for the adapter to listen on.
	addr, err := findFreeAddr()
	if err != nil {
		return fmt.Errorf("find free addr: %w", err)
	}

	// Replace the {addr} placeholder in the config args
	// with the concrete address.
	args := make([]string, len(s.cfg.args))
	for i, a := range s.cfg.args {
		args[i] = strings.ReplaceAll(a, "{addr}", addr)
	}

	watchCh := make(chan error, 1)
	watcher := workspaceapi.ChanProcessWatcher(watchCh)

	cmd := workspaceapi.Cmd{
		Path:    s.binPath,
		Args:    args,
		Dir:     strings.TrimPrefix(s.rootURI, "file://"),
		Watcher: watcher,
	}

	// Do not use ctx for lifecycle cancellation: it is scoped to the initial
	// protocol exchange. Copy logical process metadata from ctx into the server
	// lifecycle context so debug adapter processes remain associated with the
	// extension that requested them.
	lifecycleCtx := processctx.DeriveCommandContext(s.ctx, ctx)

	s.log.Info("starting server", "path", cmd.Path, "args", cmd.Args)
	pid, err := s.executor.StartCommand(lifecycleCtx, cmd)
	if err != nil {
		return fmt.Errorf("start %s: %w", s.cfg.command, err)
	}

	// Connect to the debug adapter as a client.
	conn, err := dialWithRetry(ctx, addr)
	if err != nil {
		s.cancel() // kill the spawned process
		return fmt.Errorf("connect to %s: %w", addr, err)
	}

	s.mu.Lock()
	s.pid = pid
	s.watcher = watchCh
	s.conn = conn
	s.reader = bufio.NewReader(conn)
	s.alive = true
	s.mu.Unlock()

	s.wg.Add(1)
	go debug.CapturePanicReport(func() {
		s.readLoop()
	})

	// initialize is called without s.mu held because it
	// calls sendRequest which also acquires s.mu.
	caps, err := s.initialize(ctx)
	if err != nil {
		s.closeConn()
		s.cancel() // kill the spawned process
		return fmt.Errorf("initialize %s: %w", s.cfg.command, err)
	}

	s.mu.Lock()
	s.caps = caps
	s.mu.Unlock()
	return nil
}

// findFreeAddr binds to an ephemeral port to discover a
// free address, then releases it. There is a small TOCTOU
// window before the adapter binds to the same port; in
// practice this is negligible and dialWithRetry will surface
// a clear error if it occurs.
func findFreeAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr, nil
}

func dialWithRetry(ctx context.Context, addr string) (net.Conn, error) {
	const (
		maxAttempts = 50
		retryDelay  = 100 * time.Millisecond
	)
	var lastErr error
	for range maxAttempts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, retryDelay)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(retryDelay)
	}
	return nil, fmt.Errorf("dial %s after retries: %w", addr, lastErr)
}

func (s *debugServer) initialize(ctx context.Context) (*dap.Capabilities, error) {
	s.log.Debug("dap initialize")
	req := &dap.InitializeRequest{
		Request: s.newRequest("initialize"),
		Arguments: dap.InitializeRequestArguments{
			ClientID:        "rune",
			ClientName:      "Rune IDE",
			AdapterID:       s.cfg.id,
			LinesStartAt1:   true,
			ColumnsStartAt1: true,
			PathFormat:      "path",
		},
	}
	resp, err := s.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	initResp, ok := resp.(*dap.InitializeResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}
	return &initResp.Body, nil
}

func (s *debugServer) stop(ctx context.Context) error {
	s.mu.Lock()
	s.stopCalled = true
	if !s.alive {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	req := &dap.DisconnectRequest{
		Request: s.newRequest("disconnect"),
		Arguments: &dap.DisconnectArguments{
			TerminateDebuggee: true,
		},
	}
	// best-effort disconnect
	_, _ = s.sendRequest(ctx, req)

	s.mu.Lock()
	s.alive = false
	s.mu.Unlock()

	s.closeConn()
	s.wg.Wait()
	return nil
}

func (s *debugServer) sendRequest(ctx context.Context, req dap.Message) (dap.Message, error) {
	s.mu.Lock()
	if !s.alive {
		s.mu.Unlock()
		return nil, ErrNoServer
	}
	s.mu.Unlock()

	seq := req.GetSeq()
	ch := make(chan dap.Message, 1)
	s.pendingMu.Lock()
	s.pending[seq] = ch
	s.pendingMu.Unlock()

	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, seq)
		s.pendingMu.Unlock()
	}()

	s.writeMu.Lock()
	writeErr := dap.WriteProtocolMessage(s.conn, req)
	s.writeMu.Unlock()
	if writeErr != nil {
		return nil, fmt.Errorf("write request: %w", writeErr)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg, ok := <-ch:
		if !ok || msg == nil {
			return nil, errors.New("server closed connection")
		}
		if resp, ok := msg.(dap.ResponseMessage); ok {
			if !resp.GetResponse().Success {
				return nil, fmt.Errorf("dap error: %s", resp.GetResponse().Message)
			}
		}
		return msg, nil
	}
}

func (s *debugServer) readLoop() {
	defer s.wg.Done()
	for {
		msg, err := dap.ReadProtocolMessage(s.reader)
		if err != nil {
			s.mu.Lock()
			alive := s.alive
			stopCalled := s.stopCalled
			s.mu.Unlock()
			if alive && !stopCalled {
				s.log.Warn("read error", "error", err)
			}
			s.closePending()
			return
		}

		switch m := msg.(type) {
		case dap.ResponseMessage:
			reqSeq := m.GetResponse().RequestSeq
			s.pendingMu.Lock()
			ch, ok := s.pending[reqSeq]
			s.pendingMu.Unlock()
			if ok {
				ch <- msg
			} else {
				resp := m.GetResponse()
				if !resp.Success {
					cmd := resp.Command
					if cmd == "launch" || cmd == "attach" {
						s.mu.Lock()
						s.launchErr = fmt.Errorf("dap error: %s", resp.Message)
						s.mu.Unlock()
					}
				}
				s.log.Debug("unmatched response",
					"requestSeq", reqSeq, "command", resp.Command, "success", resp.Success)
			}
		case dap.EventMessage:
			s.eventSub.OnEvent(m)
		default:
			s.log.Debug("unknown message type", "type", fmt.Sprintf("%T", msg))
		}
	}
}

func (s *debugServer) closePending() {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	for seq, ch := range s.pending {
		close(ch)
		delete(s.pending, seq)
	}
}

// writeRequest sends a DAP request without waiting for a response.
// This is needed for Launch and Attach because their responses only arrive
// after ConfigurationDone.
func (s *debugServer) writeRequest(req dap.Message) error {
	s.mu.Lock()
	if !s.alive {
		s.mu.Unlock()
		return ErrNoServer
	}
	s.mu.Unlock()
	s.writeMu.Lock()
	err := dap.WriteProtocolMessage(s.conn, req)
	s.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("write request: %w", err)
	}
	return nil
}

func (s *debugServer) closeConn() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.alive = false
		s.mu.Unlock()
		if s.conn != nil {
			_ = s.conn.Close()
		}
	})
}

func (s *debugServer) newRequest(command string) dap.Request {
	seq := int(s.seq.Add(1))
	return dap.Request{
		ProtocolMessage: dap.ProtocolMessage{
			Seq:  seq,
			Type: "request",
		},
		Command: command,
	}
}
