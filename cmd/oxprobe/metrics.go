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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/unstablebuild/ox-api/api/oxapi"
)

// metrics holds the Prometheus collectors exported by the daemon.
type metrics struct {
	up          *prometheus.GaugeVec
	latency     *prometheus.HistogramVec
	unconfirmed *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		up: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "oxprobe_layer_up",
			Help: "1 if the layer's last probe succeeded, 0 otherwise.",
		}, []string{"env", "layer"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "oxprobe_layer_latency_seconds",
			Help:    "Probe latency per layer in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"env", "layer"}),
		// oxprobe_layer_up reports the confirmed result, so a blip that
		// recovers on the re-check leaves no trace there. This counter keeps
		// flapping visible and alertable without paging on it.
		unconfirmed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "oxprobe_layer_unconfirmed_failures_total",
			Help: "Failures that passed the confirmation re-check and so did not page.",
		}, []string{"env", "layer"}),
	}
	reg.MustRegister(m.up, m.latency, m.unconfirmed)
	return m
}

func (m *metrics) observe(env string, results []oxapi.CheckResult, unconfirmed []string) {
	for _, r := range results {
		up := 0.0
		if r.Status == oxapi.CheckOK {
			up = 1
		}
		m.up.WithLabelValues(env, r.Layer).Set(up)
		m.latency.WithLabelValues(env, r.Layer).Observe(float64(r.LatencyMS) / 1000)
	}
	for _, layer := range unconfirmed {
		m.unconfirmed.WithLabelValues(env, layer).Inc()
	}
}
