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

// xsandbox impersonates the Rune editor's side of the extension
// protocol so an extension binary — built with rune-go-sdk or an SDK
// port in another language — can be exercised end-to-end without
// running Rune.
//
// It launches the extension against a real unix socket + gRPC server
// (TLS and per-RPC token auth enabled by default, exactly like Rune),
// evaluates a Starlark spec describing the expected RPCs and the
// actions the sandbox should perform, and reports pass/fail plus
// timing statistics.
//
// Usage:
//
//	xsandbox --spec spec.star -- ./bin/my-extension [ext-args...]
//
// Exit codes: 0 on success, 1 on spec/expectation failure or
// extension crash, 2 on usage or setup errors.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"unstable.build/go-tui/cmd/xsandbox/internal/sandbox"
	"unstable.build/go-tui/debug"
)

func main() {
	debug.StartPProfOnSignal()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("xsandbox", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintf(stderr,
			"usage: xsandbox --spec spec.star [flags] -- <extension> [ext-args...]\n\n")
		flags.PrintDefaults()
	}
	var (
		specPath          = flags.String("spec", "", "path to the Starlark spec (required)")
		dataDir           = flags.String("datadir", "", "host data directory (default: temp dir)")
		workspaceDir      = flags.String("workspace", "", "workspace root served to the extension (default: temp dir)")
		insecure          = flags.Bool("insecure", false, "disable both TLS and per-RPC token auth")
		insecureTransport = flags.Bool("insecure-transport", false, "disable TLS")
		insecureAuth      = flags.Bool("insecure-auth", false, "disable per-RPC token auth")
		timeout           = flags.Duration("timeout", 60*time.Second, "overall run timeout")
		expectTimeout     = flags.Duration("expect-timeout", 10*time.Second, "default per-expectation timeout")
		grace             = flags.Duration("grace", 5*time.Second, "grace period between SIGTERM and SIGKILL")
		verbose           = flags.Bool("verbose", false, "stream extension stderr and sandbox diagnostics")
		bench             = flags.Bool("bench", false, "include per-method latency stats in the summary")
		jsonPath          = flags.String("json", "", "write the machine-readable report to this file")
	)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *specPath == "" || flags.NArg() == 0 {
		flags.Usage()
		return 2
	}
	if *verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		// The extension host forwards structured extension stderr
		// records through logrus; keep the sandbox output clean
		// unless something is actually wrong.
		log.SetLevel(log.WarnLevel)
	}

	specSource, err := os.ReadFile(*specPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: read spec: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logSink := io.Discard
	if *verbose {
		logSink = stderr
	}
	report, runErr := sandbox.Run(ctx, sandbox.Options{
		SpecSource:        specSource,
		SpecFilename:      filepath.Base(*specPath),
		ExtensionArgv:     flags.Args(),
		DataDir:           *dataDir,
		WorkspaceDir:      *workspaceDir,
		InsecureTransport: *insecure || *insecureTransport,
		InsecureAuth:      *insecure || *insecureAuth,
		Timeout:           *timeout,
		ExpectTimeout:     *expectTimeout,
		Grace:             *grace,
		Verbose:           *verbose,
		Log:               logSink,
	})
	if report == nil {
		fmt.Fprintf(stderr, "error: %v\n", runErr)
		return 2
	}
	report.WriteText(stdout, *bench)
	if *jsonPath != "" {
		if err := writeJSON(*jsonPath, report); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
	}
	if runErr != nil {
		fmt.Fprintf(stderr, "error: %v\n", runErr)
		return 1
	}
	return 0
}

func writeJSON(path string, report any) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
