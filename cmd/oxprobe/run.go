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
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/unstablebuild/ox-api/api/oxapi"
	"github.com/unstablebuild/ox-api/api/pager"

	"unstable.build/go-tui/cmd/oxprobe/probe"
	"unstable.build/go-tui/debug"
)

// runbookURL is attached to every page so on-call engineers reach the
// oxprobe mitigation guide and metrics dashboard from the incident.
const runbookURL = "https://x.unstable.build/docs/runbooks/oxprobe"

// runner executes a fixed probe set against one environment and folds
// the results into a Report. It is reused across daemon iterations.
type runner struct {
	env       EnvConfig
	probes    []probe.Probe
	deep      probe.DeepProbe
	deepFanIn bool
	// client is reused for the anonymous signed-download leg of the
	// pkg_download check.
	client *http.Client
	// arch is the "<os>-<arch>" whose signed download URL the
	// pkg_download check fetches from the deep report.
	arch string
}

type runnerConfig struct {
	SkipDownloadsCDN bool
}

func newRunner(cfg EnvConfig, client *http.Client, probeSecret string, runCfg runnerConfig) *runner {
	r := &runner{
		env:    cfg,
		probes: buildProbes(cfg, client, runCfg.SkipDownloadsCDN),
		client: client,
		arch:   probeArch(),
	}
	if probeSecret != "" {
		r.deep = deepProbe(cfg, client, probeSecret)
		r.deepFanIn = true
	}
	return r
}

// run executes every probe concurrently and returns the aggregated
// report. Each probe is bounded by perProbeTimeout.
func (r *runner) run(ctx context.Context, perProbeTimeout time.Duration) oxapi.Report {
	results := make([][]oxapi.CheckResult, len(r.probes)+1)
	var wg sync.WaitGroup

	for i, p := range r.probes {
		wg.Add(1)
		go debug.CapturePanicReport(func() {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, perProbeTimeout)
			defer cancel()
			results[i] = []oxapi.CheckResult{p.Run(pctx)}
		})
	}

	if r.deepFanIn {
		wg.Add(1)
		go debug.CapturePanicReport(func() {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, perProbeTimeout)
			defer cancel()
			fanned, report, ok := r.deep.Fanout(pctx)
			if ok {
				fanned = append(fanned, probe.PkgDownloadResult(
					pctx, r.client, r.arch, report.SignedDownloads[r.arch], true))
			}
			results[len(r.probes)] = fanned
		})
	}

	wg.Wait()

	var flat []oxapi.CheckResult
	for _, rs := range results {
		flat = append(flat, rs...)
	}
	sort.SliceStable(flat, func(i, j int) bool { return flat[i].Layer < flat[j].Layer })
	return oxapi.Aggregate(flat)
}

// reconcile sends one PagerDuty event per critical layer: a trigger for
// failing layers and a resolve for passing ones. The resolve is keyed by
// the same per-env+layer dedup key as the trigger so a recovered layer
// closes its own incident. Resolves are idempotent, so reconciling a
// never-paged layer is a no-op on PagerDuty's side.
func reconcile(ctx context.Context, p pager.Pager, env string, report oxapi.Report) error {
	var firstErr error
	for _, c := range report.Checks {
		if !c.Critical {
			continue
		}
		dedupKey := fmt.Sprintf("oxprobe-%s-%s", env, c.Layer)
		var evt pager.Page
		if c.Status == oxapi.CheckFail {
			evt = pager.Page{
				Summary:   fmt.Sprintf("oxprobe %s: %s failing — %s", env, c.Layer, c.Detail),
				Source:    "oxprobe",
				Severity:  pager.SeverityCritical,
				Component: c.Layer,
				Group:     env,
				Class:     "synthetic-probe",
				DedupKey:  dedupKey,
				Details: map[string]any{
					"env":        env,
					"layer":      c.Layer,
					"detail":     c.Detail,
					"latency_ms": c.LatencyMS,
				},
				Links: []pager.Link{{Href: runbookURL, Text: "oxprobe runbook"}},
			}
		} else {
			evt = pager.Page{
				Action:   pager.ActionResolve,
				DedupKey: dedupKey,
			}
		}
		if err := p.Page(ctx, evt); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
