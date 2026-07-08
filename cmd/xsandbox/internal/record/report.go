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

package record

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"
)

// ExpectationResult is the outcome of one spec expectation or action.
type ExpectationResult struct {
	Name   string  `json:"name"`
	Pass   bool    `json:"pass"`
	WaitMS float64 `json:"wait_ms"`
	Detail string  `json:"detail,omitempty"`
}

// MethodStats aggregates latency for one RPC method. Latencies for
// streams measure the stream's lifetime.
type MethodStats struct {
	Count int     `json:"count"`
	MinMS float64 `json:"min_ms"`
	AvgMS float64 `json:"avg_ms"`
	P50MS float64 `json:"p50_ms"`
	P95MS float64 `json:"p95_ms"`
	MaxMS float64 `json:"max_ms"`
}

// Report is the machine-readable outcome of a sandbox run.
type Report struct {
	Pass           bool                   `json:"pass"`
	ExtensionCmd   string                 `json:"extension_cmd"`
	ExtensionExit  string                 `json:"extension_exit,omitempty"`
	Metadata       map[string]any         `json:"metadata,omitempty"`
	HandshakeMS    float64                `json:"handshake_ms"`
	WallTimeMS     float64                `json:"wall_time_ms"`
	Expectations   []ExpectationResult    `json:"expectations"`
	UnexpectedRPCs []Snapshot             `json:"unexpected_rpcs,omitempty"`
	Methods        map[string]MethodStats `json:"methods"`
}

// Stats computes per-method latency statistics over all finished
// recorded RPCs.
func (r *Recorder) Stats() map[string]MethodStats {
	durations := make(map[string][]time.Duration)
	for _, ev := range r.Snapshots() {
		if ev.End.IsZero() {
			continue
		}
		durations[ev.Method] = append(durations[ev.Method], ev.End.Sub(ev.Start))
	}
	ret := make(map[string]MethodStats, len(durations))
	for method, ds := range durations {
		ret[method] = computeStats(ds)
	}
	return ret
}

func computeStats(ds []time.Duration) MethodStats {
	slices.Sort(ds)
	var total time.Duration
	for _, d := range ds {
		total += d
	}
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	return MethodStats{
		Count: len(ds),
		MinMS: ms(ds[0]),
		AvgMS: ms(total) / float64(len(ds)),
		P50MS: ms(percentile(ds, 50)),
		P95MS: ms(percentile(ds, 95)),
		MaxMS: ms(ds[len(ds)-1]),
	}
}

// percentile returns the pth percentile of sorted durations using
// the nearest-rank method.
func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := (p*len(sorted) + 99) / 100
	rank = max(1, min(rank, len(sorted)))
	return sorted[rank-1]
}

// WriteText renders a human-readable summary of the report.
func (r *Report) WriteText(w io.Writer, bench bool) {
	status := "PASS"
	if !r.Pass {
		status = "FAIL"
	}
	fmt.Fprintf(w, "%s %s\n", status, r.ExtensionCmd)
	fmt.Fprintf(w, "  handshake: %.2fms  wall time: %.2fms\n",
		r.HandshakeMS, r.WallTimeMS)
	if r.ExtensionExit != "" {
		fmt.Fprintf(w, "  extension exit: %s\n", r.ExtensionExit)
	}
	for _, exp := range r.Expectations {
		mark := "ok"
		if !exp.Pass {
			mark = "FAIL"
		}
		fmt.Fprintf(w, "  [%s] %s (%.2fms)\n", mark, exp.Name, exp.WaitMS)
		if exp.Detail != "" {
			for line := range strings.SplitSeq(strings.TrimRight(exp.Detail, "\n"), "\n") {
				fmt.Fprintf(w, "       %s\n", line)
			}
		}
	}
	if len(r.UnexpectedRPCs) > 0 {
		fmt.Fprintf(w, "  unexpected rpcs:\n")
		for _, ev := range r.UnexpectedRPCs {
			fmt.Fprintf(w, "    %s %s\n", ev.Method, formatValue(ev.Request))
		}
	}
	if bench {
		r.writeBench(w)
	}
}

func (r *Report) writeBench(w io.Writer) {
	if len(r.Methods) == 0 {
		return
	}
	methods := make([]string, 0, len(r.Methods))
	for m := range r.Methods {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	fmt.Fprintf(w, "  %-45s %5s %9s %9s %9s %9s %9s\n",
		"method", "count", "min", "avg", "p50", "p95", "max")
	for _, m := range methods {
		s := r.Methods[m]
		fmt.Fprintf(w, "  %-45s %5d %8.2fm %8.2fm %8.2fm %8.2fm %8.2fm\n",
			m, s.Count, s.MinMS, s.AvgMS, s.P50MS, s.P95MS, s.MaxMS)
	}
}
