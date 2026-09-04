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

// Package probe defines black-box synthetic health probes for the ox-api
// fleet. Each Probe exercises one externally observable layer (DNS, TLS,
// OAuth config, the auth0 tenant, the downloads CDN, and the deep
// /health endpoint) and reports a CheckResult using the same shape the
// ox-api server emits, so probe output and server output stay coherent.
package probe

import (
	"context"
	"time"

	"github.com/unstablebuild/ox-api/api/oxapi"
)

// Probe runs a single black-box check against a deployed environment.
type Probe interface {
	// Layer is the stable layer identifier, e.g. "dns".
	Layer() string
	// Critical reports whether a failure should fail the overall run
	// (and page) rather than merely degrade it.
	Critical() bool
	// Run executes the probe and returns its result. Implementations
	// must respect ctx cancellation and never panic.
	Run(ctx context.Context) oxapi.CheckResult
}

// run is a helper that times f, captures its error/panic, and builds a
// CheckResult for the given layer.
func run(ctx context.Context, layer string, critical bool, f func(ctx context.Context) (detail string, err error)) (res oxapi.CheckResult) {
	res = oxapi.CheckResult{Layer: layer, Status: oxapi.CheckOK, Critical: critical}
	start := time.Now()
	defer func() {
		res.LatencyMS = time.Since(start).Milliseconds()
		if r := recover(); r != nil {
			res.Status = oxapi.CheckFail
			res.Detail = "panic"
			_ = r
		}
	}()
	detail, err := f(ctx)
	if err != nil {
		res.Status = oxapi.CheckFail
		res.Detail = err.Error()
		return res
	}
	res.Detail = detail
	return res
}
