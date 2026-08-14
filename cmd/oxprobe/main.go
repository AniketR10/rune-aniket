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

// Command oxprobe runs layered synthetic health probes against a
// deployed ox-api environment. In one-shot mode it prints a structured
// report and exits non-zero when any critical layer fails. In daemon
// mode it probes on an interval, exports Prometheus metrics, and
// reconciles PagerDuty each run — triggering an incident for every
// failing critical layer and resolving it once the layer recovers.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/ox-api/api/oxapi/pagerduty"
	"github.com/unstablebuild/ox-api/api/pager"

	"unstable.build/go-tui/debug"
)

// infraRoutingKeySecretID is the Secret Manager secret holding the
// PagerDuty Events API routing key for the infra service oxprobe pages.
// It is distinct from ox-api's own routing key.
const infraRoutingKeySecretID = "pagerduty-infra-key-prod"

func main() {
	var (
		envName          string
		daemon           bool
		interval         time.Duration
		perProbeTimeout  time.Duration
		confirmDelay     time.Duration
		metricsAddr      string
		projectID        string
		credsFile        string
		probeSecretID    string
		skipDownloadsCDN bool
	)
	// Each option is exposed under a long (--name) and a one-character
	// short (-x) alias bound to the same variable.
	strVar := func(p *string, long, short, def, usage string) {
		flag.StringVar(p, long, def, usage)
		flag.StringVar(p, short, def, usage)
	}
	boolVar := func(p *bool, long, short string, def bool, usage string) {
		flag.BoolVar(p, long, def, usage)
		flag.BoolVar(p, short, def, usage)
	}
	durVar := func(p *time.Duration, long, short string, def time.Duration, usage string) {
		flag.DurationVar(p, long, def, usage)
		flag.DurationVar(p, short, def, usage)
	}
	strVar(&envName, "env", "e", "staging", "Target environment: staging or prod")
	boolVar(&daemon, "daemon", "d", false, "Run continuously: probe on an interval, export Prometheus metrics, and page PagerDuty on critical-layer failures. Without it, probe once, print the report, and exit (non-zero on critical failure).")
	durVar(&interval, "interval", "i", time.Minute, "Daemon probe interval")
	durVar(&perProbeTimeout, "probe-timeout", "t", 15*time.Second, "Per-probe timeout")
	durVar(&confirmDelay, "confirm-delay", "r", defaultConfirmDelay, "Wait this long and re-run a probe whose critical layer failed; only a second failure pages. Zero pages on the first failure.")
	strVar(&metricsAddr, "metrics-addr", "m", ":9106", "Address for the Prometheus /metrics endpoint (daemon mode)")
	strVar(&projectID, "project", "p", "", "Override the GCP project for Secret Manager (probe secret + PagerDuty routing key). Defaults to the selected environment's project.")
	strVar(&credsFile, "creds", "c", "", "GCP credentials file for Secret Manager")
	strVar(&probeSecretID, "probe-secret-id", "s", "ox-api-health-probe", "Secret Manager resource ID for the deep-probe X-Probe-Secret value")
	boolVar(&skipDownloadsCDN, "skip-downloads-cdn", "D", false, "Skip the downloads CDN manifest probe. Cloud Run deployments use this when CDN access is covered by the external Cloudflare Worker probe.")
	flag.Parse()

	cfg, err := lookupEnv(envName)
	if err != nil {
		fatalf("%v", err)
	}

	if projectID == "" {
		projectID = cfg.GCPProject
	}
	secretStore, err := secretmanager.SecretStore(projectID, credsFile)
	if err != nil {
		fatalf("secret store: %v", err)
	}

	probeSecret := loadProbeSecret(secretStore, probeSecretID)
	client := &http.Client{Timeout: perProbeTimeout}
	r := newRunner(cfg, client, probeSecret, runnerConfig{
		SkipDownloadsCDN: skipDownloadsCDN,
		ConfirmDelay:     confirmDelay,
	})

	if !daemon {
		os.Exit(runOnce(context.Background(), r, perProbeTimeout, cfg.Name))
	}

	pgr := pagerduty.New(secretStore, infraRoutingKeySecretID)
	runDaemon(r, interval, perProbeTimeout, metricsAddr, cfg.Name, pgr)
}

// loadProbeSecret reads the deep-probe shared secret from Secret Manager.
// The secret gates verbose ox-api /health output, so a failure to load it
// is fatal rather than silently disabling the deep probe.
func loadProbeSecret(store blueauth.SecretStore, id string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	b, err := store.AccessSecret(ctx, id)
	if err != nil {
		fatalf("load probe secret %q: %v", id, err)
	}
	secret := strings.TrimSpace(string(b))
	if secret == "" {
		fatalf("probe secret %q resolved to an empty value", id)
	}
	return secret
}

// runOnce runs the probe set once, prints the JSON report, and returns
// the process exit code (1 on any critical-layer failure).
func runOnce(ctx context.Context, r *runner, timeout time.Duration, env string) int {
	report, _ := r.run(ctx, timeout)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
	}
	if report.HasCriticalFailure() {
		fmt.Fprintf(os.Stderr, "oxprobe %s: critical failure\n", env)
		return 1
	}
	return 0
}

func runDaemon(r *runner, interval, timeout time.Duration, metricsAddr, env string, pgr pager.Pager) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: metricsAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go debug.CapturePanicReport(func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatalf("metrics server: %v", err)
		}
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// probeOnce is synchronous, so a run that confirms a failure delays the
	// next tick by confirmDelay. time.Ticker coalesces missed ticks, so the
	// cadence recovers without accumulating drift.
	probeOnce := func() {
		report, unconfirmed := r.run(ctx, timeout)
		m.observe(env, report.Checks, unconfirmed)
		if err := reconcile(ctx, pgr, env, report); err != nil {
			fmt.Fprintf(os.Stderr, "reconcile: %v\n", err)
		}
	}

	probeOnce()
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
			return
		case <-ticker.C:
			probeOnce()
		}
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
