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
	"crypto/tls"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/retry"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/ide/ideauthorizer"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace/processctx"
)

// NewRunner returns a Runner with a simple protocol that
// initially exchanges metadata and secrets over stdin/stdout and secures
// resources via TLS and per rpc authentication/authorization.
func NewRunner(
	ctx context.Context, locker sync.Locker, dataDir string, opts ...Option,
) (*Runner, error) {
	ret := &Runner{
		locker:  locker,
		dataDir: dataDir,
		opts:    opts,
	}
	var err error
	ret.keys, err = auth.GenerateKeys()
	if err != nil {
		return nil, fmt.Errorf("generate keys: %w", err)
	}
	for _, o := range opts {
		o(&ret.cfg)
	}
	return ret, nil
}

const certExpiresIn = 10 * 24 * 365 * time.Hour

// Runner implements the extension host gRPC server lifecycle.
type Runner struct {
	opts    []Option
	cfg     runnerConfig
	locker  sync.Locker
	keys    auth.Keys
	dataDir string
}

// WorkspaceExtensionsRunner creates an extension runner for a workspace. It
// starts a gRPC server that hosts the extension resources and returns a runner
// that can launch extensions and execute commands in the workspace.
func (r *Runner) WorkspaceExtensionsRunner(
	uri workspaceapi.URI, res map[extensionapi.Permission]extension.ResourceRegistrar,
	authorizer *ideauthorizer.Authorizer,
	dataDir string, notifications browser.Notifications,
	executor schemeapi.Executor,
	grantor extension.Grantor,
	editor text.Editor,
	promptOpener ideauthorizer.PromptOpener, storage storageapi.Service,
	scheduleNextTick func(func()) bool,
) (extension.Runner, error) {
	var ret wrapCloser
	ret.URI = uri

	listener, err := r.newUnixListener(uri)
	if err != nil {
		return nil, fmt.Errorf("create unix listener: %w", err)
	}
	socket := listener.Addr().String()
	listener = newPeerProcessListener(listener)

	streamInterceptors := []grpc.StreamServerInterceptor{
		rpc.StreamReportRecoveryInterceptor(),
		extensionIDStreamInterceptor(),
	}
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		rpc.UnaryReportRecoveryInterceptor(),
	}
	if log.IsLevelEnabled(log.DebugLevel) {
		fields := []logging.Field{
			{Key: logging.KeyClass, Value: "grpc.Server"},
			{Key: "workspace", Value: uri.String()},
		}
		streamInterceptors = append(streamInterceptors, rpc.StreamLoggingInterceptor(fields))
		unaryInterceptors = append(unaryInterceptors, rpc.UnaryLoggingInterceptor(fields))
	}
	opts := []grpc.ServerOption{
		grpc.ChainStreamInterceptor(streamInterceptors...),
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
	}
	var cert, key []byte
	if !r.cfg.insecureTransport {
		cert, key, err = auth.GenerateSelfSignedCert(
			[]string{socket}, pkix.Name{CommonName: "ox"}, certExpiresIn)
		if err != nil {
			if cerr := listener.Close(); cerr != nil {
				err = multierror.Append(err, cerr)
			}
			return nil, fmt.Errorf("new extension runner: %v", err)
		}
		tlsCert, err := tls.X509KeyPair(cert, key)
		if err != nil {
			if cerr := listener.Close(); cerr != nil {
				err = multierror.Append(err, cerr)
			}
			return nil, fmt.Errorf("load tls credentials from cert and key: %w", err)
		}
		cfg := tls.Config{
			Certificates:       []tls.Certificate{tlsCert},
			InsecureSkipVerify: true,
		}
		creds := credentials.NewTLS(&cfg)
		if r.cfg.insecureAuth {
			opts = append(opts, grpc.Creds(creds))
		} else {
			opts = append(opts, authorizer.GRPCAuthServerOptions(r.keys, creds)...)
		}
	} else if !r.cfg.insecureAuth {
		opts = append(opts, authorizer.GRPCAuthServerOptions(r.keys, nil)...)
	}
	ret.srv = grpc.NewServer(opts...)
	for _, registrar := range res {
		closer, rerr := registrar.Register(ret.srv, r.locker)
		if rerr != nil {
			err = multierror.Append(err, rerr)
			continue
		}
		ret.closers = append(ret.closers, closer)
	}
	if err != nil {
		if cerr := ret.Close(); cerr != nil {
			err = multierror.Append(err, cerr)
			return nil, err
		}
	}

	go debug.CapturePanicReport(func() {
		_ = ret.srv.Serve(listener)
	})

	ret.workspaceRunner = newWorkspaceRunner(executor, grantor, uri,
		socket, r.dataDir, cert, r.keys, r.opts...)
	if err != nil {
		err = fmt.Errorf("new workspace runner: %w", err)
		if cerr := ret.Close(); cerr != nil {
			err = multierror.Append(err, cerr)
			return nil, err
		}
		return nil, err
	}
	if err := registerExtensionsREPLCommand(ret.workspaceRunner, editor); err != nil {
		err = fmt.Errorf("register extensions repl command: %w", err)
		if cerr := ret.Close(); cerr != nil {
			err = multierror.Append(err, cerr)
			return nil, err
		}
		return nil, err
	}
	return ret, nil
}

func extensionIDStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) error {
		claims, ok := auth.ClaimsFromContext[ideauthorizer.Extension](stream.Context())
		if ok && claims.Extra.ExtensionID != "" {
			stream = contextServerStream{
				ServerStream: stream,
				ctx: processctx.ContextWithExtensionID(
					stream.Context(), claims.Extra.ExtensionID),
			}
		}
		return handler(srv, stream)
	}
}

type contextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s contextServerStream) Context() context.Context {
	return s.ctx
}

func (r *Runner) newUnixListener(uri workspaceapi.URI) (ret net.Listener, err error) {
	ctx := context.Background()
	socket := path.Join(uri.Path(), fmt.Sprintf(".%s.sock", debug.Package))
	err = retry.Retry(ctx, retrySocketStrategy, func(context.Context) (bool, error) {
		var cfg net.ListenConfig
		ret, err = cfg.Listen(ctx, "unix", socket)
		if err == nil {
			return false, nil
		}

		if errors.Is(err, syscall.EACCES) {
			return false, err
		}

		if errors.Is(err, syscall.ENOENT) { // a component of the path does not exist
			mkdirErr := os.MkdirAll(filepath.Dir(socket), 0766)
			if mkdirErr != nil {
				err = fmt.Errorf("listen: %w", err)
				return false, multierror.Append(err, mkdirErr)
			}
			return true, err
		}

		_ = os.Remove(socket)
		return true, err
	})
	return
}

var _ schemeapi.Executor = wrapCloser{}

type wrapCloser struct {
	workspaceapi.URI
	*workspaceRunner
	closers []io.Closer
	srv     *grpc.Server
}

func (m wrapCloser) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	return m.workspaceRunner.Signal(pid, sig)
}

func (m wrapCloser) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	return m.workspaceRunner.StartCommand(ctx, cmd)
}

func (w wrapCloser) Close() (ret error) {
	log.Tracef("closing workspace %q extensions", w.URI.String())
	for _, closer := range w.closers {
		if err := closer.Close(); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	if w.workspaceRunner != nil {
		if err := w.workspaceRunner.Close(); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	if w.srv != nil {
		w.srv.Stop() // stop closes listener
	}
	return
}

var retrySocketStrategy = retry.CombinedStrategy(
	retry.LimitStrategy(4),
	retry.ExponentialStrategy(time.Millisecond, time.Second),
)
