// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	"unstable.build/rune/debug"
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
