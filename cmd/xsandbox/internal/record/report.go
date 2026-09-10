// Copyright (C) 2017-2026 The Rune Authors
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
