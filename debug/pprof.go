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

package debug

import (
	"fmt"
	"net"
	"net/http"
	// pprof handlers register themselves on http.DefaultServeMux on import.
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// StartPProfHTTP binds a pprof HTTP server to addr (e.g.
// "127.0.0.1:0" for a random port) and serves it in a background
// goroutine. It enables block and mutex profiling as a side-effect so
// the corresponding pprof endpoints have non-empty output. It returns
// the listening address as "host:port".
//
// addr follows the same syntax as net.Listen("tcp", ...). Callers that
// want a random port should pass "127.0.0.1:0".
func StartPProfHTTP(addr string) (string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("pprof listen %q: %w", addr, err)
	}
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)
	bound := ln.Addr().String()
	log.Infof("pprof server listening on http://%s/debug/pprof/", bound)
	go CapturePanicReport(func() {
		if err := http.Serve(ln, nil); err != http.ErrServerClosed {
			log.Errorf("pprof serve: %v", err)
		}
	})
	return bound, nil
}

// StartPProfOnSignal installs a SIGUSR1 handler that, on the first
// signal, starts a pprof HTTP server bound to a random localhost port
// and logs the listening address at info level so the caller can find
// it. Subsequent SIGUSR1 signals are ignored.
func StartPProfOnSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go CapturePanicReport(func() {
		<-ch
		signal.Stop(ch)
		if _, err := StartPProfHTTP("127.0.0.1:0"); err != nil {
			log.Errorf("StartPProfOnSignal: %v", err)
		}
	})
}
