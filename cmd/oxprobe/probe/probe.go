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
