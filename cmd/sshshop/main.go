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

package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gliderlabs/ssh"
	"unstable.build/go-tui/debug"
)

func main() {
	debug.StartPProfOnSignal()
	hostKey := filepath.Join(os.TempDir(), "host_ed25519")
	var cfg serverConfig
	flag.StringVar(&cfg.addr, "addr", ":2222",
		"TCP listen address (use :22 in production behind a TCP LB)")
	flag.StringVar(&cfg.hostKeyPath, "host-key", hostKey,
		"path to host ed25519 private key (created if missing)")
	flag.DurationVar(&cfg.idleTimeout, "idle-timeout", 10*time.Minute,
		"disconnect idle clients after this much wall time")
	flag.DurationVar(&cfg.maxTimeout, "max-timeout", 2*time.Hour,
		"hard upper bound on session duration")
	flag.StringVar(&cfg.banner, "banner",
		"sshshop — prototype storefront. Identity happens in-app.\r\n",
		"pre-auth banner shown to clients")
	shutdownGrace := flag.Duration("shutdown-grace", 30*time.Second,
		"grace period for active sessions on shutdown")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	srv, err := newServer(cfg, logger)
	if err != nil {
		logger.Error("build server", "err", err)
		os.Exit(1)
	}

	// Listen-and-serve in its own goroutine so we can react to signals.
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.addr)
		serveErr <- srv.ListenAndServe()
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			logger.Error("serve", "err", err)
			os.Exit(1)
		}
		return
	case sig := <-sigs:
		logger.Info("shutdown requested", "signal", sig.String(),
			"grace", shutdownGrace.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), *shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Warn("graceful shutdown", "err", err)
		if err := srv.Close(); err != nil {
			logger.Warn("force close", "err", err)
		}
	}
	logger.Info("stopped")
}
