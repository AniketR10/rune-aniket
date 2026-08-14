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
